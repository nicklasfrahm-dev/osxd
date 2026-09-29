// Package config persists the user's answers between runs.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/nicklasfrahm/osxd/pkg/features/spotlight/shortcut"
)

// Consent is the user's answer to the "may I take over Super+Space?" question.
type Consent string

const (
	Unasked Consent = ""
	Granted Consent = "granted"
	Denied  Consent = "denied"
)

// Config is stored as JSON in the user's config directory.
type Config struct {
	Consent Consent `json:"shortcut_consent"`
	// PreviousBindings holds the original value of every keybinding that used
	// Super+Space, so the shortcut can be given back with `osxd --restore`.
	PreviousBindings []shortcut.Setting `json:"previous_bindings,omitempty"`
	// PreviousInputSwitch is the original switch-input-source value, as
	// recorded by versions that only freed that key. Kept so they can restore.
	PreviousInputSwitch string `json:"previous_input_switch,omitempty"`
	// PreviousCenterNewWindows is the original org.gnome.mutter value.
	PreviousCenterNewWindows string `json:"previous_center_new_windows,omitempty"`
}

// Path returns the location of the config file.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "osxd", "config.json"), nil
}

// Load reads the config; a missing file yields the zero Config.
func Load(path string) (Config, error) {
	var c Config
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	return c, json.Unmarshal(b, &c)
}

// Save writes the config, creating parent directories as needed.
func Save(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
