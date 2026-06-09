package main

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
// DNS restoration is handled by the platform backend directly.
type Backup struct {
	Backend string `toml:"backend"`
}

// configPath returns the config file path, relative to the executable directory.
// Using exe-relative paths ensures the config is found even when UAC elevation
// changes the working directory (Windows) or when invoked from a different CWD.
func configPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "config.toml"
	}
	return filepath.Join(filepath.Dir(exe), "config.toml")
}

// ReadConfig loads the TOML config from disk. Returns a zero-value Config
// (with an empty Servers map) when the file does not exist.
func ReadConfig() (*Config, error) {
	cfg := &Config{Servers: make(map[string]string)}

	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("打开配置: %w", err)
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置: %w", err)
	}
	if cfg.Servers == nil {
		cfg.Servers = make(map[string]string)
	}
	return cfg, nil
}

// WriteConfig serialises cfg and writes it to the config file.
func WriteConfig(cfg *Config) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(configPath(), data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// SaveLastTest updates the [last_test] section in the config file.
func SaveLastTest(optimal string, rttMs float64) error {
	cfg, err := ReadConfig()
	if err != nil {
		return err
	}
	cfg.LastTest = &LastTest{
		Optimal: optimal,
		RTTMs:   rttMs,
		Time:    time.Now().UTC().Format(time.RFC3339),
	}
	return WriteConfig(cfg)
}

// WriteBackup writes the [backup] section to the config file.
func WriteBackup(backend string) error {
	cfg, err := ReadConfig()
	if err != nil {
		return err
	}
	cfg.Backup = &Backup{
		Backend: backend,
	}
	return WriteConfig(cfg)
}

// ClearBackup removes the [backup] section from the config file.
func ClearBackup() error {
	cfg, err := ReadConfig()
	if err != nil {
		return err
	}
	cfg.Backup = nil
	return WriteConfig(cfg)
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
