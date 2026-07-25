package layout

import (
	"testing"

	"github.com/zgs225/displayctl/internal/xrandr"
)

func edpActive() xrandr.Output {
	return xrandr.Output{
		Name: "eDP-1", Connected: true, Primary: true, Mode: "2560x1600",
		PosX: 0, PosY: 0, EDID: "edidEDP",
		Modes: []xrandr.Mode{
			{Name: "2560x1600", Preferred: true, Current: true},
			{Name: "1920x1080"},
		},
	}
}

func dp1ActiveRight() xrandr.Output {
	return xrandr.Output{
		Name: "DP-1", Connected: true, Mode: "3840x2160",
		PosX: 2560, PosY: 0, EDID: "edidDP1",
		Modes: []xrandr.Mode{
			{Name: "3840x2160", Preferred: true, Current: true},
			{Name: "1920x1080"},
		},
	}
}

func TestCaptureExtend(t *testing.T) {
	p := Capture(discovered(edpActive(), dp1ActiveRight()))
	if p.Layout != "extend" {
		t.Errorf("layout = %q, want extend", p.Layout)
	}
	if p.ManageMode() != "full" {
		t.Errorf("manage = %q, want full", p.ManageMode())
	}
	edp := p.Outputs["eDP-1"]
	if edp == nil || edp.Mode != "2560x1600" || edp.Pos != "0x0" || !edp.Primary {
		t.Errorf("eDP-1 captured = %+v, want 2560x1600 0x0 primary", edp)
	}
	dp := p.Outputs["DP-1"]
	if dp == nil || dp.Mode != "3840x2160" || dp.Pos != "2560x0" {
		t.Errorf("DP-1 captured = %+v, want 3840x2160 2560x0", dp)
	}
	if p.Match == nil || p.Match.ID == "" {
		t.Error("capture must compute a fingerprint match.id")
	}
	if len(p.Match.Outputs) != 2 {
		t.Errorf("match.outputs = %d, want 2 labels", len(p.Match.Outputs))
	}
	if p.DPI.Reference != "eDP-1" {
		t.Errorf("dpi.reference = %q, want primary eDP-1", p.DPI.Reference)
	}
}

func TestCaptureMirror(t *testing.T) {
	a := edpActive()
	b := dp1ActiveRight()
	b.Mode = "1920x1080"
	b.PosX = 0
	b.PosY = 0
	a.Mode = "1920x1080"
	p := Capture(discovered(a, b))
	if p.Layout != "mirror" {
		t.Errorf("layout = %q, want mirror (same pos+mode)", p.Layout)
	}
}

func TestCaptureSingle(t *testing.T) {
	p := Capture(discovered(edpActive()))
	if p.Layout != "single" {
		t.Errorf("layout = %q, want single", p.Layout)
	}
	if len(p.Outputs) != 1 {
		t.Errorf("outputs = %d, want 1", len(p.Outputs))
	}
}

func TestCaptureSkipsInactiveConnected(t *testing.T) {
	inactive := xrandr.Output{Name: "DP-1", Connected: true, Mode: "", EDID: "edidDP1"}
	p := Capture(discovered(edpActive(), inactive))
	if _, ok := p.Outputs["DP-1"]; ok {
		t.Error("inactive connected output should not be captured")
	}
	// but it IS part of the fingerprint (it's connected)
	if len(p.Match.Outputs) != 2 {
		t.Errorf("fingerprint should include both connected outputs, got %d", len(p.Match.Outputs))
	}
}

func TestMirrorCommonMode(t *testing.T) {
	p := Mirror(discovered(edpActive(), dp1ActiveRight()))
	if p.Layout != "mirror" {
		t.Errorf("layout = %q, want mirror", p.Layout)
	}
	edp := p.Outputs["eDP-1"]
	dp := p.Outputs["DP-1"]
	if edp == nil || dp == nil {
		t.Fatalf("mirror should include both outputs: %+v", p.Outputs)
	}
	// common best mode shared by both is 1920x1080
	if edp.Mode != "1920x1080" || dp.Mode != "1920x1080" {
		t.Errorf("mirror mode = %s/%s, want common 1920x1080", edp.Mode, dp.Mode)
	}
	if edp.Pos != "0x0" || dp.Pos != "0x0" {
		t.Errorf("mirror outputs must share pos 0x0: %s/%s", edp.Pos, dp.Pos)
	}
	if !edp.Primary {
		t.Error("internal output should be primary in mirror")
	}
}

func TestSingleOutput(t *testing.T) {
	p, err := Single(discovered(edpActive(), dp1ActiveRight()), "DP-1")
	if err != nil {
		t.Fatalf("Single: %v", err)
	}
	if p.Layout != "single" {
		t.Errorf("layout = %q, want single", p.Layout)
	}
	if len(p.Outputs) != 1 {
		t.Fatalf("outputs = %d, want 1 (only DP-1)", len(p.Outputs))
	}
	dp := p.Outputs["DP-1"]
	if dp == nil || !dp.Primary || dp.Mode != "3840x2160" {
		t.Errorf("DP-1 = %+v, want preferred 3840x2160 primary", dp)
	}
	if p.ManageMode() != "full" {
		t.Error("single must be manage=full so other outputs turn off")
	}
}

func TestSingleDisconnectedError(t *testing.T) {
	if _, err := Single(discovered(edpActive()), "HDMI-1-0"); err == nil {
		t.Error("Single on a disconnected output should error")
	}
}
