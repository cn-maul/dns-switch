package main

import (
	"fmt"
	"net"
	"os"

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
	ip, found := LookupServer(servers, arg)
	if !found {
		return "", fmt.Errorf("未知名称 %q，请使用 IP 地址或配置中的名称", arg)
	}
	return ip, nil
}

// setCmd switches to DNS server(s). primary is required, secondary is optional.
// Each argument can be a raw IP address or a name from config's [servers].
func setCmd(primary, secondary string) {
	cfg, err := ReadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERR 读取配置失败: %v\n", err)
		os.Exit(1)
	}

	ip1, err := resolveDNSArg(primary, cfg.Servers)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERR", err)
		os.Exit(1)
	}

	dnsIPs := []string{ip1}
	displayNames := []string{primary}

	if secondary != "" {
		ip2, err := resolveDNSArg(secondary, cfg.Servers)
		if err != nil {
			fmt.Fprintln(os.Stderr, "ERR", err)
			os.Exit(1)
		}
		dnsIPs = append(dnsIPs, ip2)
		displayNames = append(displayNames, secondary)
	}

	if err := execSet(dnsIPs); err != nil {
		if platform.IsPrivilegedError(err) {
			fmt.Fprintln(os.Stderr, "ERR 权限不足，请以 root/管理员身份运行")
		} else {
			fmt.Fprintf(os.Stderr, "ERR 设置 DNS 失败: %v\n", err)
		}
		os.Exit(1)
	}

	labels := displayNames[0]
	if len(displayNames) > 1 {
		labels = displayNames[0] + " + " + displayNames[1]
	}
	fmt.Printf("OK %s 已切换\n", labels)
}

// execSet 执行 DNS 切换，返回 error（供 CLI 共用）。
func execSet(dnsIPs []string) error {
	be := platform.Detect()

	iface, err := be.DefaultIface()
	if err != nil {
		return fmt.Errorf("检测网卡失败: %w", err)
	}

	if err := WriteBackup(be.Name()); err != nil {
		return fmt.Errorf("写入备份失败: %w", err)
	}

	if err := be.SetDNS(iface, dnsIPs...); err != nil {
		return fmt.Errorf("设置 DNS 失败: %w", err)
	}

	return nil
}

// ── restore ──

// restoreCmd 恢复网卡为 DHCP 自动获取 DNS。
func restoreCmd() {
	if err := execRestore(); err != nil {
		if platform.IsPrivilegedError(err) {
			fmt.Fprintln(os.Stderr, "ERR 权限不足，请以 root/管理员身份运行")
		} else {
			fmt.Fprintf(os.Stderr, "ERR 恢复 DNS 失败: %v\n", err)
		}
		os.Exit(1)
	}
	fmt.Println("OK 已恢复为 DHCP 自动获取")
}

// execRestore 执行 DNS 恢复，返回 error（供 CLI 共用）。
func execRestore() error {
	cfg, err := ReadConfig()
	if err != nil {
		return fmt.Errorf("读取配置失败: %w", err)
	}

	if cfg.Backup == nil {
		return fmt.Errorf("没有找到备份记录，无需恢复")
	}

	be := platform.Detect()

	iface, err := be.DefaultIface()
	if err != nil {
		return fmt.Errorf("检测网卡失败: %w", err)
	}

	if err := be.RestoreDNS(iface); err != nil {
		return fmt.Errorf("恢复 DNS 失败: %w", err)
	}

	if err := ClearBackup(); err != nil {
		return fmt.Errorf("清除备份记录失败: %w", err)
	}

	return nil
}
