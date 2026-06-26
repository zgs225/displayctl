package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

type OutputConfig struct {
	Name string `toml:"name"`
	Mode string `toml:"mode"`
	Rate int    `toml:"rate,omitempty"`
}

type DPIConfig struct {
	Value *int  `toml:"value,omitempty"`
	Tiers *bool `toml:"tiers,omitempty"`
}

type Profile struct {
	Default bool         `toml:"default"`
	Output  OutputConfig `toml:"output"`
	DPI     DPIConfig    `toml:"dpi"`
}

type ProfileSummary struct {
	Name    string
	Mode    string
	DPI     string
	Default bool
}

func Load(dir, name string) (*Profile, error) {
	path := filepath.Join(dir, "profiles", name+".toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("profile %s not found: %w", name, err)
	}
	var p Profile
	if err := toml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse profile %s: %w", name, err)
	}
	return &p, nil
}

func LoadDefault(dir string) (*Profile, error) {
	entries, err := os.ReadDir(filepath.Join(dir, "profiles"))
	if err != nil {
		return nil, fmt.Errorf("read profiles dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".toml")
		p, err := Load(dir, name)
		if err != nil {
			continue
		}
		if p.Default {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no default profile found")
}

func List(dir string) ([]ProfileSummary, error) {
	entries, err := os.ReadDir(filepath.Join(dir, "profiles"))
	if err != nil {
		return nil, fmt.Errorf("read profiles dir: %w", err)
	}
	var summaries []ProfileSummary
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".toml")
		p, err := Load(dir, name)
		if err != nil {
			continue
		}
		dpiStr := "none"
		if p.DPI.Value != nil {
			dpiStr = fmt.Sprintf("%d", *p.DPI.Value)
		} else if p.DPI.Tiers != nil && *p.DPI.Tiers {
			dpiStr = "tiers"
		}
		summaries = append(summaries, ProfileSummary{
			Name:    name,
			Mode:    p.Output.Mode,
			DPI:     dpiStr,
			Default: p.Default,
		})
	}
	return summaries, nil
}
