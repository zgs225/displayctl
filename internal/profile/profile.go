package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// OutputConfig is the v2 per-output configuration, keyed by output name under
// the [outputs.<name>] tables.
type OutputConfig struct {
	Mode    string  `toml:"mode"`              // "WxH" | "current" | "preferred" | "off"
	Rate    int     `toml:"rate,omitempty"`    // refresh rate; 0 = xrandr default
	Pos     string  `toml:"pos,omitempty"`     // "XxY" geometry for multi-monitor
	Primary bool    `toml:"primary,omitempty"` // primary output (DPI reference + i3)
	Rotate  string  `toml:"rotate,omitempty"`  // normal|left|right|inverted
	Scale   float64 `toml:"scale,omitempty"`   // xrandr --scale (bitmap transform)
}

// LegacyOutput is the v1 single-output [output] table, kept for backward
// compatibility with existing xrdp/sunshine/monitor profiles.
type LegacyOutput struct {
	Name string `toml:"name"`
	Mode string `toml:"mode"`
	Rate int    `toml:"rate,omitempty"`
}

// Match binds a profile to one or more hardware combination fingerprints.
//
// ID is the historical name+EDID fingerprint. IDs holds alternatives — in
// particular the name-independent EDID-set fingerprint (see
// fingerprint.ComputeEDIDs), which survives output renames such as DP-1 vs
// DP-1-1 across SST/MST dock enumeration. A profile matches when any of its
// ids equals either current fingerprint.
type Match struct {
	ID      string   `toml:"id"`
	IDs     []string `toml:"ids,omitempty"`     // alternative fingerprints (EDID-set ids)
	Outputs []string `toml:"outputs,omitempty"` // human-readable labels for debugging
}

// DPIConfig controls the (global) DPI. X11 only supports one DPI per screen.
type DPIConfig struct {
	Value     *int   `toml:"value,omitempty"`     // fixed DPI; takes priority over tiers
	Tiers     *bool  `toml:"tiers,omitempty"`     // auto-calculate from reference width
	Reference string `toml:"reference,omitempty"` // output name tiers keys off; default primary
}

// Profile is a display configuration. A profile is either v2 (Outputs map) or
// legacy (single Output); see IsLegacy.
type Profile struct {
	Default bool                     `toml:"default"`
	Layout  string                   `toml:"layout,omitempty"` // extend|mirror|single|custom (informational)
	Manage  string                   `toml:"manage,omitempty"` // full|partial; default full
	Match   *Match                   `toml:"match,omitempty"`
	Outputs map[string]*OutputConfig `toml:"outputs,omitempty"` // v2 multi-output
	Output  *LegacyOutput            `toml:"output,omitempty"`  // v1 single-output
	DPI     DPIConfig                `toml:"dpi"`
}

// IsLegacy reports whether the profile uses the v1 single-output [output] form.
func (p *Profile) IsLegacy() bool {
	return p.Output != nil && len(p.Outputs) == 0
}

// ManageMode returns "full" or "partial", defaulting to "full".
// full: apply turns off connected outputs not listed in the profile.
// partial: apply only touches listed outputs (used for virtual sessions).
func (p *Profile) ManageMode() string {
	if p.Manage == "" {
		return "full"
	}
	return p.Manage
}

// MatchID returns the profile's primary fingerprint id, or "" if it has none.
func (p *Profile) MatchID() string {
	if p.Match == nil {
		return ""
	}
	return p.Match.ID
}

