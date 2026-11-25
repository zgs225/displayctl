package config

import (
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

// Settings is the optional <config>/config.toml. It lets a machine pin the
// global DPI that generated profiles (auto-arrange, quick layouts, temporary
// modes) must use instead of deriving one from the reference width.
//
// X11 has exactly one global DPI (Xft.dpi), so a width-tier fallback renders
// the same panel at a different size depending on which outputs happen to be
// connected — e.g. 144 on a 2560px laptop panel when an unknown dock is
// plugged in, but 168 on the same panel in single-screen mode.
type Settings struct {
	Defaults Defaults `toml:"defaults"`
}

// Defaults holds machine-wide fallbacks.
type Defaults struct {
	// DPI is the global DPI generated physical profiles must pin. 0 (or a
	// missing key) keeps the historical width-tier behaviour and should only
	// be used for virtual outputs (XRDP/Sunshine), whose resolution is
	// genuinely unknown until the client connects.
	DPI int `toml:"dpi"`
}

// LoadSettings reads <config>/config.toml. A missing or unparsable file yields
// zero Settings so callers fall back to tier-based DPI.
func LoadSettings() Settings {
	var s Settings
	data, err := os.ReadFile(filepath.Join(ConfigDir(), "config.toml"))
	if err != nil {
		return Settings{}
	}
	if err := toml.Unmarshal(data, &s); err != nil {
		return Settings{}
	}
	return s
}

// FixedDPI returns the pinned DPI, or 0 when it is unset.
func (s Settings) FixedDPI() int {
	if s.Defaults.DPI > 0 {
		return s.Defaults.DPI
	}
	return 0
}
