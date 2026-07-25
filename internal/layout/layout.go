// Package layout turns display profiles into concrete xrandr invocations and
// implements the auto-selection logic: fingerprint matching for known hardware
// combinations, default-profile fallback for virtual sessions, and on-the-fly
// auto-arrangement for unknown combinations.
package layout

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/zgs225/displayctl/internal/dpi"
	"github.com/zgs225/displayctl/internal/fingerprint"
	"github.com/zgs225/displayctl/internal/profile"
	"github.com/zgs225/displayctl/internal/xrandr"
)

// Plan is a fully-resolved display change ready to execute.
type Plan struct {
	Args      []string          // xrandr arguments (excluding the "xrandr" binary)
	DPI       int               // resolved global DPI; 0 = leave unchanged
	Mode      string            // reference output's resolved mode (for hooks/display)
	Reference string            // output name the DPI was keyed off
	HookEnv   map[string]string // environment for post-switch hooks
}

// Plan resolves a v2 profile against the currently discovered outputs into a
// concrete xrandr argument list and DPI value. It is a pure function.
func Build(p *profile.Profile, disc []xrandr.Output) (*Plan, error) {
	byName := make(map[string]xrandr.Output, len(disc))
	var connected []string
	for _, o := range disc {
		byName[o.Name] = o
		if o.Connected {
			connected = append(connected, o.Name)
		}
	}

	var args []string
	resolvedModes := make(map[string]string)

	for _, name := range p.OutputNames() {
		cfg := p.Outputs[name]
		out, isConn := byName[name]
		if !isConn || !out.Connected {
			// Cannot configure an output that isn't connected; skip it.
			continue
		}
		resolved, off, skipMode, err := resolveMode(cfg.Mode, out)
		if err != nil {
			return nil, err
		}
		resolvedModes[name] = resolved

		args = append(args, "--output", name)
		if off {
			args = append(args, "--off")
			continue
		}
		if !skipMode && resolved != "" {
			args = append(args, "--mode", resolved)
			if cfg.Rate > 0 {
				args = append(args, "--rate", strconv.Itoa(cfg.Rate))
			}
		}
		if cfg.Pos != "" {
			args = append(args, "--pos", cfg.Pos)
		}
		if cfg.Rotate != "" {
			args = append(args, "--rotate", cfg.Rotate)
		}
		if cfg.Scale > 0 && cfg.Scale != 1 {
			s := strconv.FormatFloat(cfg.Scale, 'f', -1, 64)
			args = append(args, "--scale", s+"x"+s)
		}
		if cfg.Primary {
			args = append(args, "--primary")
		}
	}

	// manage=full: disable connected outputs the profile does not mention.
	if p.ManageMode() == "full" {
		sort.Strings(connected)
		for _, name := range connected {
			if _, inProfile := p.Outputs[name]; !inProfile {
				args = append(args, "--output", name, "--off")
			}
		}
	}

	refName, refWidth := resolveReference(p, resolvedModes)
	dpiValue := 0
	switch {
	case p.DPI.Value != nil:
		dpiValue = *p.DPI.Value
	case p.DPI.Tiers != nil && *p.DPI.Tiers && refWidth > 0:
		dpiValue = dpi.CalculateFromTiers(refWidth)
	}

	return &Plan{
		Args:      args,
		DPI:       dpiValue,
		Mode:      resolvedModes[refName],
		Reference: refName,
		HookEnv: map[string]string{
			"DISPLAYCTL_OUTPUT":       refName,
			"DISPLAYCTL_MODE":         resolvedModes[refName],
			"DISPLAYCTL_DPI":          strconv.Itoa(dpiValue),
			"DISPLAYCTL_XCURSOR_SIZE": strconv.Itoa(dpi.CalculateXcursorSize(dpiValue)),
		},
	}, nil
}

func resolveMode(mode string, out xrandr.Output) (resolved string, off, skipMode bool, err error) {
	switch mode {
	case "off":
		return "", true, false, nil
	case "current":
		return out.Mode, false, true, nil
	case "preferred", "auto", "":
		pm := out.PreferredMode()
		if pm == "" {
			return "", false, false, fmt.Errorf("output %s has no preferred mode", out.Name)
		}
		return pm, false, false, nil
	default:
		return mode, false, false, nil
	}
}

