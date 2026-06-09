//go:build linux

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// LinuxBackend manages DNS on Linux via systemd-resolved, NetworkManager,
// or direct resolv.conf manipulation.
type LinuxBackend struct {
	mode  string // resolved | nm | resolvconf
	iface string
}

// Name returns a human-readable name for this backend.
func (b *LinuxBackend) Name() string {
	switch b.mode {
	case "resolved":
		return "systemd-resolved"
	case "nm":
		return "NetworkManager"
	default:
		return "resolv.conf"
	}
}

// Detect returns the most appropriate Linux backend.
func Detect() *LinuxBackend {
	b := &LinuxBackend{}

	if _, err := os.Stat("/run/systemd/resolve/stub-resolv.conf"); err == nil {
		b.mode = "resolved"
	} else if nmAvailable() {
		b.mode = "nm"
	} else {
		b.mode = "resolvconf"
	}

	return b
}

func nmAvailable() bool {
	out, err := exec.Command("nmcli", "-t", "-f", "RUNNING", "general").Output()
	return err == nil && strings.TrimSpace(string(out)) == "running"
}

// DefaultIface returns the default route interface name.
func (b *LinuxBackend) DefaultIface() (string, error) {
	if b.iface != "" {
		return b.iface, nil
	}

	out, err := exec.Command("ip", "-o", "route", "show", "default").Output()
	if err != nil {
		return "", fmt.Errorf("get default route: %w", err)
	}
	// Output: "default via 192.168.1.1 dev eth0 proto dhcp src 192.168.1.100 metric 100"
	fields := strings.Fields(string(out))
	for i, f := range fields {
		if f == "dev" && i+1 < len(fields) {
			b.iface = fields[i+1]
			return b.iface, nil
		}
	}
	return "", fmt.Errorf("no 'dev' in default route output")
}

// SetDNS sets one or more DNS servers via the detected backend.
func (b *LinuxBackend) SetDNS(iface string, dns ...string) error {
	switch b.mode {
	case "resolved":
		return b.setResolved(iface, dns...)
	case "nm":
		return b.setNM(iface, dns...)
	default:
		return b.setResolvConf(dns...)
	}
}

// RestoreDNS restores DNS to DHCP/default via the detected backend.
func (b *LinuxBackend) RestoreDNS(iface string) error {
	switch b.mode {
	case "resolved":
		return exec.Command("resolvectl", "revert", iface).Run()
	case "nm":
		return b.restoreNM(iface)
	default:
		return b.restoreResolvConf()
	}
}

// --- systemd-resolved ---

func (b *LinuxBackend) setResolved(iface string, dns ...string) error {
	args := append([]string{"dns", iface}, dns...)
	return exec.Command("resolvectl", args...).Run()
}

// --- NetworkManager ---

func (b *LinuxBackend) setNM(iface string, dns ...string) error {
	con, err := b.nmConnectionName(iface)
	if err != nil {
		return fmt.Errorf("nm: %w", err)
	}
	// nmcli expects space-separated list as a single argument
	dnsVal := strings.Join(dns, " ")
	if err := exec.Command("nmcli", "con", "mod", con, "ipv4.dns", dnsVal).Run(); err != nil {
		return fmt.Errorf("nmcli con mod: %w", err)
	}
	return exec.Command("nmcli", "con", "up", con).Run()
}

func (b *LinuxBackend) restoreNM(iface string) error {
	con, err := b.nmConnectionName(iface)
	if err != nil {
		return fmt.Errorf("nm restore: %w", err)
	}
	if err := exec.Command("nmcli", "con", "mod", con, "ipv4.dns", "").Run(); err != nil {
		return fmt.Errorf("nmcli con mod: %w", err)
	}
	return exec.Command("nmcli", "con", "up", con).Run()
}

func (b *LinuxBackend) nmConnectionName(iface string) (string, error) {
	out, err := exec.Command("nmcli", "-t", "-f", "NAME,DEVICE", "con", "show", "--active").Output()
	if err != nil {
		return "", fmt.Errorf("list connections: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		// Format: "Wired connection 1:eth0"
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && parts[1] == iface {
			return parts[0], nil
		}
	}
	return "", fmt.Errorf("no NM connection found for interface %q", iface)
}

// --- resolv.conf ---

var resolvConfPath = "/etc/resolv.conf"

func (b *LinuxBackend) setResolvConf(dns ...string) error {
	backupPath := resolvConfPath + ".bak"
	data, err := os.ReadFile(resolvConfPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read resolv.conf: %w", err)
	}
	if err := os.WriteFile(backupPath, data, 0o644); err != nil {
		return fmt.Errorf("backup resolv.conf: %w", err)
	}
	var sb strings.Builder
	for _, s := range dns {
		sb.WriteString(fmt.Sprintf("nameserver %s\n", s))
	}
	return os.WriteFile(resolvConfPath, []byte(sb.String()), 0o644)
}

func (b *LinuxBackend) restoreResolvConf() error {
	backupPath := resolvConfPath + ".bak"
	data, err := os.ReadFile(backupPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No backup; remove our entry and let DHCP rewrite it
			return os.Remove(resolvConfPath)
		}
		return fmt.Errorf("read backup: %w", err)
	}
	if err := os.WriteFile(resolvConfPath, data, 0o644); err != nil {
		return fmt.Errorf("restore resolv.conf: %w", err)
	}
	return os.Remove(backupPath)
}

// Ensure LinuxBackend implements Backend.
var _ Backend = (*LinuxBackend)(nil)

// isPrivilegedError checks if a command error is due to missing privileges.
func isPrivilegedError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "permission denied") ||
		strings.Contains(msg, "denied") ||
		strings.Contains(strings.ToLower(msg), "permission")
}
