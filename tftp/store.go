package tftp

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

const configFilename = "tftp-config.json"

// loadConfig reads the persisted TFTPConfig from dir. A missing file yields
// the zero-value config with no error.
func loadConfig(dir string) (TFTPConfig, error) {
	var cfg TFTPConfig
	data, err := os.ReadFile(filepath.Join(dir, configFilename))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return TFTPConfig{}, err
	}
	return cfg, nil
}

// saveConfig writes cfg to dir/tftp-config.json atomically.
func saveConfig(dir string, cfg TFTPConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(filepath.Join(dir, configFilename), data)
}

// atomicWriteFile writes data to path via a temp file + rename. Duplicated
// from dhcp/lease.go intentionally to avoid a cross-package dependency on an
// unexported helper.
func atomicWriteFile(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