// MatchIDs returns every fingerprint id the profile accepts, ID first.
func (p *Profile) MatchIDs() []string {
	if p.Match == nil {
		return nil
	}
	ids := make([]string, 0, len(p.Match.IDs)+1)
	if p.Match.ID != "" {
		ids = append(ids, p.Match.ID)
	}
	for _, id := range p.Match.IDs {
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// Matches reports whether the profile accepts any of the given fingerprint ids
// (current name-based and EDID-set fingerprints).
func (p *Profile) Matches(ids ...string) bool {
	for _, want := range p.MatchIDs() {
		for _, got := range ids {
			if got != "" && want == got {
				return true
			}
		}
	}
	return false
}

// MatchAny reports whether any of want equals any of got. Empty ids never
// match, so profiles without a fingerprint stay inert.
func MatchAny(want, got []string) bool {
	for _, w := range want {
		if w == "" {
			continue
		}
		for _, g := range got {
			if g != "" && w == g {
				return true
			}
		}
	}
	return false
}

// OutputNames returns the v2 output names in stable (sorted) order.
func (p *Profile) OutputNames() []string {
	names := make([]string, 0, len(p.Outputs))
	for name := range p.Outputs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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

// Save marshals p to TOML and writes it to profiles/<name>.toml.
func Save(dir, name string, p *Profile) error {
	pdir := filepath.Join(dir, "profiles")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		return fmt.Errorf("create profiles dir: %w", err)
	}
	data, err := toml.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal profile %s: %w", name, err)
	}
	path := filepath.Join(pdir, name+".toml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write profile %s: %w", name, err)
	}
	return nil
}

func LoadDefault(dir string) (*Profile, error) {
	for _, name := range profileNames(dir) {
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

// FindByFingerprint returns the (name, profile) whose match.id equals id.
func FindByFingerprint(dir, id string) (string, *Profile, error) {
	for _, name := range profileNames(dir) {
		p, err := Load(dir, name)
		if err != nil {
			continue
		}
		if p.MatchID() == id {
			return name, p, nil
		}
	}
	return "", nil, fmt.Errorf("no profile matches fingerprint %s", id)
}

// LoadAll loads every profile in the directory keyed by name. Profiles that
// fail to parse are skipped.
func LoadAll(dir string) map[string]*Profile {
	result := make(map[string]*Profile)
	for _, name := range profileNames(dir) {
		p, err := Load(dir, name)
		if err != nil {
			continue
		}
		result[name] = p
	}
	return result
}

// LoadDefaultName is like LoadDefault but also returns the profile's name.
func LoadDefaultName(dir string) (string, *Profile, error) {
	for _, name := range profileNames(dir) {
		p, err := Load(dir, name)
		if err != nil {
			continue
		}
		if p.Default {
			return name, p, nil
		}
	}
	return "", nil, fmt.Errorf("no default profile found")
}

func profileNames(dir string) []string {
	entries, err := os.ReadDir(filepath.Join(dir, "profiles"))
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		names = append(names, strings.TrimSuffix(entry.Name(), ".toml"))
	}
	sort.Strings(names)
	return names
}

type ProfileSummary struct {
	Name     string
	Layout   string
	Outputs  string
	Mode     string // legacy: single mode; v2: joined modes
	DPI      string
	Default  bool
	MatchID  string
	MatchIDs []string
}

func List(dir string) ([]ProfileSummary, error) {
	var summaries []ProfileSummary
	for _, name := range profileNames(dir) {
		p, err := Load(dir, name)
		if err != nil {
			continue
		}
		summaries = append(summaries, summarize(name, p))
	}
	return summaries, nil
}

func summarize(name string, p *Profile) ProfileSummary {
	dpiStr := "none"
	if p.DPI.Value != nil {
		dpiStr = fmt.Sprintf("%d", *p.DPI.Value)
	} else if p.DPI.Tiers != nil && *p.DPI.Tiers {
		dpiStr = "tiers"
	}

	s := ProfileSummary{
		Name:     name,
		Layout:   p.Layout,
		DPI:      dpiStr,
		Default:  p.Default,
		MatchID:  p.MatchID(),
		MatchIDs: p.MatchIDs(),
	}
	if p.IsLegacy() {
		s.Mode = p.Output.Mode
		s.Outputs = p.Output.Name
		if s.Layout == "" {
			s.Layout = "legacy"
		}
		return s
	}
	names := p.OutputNames()
	s.Outputs = strings.Join(names, ",")
	modes := make([]string, 0, len(names))
	for _, n := range names {
		modes = append(modes, p.Outputs[n].Mode)
	}
	s.Mode = strings.Join(modes, "+")
	return s
}
