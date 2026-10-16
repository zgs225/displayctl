package cmd

import (
	"testing"

	"github.com/zgs225/displayctl/internal/profile"
	"github.com/zgs225/displayctl/internal/xrandr"
)

// resizeStubs replaces the resize path's side effects with recorders so
// refreshDPI can be exercised without a live X server or real hooks.
type resizeStubs struct {
	dpi        int
	xcursor    int
	hookEnv    map[string]string
	hookCalled bool
}

func stubResize(t *testing.T) *resizeStubs {
	t.Helper()
	s := &resizeStubs{}
	origXrandr, origXft, origCursor, origHook := setXrandrDPI, setXftDPI, setXcursorSize, runPostSwitch
	t.Cleanup(func() {
		setXrandrDPI, setXftDPI, setXcursorSize, runPostSwitch = origXrandr, origXft, origCursor, origHook
	})
	setXrandrDPI = func(v int) error { s.dpi = v; return nil }
	setXftDPI = func(v int) error { return nil }
	setXcursorSize = func(v int) error { s.xcursor = v; return nil }
	runPostSwitch = func(dir string, env map[string]string) error {
		s.hookCalled = true
		s.hookEnv = env
		return nil
	}
	return s
}

func intPtr(v int) *int { return &v }

func activePrimary(mode string) []xrandr.Output {
	return []xrandr.Output{
		{Name: "eDP-1", Connected: true, Primary: true, Mode: mode},
	}
}

// A profile with a fixed [dpi] value must be honored on a pure resize, even
// when the width-based tier would pick something else (2560px → 144).
func TestRefreshDPIResizeHonorsProfileFixedValue(t *testing.T) {
	s := stubResize(t)
	lastProfile = &profile.Profile{DPI: profile.DPIConfig{Value: intPtr(168)}}
	t.Cleanup(func() { lastProfile = nil })

	refreshDPI(t.TempDir(), activePrimary("2560x1600"))

	if s.dpi != 168 {
		t.Errorf("refreshDPI dpi = %d, want 168 (profile fixed value)", s.dpi)
	}
	if s.xcursor != 42 { // 24 * 168 / 96
		t.Errorf("xcursor size = %d, want 42", s.xcursor)
	}
	if !s.hookCalled {
		t.Fatal("post-switch hooks were not run")
	}
	if got := s.hookEnv["DISPLAYCTL_DPI"]; got != "168" {
		t.Errorf("hook DISPLAYCTL_DPI = %q, want 168", got)
	}
}

// Legacy [output] profiles (e.g. virtual-default / xrdp) are recorded the same
// way, so their fixed DPI also survives a resize.
func TestRefreshDPIResizeHonorsLegacyProfileFixedValue(t *testing.T) {
	s := stubResize(t)
	lastProfile = &profile.Profile{
		Output: &profile.LegacyOutput{Name: "eDP-1", Mode: "current"},
		DPI:    profile.DPIConfig{Value: intPtr(192)},
	}
	t.Cleanup(func() { lastProfile = nil })

	refreshDPI(t.TempDir(), activePrimary("2560x1600"))

	if s.dpi != 192 {
		t.Errorf("refreshDPI dpi = %d, want 192 (legacy profile fixed value)", s.dpi)
	}
	if got := s.hookEnv["DISPLAYCTL_DPI"]; got != "192" {
		t.Errorf("hook DISPLAYCTL_DPI = %q, want 192", got)
	}
}

// Tier-based (or unknown) profiles keep the historical width-based behaviour.
func TestRefreshDPIResizeFallsBackToTiers(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode string
		want int
	}{
		{"2560px -> 144", "2560x1600", 144},
		{"3840px -> 192", "3840x2160", 192},
		{"1920px -> 96", "1920x1080", 96},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := stubResize(t)
			tiers := true
			lastProfile = &profile.Profile{DPI: profile.DPIConfig{Tiers: &tiers}}
			t.Cleanup(func() { lastProfile = nil })

			refreshDPI(t.TempDir(), activePrimary(tc.mode))

			if s.dpi != tc.want {
				t.Errorf("refreshDPI dpi = %d, want %d (tiers)", s.dpi, tc.want)
			}
		})
	}
}

// No remembered profile at all (e.g. no profile resolved yet) → tiers.
func TestEffectiveDPINilProfileUsesTiers(t *testing.T) {
	if got := effectiveDPI(nil, 2560); got != 144 {
		t.Errorf("effectiveDPI(nil, 2560) = %d, want 144", got)
	}
}
