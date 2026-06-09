package main

import (
	"context"
	"fmt"
	"math"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// probeTarget is the domain name used for DNS resolution benchmarks.
const probeTarget = "www.baidu.com"

// briefErr shortens common Go net errors for display.
func briefErr(err error) string {
	s := err.Error()
	if i := strings.LastIndex(s, ": "); i >= 0 {
		s = s[i+2:]
	}
	switch {
	case s == "EOF":
		return "refused"
	case s == "i/o timeout":
		return "timeout"
	case s == "connection refused":
		return "refused"
	case s == "no such host":
		return "nxdomain"
	case len(s) > 25:
		return s[:22] + "..."
	default:
		return s
	}
}

// measureOne benchmarks a single DNS server: n probes, each with the given
// timeout. Returns the average RTT of successful probes, false when all
// probes failed, and the last error message.
func measureOne(dnsIP string, n int, timeout time.Duration) (avg time.Duration, ok bool, errMsg string) {
	dialer := &net.Dialer{Timeout: timeout}

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "udp", dnsIP+":53")
		},
	}

	var total time.Duration
	success := 0
	var lastErr string
	for range n {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		start := time.Now()
		_, err := resolver.LookupHost(ctx, probeTarget)
		cancel()
		if err == nil {
			total += time.Since(start)
			success++
		} else {
			lastErr = briefErr(err)
		}
	}
	if success == 0 {
		return 0, false, lastErr
	}
	return total / time.Duration(success), true, ""
}

// onResult is a callback invoked as each DNS server finishes benchmarking.
type onResult func(name, ip string, rttMs float64, err bool, errMsg string)

// benchmarkAll concurrently benchmarks all given DNS servers.
// Each server is probed 3 times with a 2-second timeout.
// The onResult callback is called as each server completes.
func benchmarkAll(servers map[string]string, cb onResult) {
	var wg sync.WaitGroup

	const probes = 3
	const timeout = 2 * time.Second

	for name, ip := range servers {
		name, ip := name, ip
		wg.Go(func() {
			rtt, ok, errMsg := measureOne(ip, probes, timeout)

			var rttMs float64
			if ok {
				rttMs = float64(rtt) / float64(time.Millisecond)
				rttMs = math.Round(rttMs*10) / 10
			}
			if cb != nil {
				cb(name, ip, rttMs, !ok, errMsg)
			}
		})
	}
	wg.Wait()
}

// ── CLI entry (was test.go) ──

// probeResult records a single DNS server's benchmark result.
type probeResult struct {
	Name   string
	IP     string
	AvgRTT float64
	Loss   int
	Err    bool
	ErrMsg string
}

// testCmd 并发测速所有 DNS，出错时退出进程。
func testCmd() {
	cfg, err := ReadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERR 读取配置失败: %v\n", err)
		os.Exit(1)
	}
	if len(cfg.Servers) == 0 {
		fmt.Fprintln(os.Stderr, "ERR 配置文件中没有定义 DNS 服务器")
		os.Exit(1)
	}
	runTest(cfg)
}

// runTest 并发测速所有 DNS，收集结果后输出排序表格。
func runTest(cfg *Config) {
	var mu sync.Mutex
	results := make([]probeResult, 0, len(cfg.Servers))
	bestIdx := -1

	progress := func(name, ip string, rttMs float64, err bool, errMsg string) {
		r := probeResult{Name: name, IP: ip}
		if err {
			r.Err = true
			r.ErrMsg = errMsg
			r.Loss = 100
		} else {
			r.AvgRTT = rttMs
		}

		mu.Lock()
		idx := sort.Search(len(results), func(i int) bool {
			cr := results[i]
			if cr.Err {
				return true
			}
			if r.Err {
				return false
			}
			return r.AvgRTT < cr.AvgRTT
		})
		results = append(results, probeResult{})
		copy(results[idx+1:], results[idx:])
		results[idx] = r

		if !r.Err {
			if bestIdx < 0 {
				bestIdx = idx
			} else if r.AvgRTT < results[bestIdx].AvgRTT {
				bestIdx = idx
			} else if bestIdx >= idx {
				bestIdx++
			}
		}
		mu.Unlock()
	}

	benchmarkAll(cfg.Servers, progress)

	fmt.Printf("%-16s %-16s %8s  %s\n", "名称", "地址", "延迟", "丢包")
	for _, r := range results {
		rttStr := fmt.Sprintf("%.1fms", r.AvgRTT)
		lossStr := fmt.Sprintf("%d%%", r.Loss)
		if r.Err {
			rttStr = r.ErrMsg
			lossStr = "100%"
		}
		fmt.Printf("%-16s %-16s %8s  %s\n", r.Name, r.IP, rttStr, lossStr)
	}
	fmt.Println("---")

	if bestIdx >= 0 {
		b := results[bestIdx]
		fmt.Printf("最优: %s (%s) %.1fms\n", b.Name, b.IP, b.AvgRTT)
		if err := SaveLastTest(b.Name, b.AvgRTT); err != nil {
			fmt.Fprintf(os.Stderr, "ERR 保存测速结果失败: %v\n", err)
		}
	} else {
		fmt.Println("所有 DNS 服务器均不可达")
	}
}
