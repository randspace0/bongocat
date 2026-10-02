// Package config persists user settings (character, window geometry).
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Character string `json:"character,omitempty"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Width     int    `json:"width,omitempty"` // 0 = never saved, use skin default
}

func path() (string, error) {
	d, err := os.UserConfigDir()
	return filepath.Join(d, "bongocat", "config.json"), err
}

// Load returns the saved config, or zero value if missing or unreadable.
func Load() Config {
	var c Config
	if p, err := path(); err == nil {
		if b, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(b, &c)
		}
	}
	return c
}

func Save(c Config) error {
	p, err := path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(p, b, 0o644)
}
