package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func writeProfile(t *testing.T, dir, name, body string) {
	t.Helper()
	pdir := filepath.Join(dir, "profiles")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, name+".toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const v2Body = `default = false
layout = "extend"
manage = "full"

[match]
id = "9f2a7c1bdeadbeef"
outputs = ["DP-1:ab12cd34", "eDP-1:ef56ab78"]

[outputs.eDP-1]
mode = "2560x1600"
rate = 120
pos = "0x0"
primary = true

[outputs.DP-1]
mode = "3840x2160"
pos = "2560x0"

[dpi]
tiers = true
reference = "eDP-1"
`

const legacyBody = `default = true

[output]
name = "Virtual-1"
mode = "current"

[dpi]
tiers = true
`

func TestParseV2(t *testing.T) {
	dir := t.TempDir()
	writeProfile(t, dir, "dock", v2Body)

	p, err := Load(dir, "dock")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.Layout != "extend" {
		t.Errorf("layout = %q, want extend", p.Layout)
	}
	if p.Manage != "full" {
		t.Errorf("manage = %q, want full", p.Manage)
	}
	if p.Match == nil || p.Match.ID != "9f2a7c1bdeadbeef" {
		t.Fatalf("match.id not parsed: %+v", p.Match)
	}
	if len(p.Match.Outputs) != 2 {
		t.Errorf("match.outputs len = %d, want 2", len(p.Match.Outputs))
	}
	if len(p.Outputs) != 2 {
		t.Fatalf("outputs len = %d, want 2", len(p.Outputs))
	}
	edp := p.Outputs["eDP-1"]
	if edp == nil {
		t.Fatal("eDP-1 output missing")
	}
	if edp.Mode != "2560x1600" || edp.Rate != 120 || edp.Pos != "0x0" || !edp.Primary {
		t.Errorf("eDP-1 = %+v, want mode 2560x1600 rate 120 pos 0x0 primary", edp)
	}
	dp1 := p.Outputs["DP-1"]
	if dp1 == nil || dp1.Mode != "3840x2160" || dp1.Pos != "2560x0" {
		t.Errorf("DP-1 = %+v, want mode 3840x2160 pos 2560x0", dp1)
	}
	if p.DPI.Reference != "eDP-1" {
		t.Errorf("dpi.reference = %q, want eDP-1", p.DPI.Reference)
	}
	if p.DPI.Tiers == nil || !*p.DPI.Tiers {
		t.Error("dpi.tiers should be true")
	}
}

func TestParseLegacy(t *testing.T) {
	dir := t.TempDir()
	writeProfile(t, dir, "xrdp", legacyBody)

	p, err := Load(dir, "xrdp")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !p.Default {
		t.Error("legacy default should be true")
	}
	if p.Output == nil {
		t.Fatal("legacy [output] not parsed")
	}
	if p.Output.Name != "Virtual-1" || p.Output.Mode != "current" {
		t.Errorf("legacy output = %+v, want Virtual-1/current", p.Output)
	}
	if len(p.Outputs) != 0 {
		t.Errorf("legacy profile should have no v2 outputs, got %d", len(p.Outputs))
	}
	if !p.IsLegacy() {
		t.Error("IsLegacy should be true for [output] profiles")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	writeProfile(t, dir, "dock", v2Body)
	p, err := Load(dir, "dock")
	if err != nil {
		t.Fatal(err)
	}

	dir2 := t.TempDir()
	if err := Save(dir2, "dock", p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	p2, err := Load(dir2, "dock")
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if p2.Layout != p.Layout || len(p2.Outputs) != len(p.Outputs) {
		t.Errorf("round-trip mismatch: %+v vs %+v", p2, p)
	}
	if p2.Match == nil || p2.Match.ID != p.Match.ID {
		t.Errorf("round-trip match lost: %+v", p2.Match)
	}
	if p2.Outputs["DP-1"].Pos != "2560x0" {
		t.Errorf("round-trip DP-1 pos lost: %+v", p2.Outputs["DP-1"])
	}
}

func TestLoadDefault(t *testing.T) {
	dir := t.TempDir()
	writeProfile(t, dir, "dock", v2Body)     // default=false
	writeProfile(t, dir, "xrdp", legacyBody) // default=true

	p, err := LoadDefault(dir)
	if err != nil {
		t.Fatalf("LoadDefault: %v", err)
	}
	if !p.Default {
		t.Error("LoadDefault should return the default profile")
	}
}

func TestFindByFingerprint(t *testing.T) {
	dir := t.TempDir()
	writeProfile(t, dir, "dock", v2Body) // match.id = 9f2a7c1bdeadbeef
	writeProfile(t, dir, "xrdp", legacyBody)

	name, p, err := FindByFingerprint(dir, "9f2a7c1bdeadbeef")
	if err != nil {
		t.Fatalf("FindByFingerprint: %v", err)
	}
	if name != "dock" || p == nil {
		t.Errorf("FindByFingerprint = %q, want dock", name)
	}

	if _, _, err := FindByFingerprint(dir, "nomatch"); err == nil {
		t.Error("FindByFingerprint should error on no match")
	}
}

func TestLoadAll(t *testing.T) {
	dir := t.TempDir()
	writeProfile(t, dir, "dock", v2Body)
	writeProfile(t, dir, "xrdp", legacyBody)

	all := LoadAll(dir)
	if len(all) != 2 {
		t.Fatalf("LoadAll returned %d profiles, want 2", len(all))
	}
	if all["dock"] == nil || all["xrdp"] == nil {
		t.Error("LoadAll missing expected profiles")
	}

	name, p, err := LoadDefaultName(dir)
	if err != nil || name != "xrdp" || !p.Default {
		t.Errorf("LoadDefaultName = %q, %v; want xrdp default", name, err)
	}
}
