package layout

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/zgs225/displayctl/internal/fingerprint"
	"github.com/zgs225/displayctl/internal/profile"
	"github.com/zgs225/displayctl/internal/xrandr"
)

func boolTrue() *bool { b := true; return &b }

func modeArea(mode string) int {
	parts := strings.SplitN(mode, "x", 2)
	if len(parts) != 2 {
		return 0
	}
	w, _ := strconv.Atoi(parts[0])
	h, _ := strconv.Atoi(parts[1])
	return w * h
}

func connectedOutputs(disc []xrandr.Output) []xrandr.Output {
	var active []xrandr.Output
	for _, o := range disc {
		if o.Connected {
			active = append(active, o)
		}
	}
	sort.Slice(active, func(i, j int) bool { return active[i].Name < active[j].Name })
	return active
}

func primaryIndex(active []xrandr.Output) int {
	for i, o := range active {
		if isInternal(o.Name) {
			return i
		}
	}
	return 0
}

func fingerprintMatch(disc []xrandr.Output) *profile.Match {
	refs := fingerprint.FromOutputs(disc)
	labels := make([]string, len(refs))
	for i, r := range refs {
		labels[i] = r.Label()
	}
	sort.Strings(labels)
	return &profile.Match{ID: fingerprint.Compute(refs), Outputs: labels}
}

// Capture builds a v2 profile from the current live state: active outputs keep
// their exact mode/position/primary, the fingerprint binds it to this hardware
// combination, and the layout label is inferred. manage=full so re-applying
// reproduces the state exactly (inactive connected outputs stay off).
func Capture(disc []xrandr.Output) *profile.Profile {
	p := &profile.Profile{
		Manage:  "full",
		Outputs: map[string]*profile.OutputConfig{},
		DPI:     profile.DPIConfig{Tiers: boolTrue()},
	}

	var active []xrandr.Output
	for _, o := range disc {
		if o.Connected && o.Active() {
			active = append(active, o)
			p.Outputs[o.Name] = &profile.OutputConfig{
				Mode:    o.Mode,
				Pos:     fmt.Sprintf("%dx%d", o.PosX, o.PosY),
				Primary: o.Primary,
			}
			if o.Primary {
				p.DPI.Reference = o.Name
			}
		}
	}
	if p.DPI.Reference == "" && len(active) > 0 {
		p.DPI.Reference = active[primaryIndex(active)].Name
	}

	p.Layout = inferLayout(active)
	p.Match = fingerprintMatch(disc)
	return p
}

func inferLayout(active []xrandr.Output) string {
	if len(active) <= 1 {
		return "single"
	}
	samePos, sameMode := true, true
	for i := 1; i < len(active); i++ {
		if active[i].PosX != active[0].PosX || active[i].PosY != active[0].PosY {
			samePos = false
		}
		if active[i].Mode != active[0].Mode {
			sameMode = false
		}
	}
	if samePos && sameMode {
		return "mirror"
	}
	return "extend"
}

// Mirror generates a mirror profile: all connected outputs at pos 0x0 sharing
// the largest mode they all support.
func Mirror(disc []xrandr.Output) *profile.Profile {
	active := connectedOutputs(disc)
	p := &profile.Profile{
		Layout:  "mirror",
		Manage:  "full",
		Outputs: map[string]*profile.OutputConfig{},
		DPI:     profile.DPIConfig{Tiers: boolTrue()},
	}
	if len(active) == 0 {
		return p
	}
	mode := commonMode(active)
	pi := primaryIndex(active)
	for i, o := range active {
		cfg := &profile.OutputConfig{Mode: mode, Pos: "0x0"}
		if i == pi {
			cfg.Primary = true
			p.DPI.Reference = o.Name
		}
		p.Outputs[o.Name] = cfg
	}
	return p
}

// commonMode returns the largest-area mode supported by every active output,
// falling back to the primary output's preferred mode when none is shared.
func commonMode(active []xrandr.Output) string {
	counts := map[string]int{}
	for _, o := range active {
		seen := map[string]bool{}
		for _, m := range o.Modes {
			if seen[m.Name] {
				continue
			}
			seen[m.Name] = true
			counts[m.Name]++
		}
	}
	best, bestArea := "", -1
	for name, c := range counts {
		if c == len(active) && modeArea(name) > bestArea {
			best, bestArea = name, modeArea(name)
		}
	}
	if best != "" {
		return best
	}
	return active[primaryIndex(active)].PreferredMode()
}

// Single generates a single-output profile: only the named output is enabled
// (preferred mode, primary); manage=full turns every other output off.
func Single(disc []xrandr.Output, name string) (*profile.Profile, error) {
	var target *xrandr.Output
	for i := range disc {
		if disc[i].Name == name && disc[i].Connected {
			target = &disc[i]
		}
	}
	if target == nil {
		return nil, fmt.Errorf("output %s is not connected", name)
	}
	mode := target.PreferredMode()
	if mode == "" {
		mode = target.Mode
	}
	return &profile.Profile{
		Layout: "single",
		Manage: "full",
		Outputs: map[string]*profile.OutputConfig{
			name: {Mode: mode, Pos: "0x0", Primary: true},
		},
		DPI: profile.DPIConfig{Tiers: boolTrue(), Reference: name},
	}, nil
}
