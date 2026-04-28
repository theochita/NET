package syslog

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

const configFilename = "syslog-config.json"

func loadConfig(dir string) (SyslogConfig, error) {
	var cfg SyslogConfig
	data, err := os.ReadFile(filepath.Join(dir, configFilename))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return SyslogConfig{}, err
	}
	return cfg, nil
}

func saveConfig(dir string, cfg SyslogConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(filepath.Join(dir, configFilename), data)
}

// atomicWriteFile writes data to path via a temp-file rename. Duplicated from
// dhcp/lease.go and tftp/store.go intentionally — no cross-package dependency
// on an unexported helper.
func atomicWriteFile(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
