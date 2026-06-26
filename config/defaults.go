package config

import (
	"os"
	"path/filepath"
)

func ConfigDir() string {
	if dir := os.Getenv("DISPLAYCTL_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return filepath.Join(home, ".config", "display")
}

func ProfilesDir() string {
	return filepath.Join(ConfigDir(), "profiles")
}

func PostSwitchDir() string {
	return filepath.Join(ConfigDir(), "post-switch.d")
}
