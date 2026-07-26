package xrandr

import (
	"os"
	"strings"
	"testing"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

func findOutput(t *testing.T, outs []Output, name string) Output {
	t.Helper()
	for _, o := range outs {
		if o.Name == name {
			return o
		}
	}
	t.Fatalf("output %s not found in parsed result", name)
	return Output{}
}

func TestParseSingleActive(t *testing.T) {
	outs, err := Parse(readFixture(t, "current_single.txt"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	edp := findOutput(t, outs, "eDP-1")
	if !edp.Connected {
		t.Error("eDP-1 should be connected")
	}
	if !edp.Primary {
		t.Error("eDP-1 should be primary")
	}
	if edp.Mode != "2560x1600" {
		t.Errorf("eDP-1 mode = %q, want 2560x1600", edp.Mode)
	}
	if edp.PosX != 0 || edp.PosY != 0 {
		t.Errorf("eDP-1 pos = %d,%d, want 0,0", edp.PosX, edp.PosY)
	}
	if edp.WidthMM != 344 || edp.HeightMM != 215 {
		t.Errorf("eDP-1 mm = %dx%d, want 344x215", edp.WidthMM, edp.HeightMM)
	}

	dp1 := findOutput(t, outs, "DP-1")
	if !dp1.Connected {
		t.Error("DP-1 should be connected")
	}
	if dp1.Mode != "" {
		t.Errorf("DP-1 should be inactive, got mode %q", dp1.Mode)
	}

	dp2 := findOutput(t, outs, "DP-2")
	if dp2.Connected {
		t.Error("DP-2 should be disconnected")
	}
}

func TestParseDualActive(t *testing.T) {
	outs, err := Parse(readFixture(t, "current_dual.txt"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	edp := findOutput(t, outs, "eDP-1")
	if edp.Mode != "2560x1600" || edp.PosX != 0 || edp.PosY != 0 {
		t.Errorf("eDP-1 = %q pos %d,%d; want 2560x1600 pos 0,0", edp.Mode, edp.PosX, edp.PosY)
	}
	if !edp.Primary {
		t.Error("eDP-1 should be primary")
	}

	dp1 := findOutput(t, outs, "DP-1")
	if dp1.Mode != "3840x2160" {
		t.Errorf("DP-1 mode = %q, want 3840x2160", dp1.Mode)
	}
	if dp1.PosX != 2560 || dp1.PosY != 0 {
		t.Errorf("DP-1 pos = %d,%d, want 2560,0", dp1.PosX, dp1.PosY)
	}
	if dp1.Primary {
		t.Error("DP-1 should not be primary")
	}
	if dp1.WidthMM != 597 || dp1.HeightMM != 336 {
		t.Errorf("DP-1 mm = %dx%d, want 597x336", dp1.WidthMM, dp1.HeightMM)
	}
}

func TestParseModeFlags(t *testing.T) {
	outs, err := Parse(readFixture(t, "current_dual.txt"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	edp := findOutput(t, outs, "eDP-1")

	var cur *Mode
	for i := range edp.Modes {
		if edp.Modes[i].Name == "2560x1600" {
			cur = &edp.Modes[i]
		}
	}
	if cur == nil {
		t.Fatal("mode 2560x1600 not found")
	}
	if !cur.Current {
		t.Error("2560x1600 should be current (*)")
	}
	if !cur.Preferred {
		t.Error("2560x1600 should be preferred (+)")
	}
}

func TestPreferredMode(t *testing.T) {
	outs, err := Parse(readFixture(t, "current_dual.txt"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	dp1 := findOutput(t, outs, "DP-1")
	if got := dp1.PreferredMode(); got != "3840x2160" {
		t.Errorf("DP-1 preferred = %q, want 3840x2160", got)
	}
}

func TestParseEDID(t *testing.T) {
	outs, err := Parse(readFixture(t, "verbose_dual.txt"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	edp := findOutput(t, outs, "eDP-1")
	if !strings.HasPrefix(edp.EDID, "00ffffffffffff0006af") {
		t.Errorf("eDP-1 EDID prefix = %q, want 00ffffffffffff0006af...", trunc(edp.EDID, 24))
	}
	dp1 := findOutput(t, outs, "DP-1")
	if !strings.HasPrefix(dp1.EDID, "00ffffffffffff0010ac") {
		t.Errorf("DP-1 EDID prefix = %q, want 00ffffffffffff0010ac...", trunc(dp1.EDID, 24))
	}
}

func TestScreenSize(t *testing.T) {
	w, h, err := ScreenSize(readFixture(t, "current_dual.txt"))
	if err != nil {
		t.Fatalf("ScreenSize: %v", err)
	}
	if w != 6400 || h != 2160 {
		t.Errorf("screen = %dx%d, want 6400x2160", w, h)
	}
}

func TestModeWidth(t *testing.T) {
	outs, err := Parse(readFixture(t, "current_dual.txt"))
	if err != nil {
		t.Fatal(err)
	}
	dp1 := findOutput(t, outs, "DP-1")
	if got := dp1.ModeWidth(); got != 3840 {
		t.Errorf("DP-1 ModeWidth = %d, want 3840", got)
	}
	inactive := Output{Name: "X", Connected: true}
	if got := inactive.ModeWidth(); got != 0 {
		t.Errorf("inactive ModeWidth = %d, want 0", got)
	}
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
