package main

import (
	"fmt"
	"net"
	"os"

	"dns-switch/internal/bench"
	"dns-switch/internal/config"
	"dns-switch/internal/dns"
	"dns-switch/platform"
)

// ── set ──

// resolveDNSArg returns the IP address for arg.
// If arg is a valid IP, it's used directly;
// otherwise it's looked up by name in config's [servers].
func resolveDNSArg(arg string, servers map[string]string) (string, error) {
	if ip := net.ParseIP(arg); ip != nil {
		return arg, nil
	}
	ip, found := config.LookupServer(servers, arg)
	if !found {
		return "", fmt.Errorf("未知名称 %q，请使用 IP 地址或配置中的名称", arg)
	}
	return ip, nil
}

// setCmd switches to DNS server(s). primary is required, secondary is optional.
// Each argument can be a raw IP address or a name from config's [servers].
func setCmd(primary, secondary string) error {
	cfg, err := config.Read()
	if err != nil {
		return fmt.Errorf("读取配置失败: %w", err)
	}

	ip1, err := resolveDNSArg(primary, cfg.Servers)
	if err != nil {
		return err
	}

	dnsIPs := []string{ip1}
	displayNames := []string{primary}

	if secondary != "" {
		ip2, err := resolveDNSArg(secondary, cfg.Servers)
		if err != nil {
			return err
		}
		dnsIPs = append(dnsIPs, ip2)
		displayNames = append(displayNames, secondary)
	}

	mgr := dns.New(config.FileStore{})
	if err := mgr.Set(dnsIPs); err != nil {
		if platform.IsPrivilegedError(err) {
			return fmt.Errorf("权限不足，请以 root/管理员身份运行")
		}
		return fmt.Errorf("设置 DNS 失败: %w", err)
	}

	labels := displayNames[0]
	if len(displayNames) > 1 {
		labels = displayNames[0] + " + " + displayNames[1]
	}
	fmt.Printf("OK %s 已切换\n", labels)
	return nil
}

// ── restore ──

// restoreCmd 恢复网卡为 DHCP 自动获取 DNS。
func restoreCmd() error {
	if err := dns.New(config.FileStore{}).Restore(); err != nil {
		if platform.IsPrivilegedError(err) {
			return fmt.Errorf("权限不足，请以 root/管理员身份运行")
		}
		return fmt.Errorf("恢复 DNS 失败: %w", err)
	}
	fmt.Println("OK 已恢复为 DHCP 自动获取")
	return nil
}

// ── test (CLI entry) ──

// testCmd 并发测速所有 DNS，出错时退出进程。
func testCmd() error {
	cfg, err := config.Read()
	if err != nil {
		return fmt.Errorf("读取配置失败: %w", err)
	}
	if len(cfg.Servers) == 0 {
		return fmt.Errorf("配置文件中没有定义 DNS 服务器")
	}
	runTest(cfg)
	return nil
}

// runTest 并发测速所有 DNS，收集结果后输出排序表格。
func runTest(cfg *config.Config) {
	bench.Run(cfg.Servers, func(results []bench.Result, bestIdx int) {
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
			if err := config.SaveLastTest(b.Name, b.AvgRTT); err != nil {
				fmt.Fprintf(os.Stderr, "ERR 保存测速结果失败: %v\n", err)
			}
		} else {
			fmt.Println("所有 DNS 服务器均不可达")
		}
	})
}
