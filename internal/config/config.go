// Package config handles TOML configuration file read/write for dns-switch.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// Config is the top-level TOML configuration structure.
type Config struct {
	Servers  map[string]string `toml:"servers"`
	LastTest *LastTest         `toml:"last_test,omitempty"`
	Backup   *Backup           `toml:"backup,omitempty"`
}

// LastTest records the most recent benchmark result.
type LastTest struct {
	Optimal string  `toml:"optimal"`
	RTTMs   float64 `toml:"rtt_ms"`
	Time    string  `toml:"time"`
}

// Backup stores pre-switch state for restore.
type Backup struct {
	Backend string `toml:"backend"`
}

// configDir returns the platform-appropriate config directory for dns-switch.
func configDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "dns-switch")
}

// configPath returns the canonical config file path.
func configPath() string {
	if dir := configDir(); dir != "" {
		return filepath.Join(dir, "config.toml")
	}
	return "config.toml"
}

// legacyConfigPath returns the old exe-relative config path for migration.
func legacyConfigPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "config.toml")
}

// configRead tries to read and parse config from the given path.
func configRead(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("打开配置 %s: %w", path, err)
	}

	cfg := &Config{Servers: make(map[string]string)}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置 %s: %w", path, err)
	}
	if cfg.Servers == nil {
		cfg.Servers = make(map[string]string)
	}
	return cfg, nil
}

// write serialises cfg and writes to the config file, creating directories as needed.
func write(cfg *Config) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	path := configPath()

	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("创建配置目录: %w", err)
		}
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// ── Exported package-level functions ──

// Read loads the TOML config from the canonical path with legacy fallback.
func Read() (*Config, error) {
	// Try canonical path first
	if cfg, err := configRead(configPath()); err != nil {
		return nil, err
	} else if cfg != nil {
		return cfg, nil
	}

	// Fallback: try legacy exe-relative path (silent migration)
	if legacy := legacyConfigPath(); legacy != "" {
		if cfg, err := configRead(legacy); err != nil {
			return nil, err
		} else if cfg != nil {
			_ = write(cfg) // silently migrate
			return cfg, nil
		}
	}

	return &Config{Servers: make(map[string]string)}, nil
}

// SaveLastTest updates the [last_test] section in the config file.
func SaveLastTest(optimal string, rttMs float64) error {
	cfg, err := Read()
	if err != nil {
		return err
	}
	cfg.LastTest = &LastTest{
		Optimal: optimal,
		RTTMs:   rttMs,
		Time:    time.Now().UTC().Format(time.RFC3339),
	}
	return write(cfg)
}

// WriteBackup writes the [backup] section to the config file.
func WriteBackup(backend string) error {
	cfg, err := Read()
	if err != nil {
		return err
	}
	cfg.Backup = &Backup{Backend: backend}
	return write(cfg)
}

// ClearBackup removes the [backup] section from the config file.
func ClearBackup() error {
	cfg, err := Read()
	if err != nil {
		return err
	}
	cfg.Backup = nil
	return write(cfg)
}

// LookupServer performs a case-insensitive lookup of name in servers.
func LookupServer(servers map[string]string, name string) (string, bool) {
	for k, v := range servers {
		if strings.EqualFold(k, name) {
			return v, true
		}
	}
	return "", false
}
