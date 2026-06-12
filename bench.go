package main

import (
	"context"
	"math"
	"net"
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

// RunBenchmark benchmarks all servers, collects sorted results, and calls
// onComplete with the results slice and the index of the best server.
func RunBenchmark(servers map[string]string, onComplete func([]BenchResult, int)) {
	var mu sync.Mutex
	results := make([]BenchResult, 0, len(servers))
	bestIdx := -1

	benchmarkAll(servers, func(name, ip string, rttMs float64, err bool, errMsg string) {
		r := BenchResult{Name: name, IP: ip}
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
		results = append(results, BenchResult{})
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
	})

	if onComplete != nil {
		onComplete(results, bestIdx)
	}
}