func resolveReference(p *profile.Profile, resolvedModes map[string]string) (string, int) {
	refName := p.DPI.Reference
	if refName == "" {
		for _, name := range p.OutputNames() {
			if p.Outputs[name].Primary {
				refName = name
				break
			}
		}
	}
	if refName == "" {
		for _, name := range p.OutputNames() {
			if _, ok := resolvedModes[name]; ok {
				refName = name
				break
			}
		}
	}
	width := modeWidth(resolvedModes[refName])
	return refName, width
}

func modeWidth(mode string) int {
	parts := strings.SplitN(mode, "x", 2)
	if len(parts) != 2 {
		return 0
	}
	w, _ := strconv.Atoi(parts[0])
	return w
}

func isInternal(name string) bool {
	return strings.HasPrefix(name, "eDP-") || strings.HasPrefix(name, "LVDS-")
}

// AutoArrange generates a temporary extend/single profile from the discovered
// connected outputs: preferred modes, internal output as primary at 0x0, other
// outputs chained to the right. Used for unknown hardware combinations.
func AutoArrange(disc []xrandr.Output) *profile.Profile {
	var active []xrandr.Output
	for _, o := range disc {
		if o.Connected {
			active = append(active, o)
		}
	}
	sort.Slice(active, func(i, j int) bool { return active[i].Name < active[j].Name })

	tiers := true
	p := &profile.Profile{
		Outputs: map[string]*profile.OutputConfig{},
		DPI:     profile.DPIConfig{Tiers: &tiers},
	}
	if len(active) == 0 {
		p.Layout = "single"
		return p
	}
	if len(active) == 1 {
		p.Layout = "single"
	} else {
		p.Layout = "extend"
	}

	primaryIdx := 0
	for i, o := range active {
		if isInternal(o.Name) {
			primaryIdx = i
			break
		}
	}
	primary := active[primaryIdx]

	p.Outputs[primary.Name] = &profile.OutputConfig{
		Mode:    primary.PreferredMode(),
		Pos:     "0x0",
		Primary: true,
	}
	p.DPI.Reference = primary.Name

	cursor := modeWidth(primary.PreferredMode())
	for i, o := range active {
		if i == primaryIdx {
			continue
		}
		pm := o.PreferredMode()
		p.Outputs[o.Name] = &profile.OutputConfig{
			Mode: pm,
			Pos:  fmt.Sprintf("%dx0", cursor),
		}
		cursor += modeWidth(pm)
	}
	return p
}

// Auto-selection reasons.
const (
	ReasonVirtualDefault   = "virtual-default"   // edid-less output → use default profile
	ReasonFingerprintMatch = "fingerprint-match" // known hardware combination
	ReasonAutoArranged     = "auto-arranged"     // unknown combination → generated layout
)

// Decision is the outcome of ResolveAuto.
type Decision struct {
	Profile   *profile.Profile
	Name      string // profile name; "default" or "auto" for fallbacks
	Generated bool   // true when the profile was auto-arranged (temporary)
	Reason    string
}

// ResolveAuto decides which profile to apply for the current hardware:
// virtual session → default profile; known fingerprint → matching profile;
// unknown combination → auto-arranged temporary profile.
func ResolveAuto(disc []xrandr.Output, profiles map[string]*profile.Profile, defaultProfile *profile.Profile) Decision {
	refs := fingerprint.FromOutputs(disc)
	if fingerprint.HasVirtual(refs) && defaultProfile != nil {
		return Decision{Profile: defaultProfile, Name: "default", Reason: ReasonVirtualDefault}
	}

	id := fingerprint.Compute(refs)
	names := make([]string, 0, len(profiles))
	for n := range profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if profiles[n].MatchID() == id {
			return Decision{Profile: profiles[n], Name: n, Reason: ReasonFingerprintMatch}
		}
	}

	return Decision{Profile: AutoArrange(disc), Name: "auto", Generated: true, Reason: ReasonAutoArranged}
}
