// Package dns manages system DNS configuration via platform backends.
package dns

import (
	"fmt"

	"dns-switch/internal/config"
	"dns-switch/platform"
)

// Manager handles DNS set/restore operations.
type Manager struct {
	store config.Store
}

// New creates a Manager with the given config store.
func New(store config.Store) *Manager {
	return &Manager{store: store}
}

// Set writes the given DNS IPs to the default network interface.
func (m *Manager) Set(dnsIPs []string) error {
	be := platform.Detect()

	iface, err := be.DefaultIface()
	if err != nil {
		return fmt.Errorf("检测网卡失败: %w", err)
	}

	if err := m.store.WriteBackup(be.Name()); err != nil {
		return fmt.Errorf("写入备份失败: %w", err)
	}

	if err := be.SetDNS(iface, dnsIPs...); err != nil {
		return fmt.Errorf("设置 DNS 失败: %w", err)
	}

	return nil
}

// Restore reverts the DNS configuration to DHCP.
func (m *Manager) Restore() error {
	cfg, err := m.store.Read()
	if err != nil {
		return fmt.Errorf("读取配置失败: %w", err)
	}

	if cfg.Backup == nil {
		return fmt.Errorf("没有找到备份记录，无需恢复")
	}

	be := platform.Detect()

	// Validate that the detected backend matches the backup record.
	// This prevents silent DNS corruption when the OS state changes
	// between Set and Restore (e.g. service upgrade, container transition).
	if be.Name() != cfg.Backup.Backend {
		return fmt.Errorf(
			"后端不匹配: 备份记录为 %q，当前检测为 %q",
			cfg.Backup.Backend, be.Name(),
		)
	}

	iface, err := be.DefaultIface()
	if err != nil {
		return fmt.Errorf("检测网卡失败: %w", err)
	}

	if err := be.RestoreDNS(iface); err != nil {
		return fmt.Errorf("恢复 DNS 失败: %w", err)
	}

	if err := m.store.ClearBackup(); err != nil {
		return fmt.Errorf("清除备份记录失败: %w", err)
	}

	return nil
}
