package layout

import (
	"testing"

	"github.com/zgs225/displayctl/internal/fingerprint"
	"github.com/zgs225/displayctl/internal/profile"
	"github.com/zgs225/displayctl/internal/xrandr"
)

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

func discovered(outputs ...xrandr.Output) []xrandr.Output { return outputs }

func edp() xrandr.Output {
	return xrandr.Output{
		Name: "eDP-1", Connected: true, Primary: true, Mode: "2560x1600",
		EDID: "edidEDP",
		Modes: []xrandr.Mode{{Name: "2560x1600", Preferred: true, Current: true}},
	}
}

func dp1() xrandr.Output {
	return xrandr.Output{
		Name: "DP-1", Connected: true, Mode: "3840x2160", PosX: 2560,
		EDID: "edidDP1",
		Modes: []xrandr.Mode{{Name: "3840x2160", Preferred: true, Current: true}},
	}
}

func dockProfile() *profile.Profile {
	return &profile.Profile{
		Layout: "extend",
		Outputs: map[string]*profile.OutputConfig{
			"eDP-1": {Mode: "2560x1600", Pos: "0x0", Primary: true, Rate: 120},
			"DP-1":  {Mode: "3840x2160", Pos: "2560x0"},
		},
		DPI: profile.DPIConfig{Tiers: boolPtr(true), Reference: "eDP-1"},
	}
}

func hasSeq(args []string, seq ...string) bool {
	for i := 0; i+len(seq) <= len(args); i++ {
		match := true
		for j, s := range seq {
			if args[i+j] != s {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func TestPlanExtendArgs(t *testing.T) {
	plan, err := Build(dockProfile(), discovered(edp(), dp1()))
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if !hasSeq(plan.Args, "--output", "eDP-1", "--mode", "2560x1600") {
		t.Errorf("missing eDP-1 mode args: %v", plan.Args)
	}
	if !hasSeq(plan.Args, "--pos", "0x0") || !hasSeq(plan.Args, "--primary") {
		t.Errorf("eDP-1 should have pos 0x0 and primary: %v", plan.Args)
	}
	if !hasSeq(plan.Args, "--rate", "120") {
		t.Errorf("eDP-1 should have rate 120: %v", plan.Args)
	}
	if !hasSeq(plan.Args, "--output", "DP-1", "--mode", "3840x2160") {
		t.Errorf("missing DP-1 mode args: %v", plan.Args)
	}
	if !hasSeq(plan.Args, "--pos", "2560x0") {
		t.Errorf("DP-1 should have pos 2560x0: %v", plan.Args)
	}
}

func TestPlanDPIFromReference(t *testing.T) {
	plan, err := Build(dockProfile(), discovered(edp(), dp1()))
	if err != nil {
		t.Fatal(err)
	}
	if plan.DPI != 144 { // tiers(2560) = 144
		t.Errorf("DPI = %d, want 144 (tiers off eDP-1 width 2560)", plan.DPI)
	}

	// Reference the 4K output instead
	p := dockProfile()
	p.DPI.Reference = "DP-1"
	plan2, _ := Build(p, discovered(edp(), dp1()))
	if plan2.DPI != 192 { // tiers(3840) = 192
		t.Errorf("DPI = %d, want 192 (tiers off DP-1 width 3840)", plan2.DPI)
	}
}

func TestPlanDPIFixedValue(t *testing.T) {
	p := dockProfile()
	p.DPI = profile.DPIConfig{Value: intPtr(168)}
	plan, _ := Build(p, discovered(edp(), dp1()))
	if plan.DPI != 168 {
		t.Errorf("DPI = %d, want fixed 168", plan.DPI)
	}
}

func TestPlanTurnsOffUnlistedWhenFull(t *testing.T) {
	hdmi := xrandr.Output{Name: "HDMI-1-0", Connected: true, EDID: "edidHDMI"}
	plan, err := Build(dockProfile(), discovered(edp(), dp1(), hdmi))
	if err != nil {
		t.Fatal(err)
	}
	if !hasSeq(plan.Args, "--output", "HDMI-1-0", "--off") {
		t.Errorf("manage=full should turn off unlisted HDMI-1-0: %v", plan.Args)
	}
}

func TestPlanPartialKeepsUnlisted(t *testing.T) {
	p := dockProfile()
	p.Manage = "partial"
	hdmi := xrandr.Output{Name: "HDMI-1-0", Connected: true, EDID: "edidHDMI"}
	plan, _ := Build(p, discovered(edp(), dp1(), hdmi))
	if hasSeq(plan.Args, "--output", "HDMI-1-0", "--off") {
		t.Errorf("manage=partial must NOT turn off unlisted outputs: %v", plan.Args)
	}
}

func TestPlanPreferredModeResolved(t *testing.T) {
	p := &profile.Profile{
		Outputs: map[string]*profile.OutputConfig{
			"DP-1": {Mode: "preferred", Pos: "0x0", Primary: true},
		},
		DPI: profile.DPIConfig{Tiers: boolPtr(true)},
	}
	plan, err := Build(p, discovered(dp1()))
	if err != nil {
		t.Fatal(err)
	}
	if !hasSeq(plan.Args, "--output", "DP-1", "--mode", "3840x2160") {
		t.Errorf("preferred should resolve to 3840x2160: %v", plan.Args)
	}
}

func TestPlanOffOutput(t *testing.T) {
	p := &profile.Profile{
		Outputs: map[string]*profile.OutputConfig{
			"eDP-1": {Mode: "2560x1600", Pos: "0x0", Primary: true},
			"DP-1":  {Mode: "off"},
		},
		DPI: profile.DPIConfig{Tiers: boolPtr(true)},
	}
	plan, _ := Build(p, discovered(edp(), dp1()))
	if !hasSeq(plan.Args, "--output", "DP-1", "--off") {
		t.Errorf("mode=off should emit --off: %v", plan.Args)
	}
}

func TestAutoArrangeExtend(t *testing.T) {
	p := AutoArrange(discovered(edp(), dp1()))
	if p.Layout != "extend" {
		t.Errorf("layout = %q, want extend", p.Layout)
	}
	if len(p.Outputs) != 2 {
		t.Fatalf("outputs = %d, want 2", len(p.Outputs))
	}
	edpCfg := p.Outputs["eDP-1"]
	if edpCfg == nil || !edpCfg.Primary || edpCfg.Pos != "0x0" {
		t.Errorf("eDP-1 (internal) should be primary at 0x0: %+v", edpCfg)
	}
	if edpCfg.Mode != "2560x1600" {
		t.Errorf("eDP-1 mode = %q, want preferred 2560x1600", edpCfg.Mode)
	}
	dpCfg := p.Outputs["DP-1"]
	if dpCfg == nil || dpCfg.Pos != "2560x0" {
		t.Errorf("DP-1 should be placed right of eDP-1 at 2560x0: %+v", dpCfg)
	}
	if dpCfg.Mode != "3840x2160" {
		t.Errorf("DP-1 mode = %q, want preferred 3840x2160", dpCfg.Mode)
	}
}

func TestAutoArrangeSingle(t *testing.T) {
	p := AutoArrange(discovered(edp()))
	if p.Layout != "single" {
		t.Errorf("layout = %q, want single", p.Layout)
	}
	if len(p.Outputs) != 1 {
		t.Errorf("outputs = %d, want 1", len(p.Outputs))
	}
}

func TestResolveAutoVirtualUsesDefault(t *testing.T) {
	def := &profile.Profile{Default: true, Output: &profile.LegacyOutput{Name: "Virtual-1", Mode: "current"}}
	virt := xrandr.Output{Name: "Virtual-1", Connected: true, Mode: "1920x1080", EDID: ""}
	profiles := map[string]*profile.Profile{"monitor": def}

	d := ResolveAuto(discovered(virt), profiles, def)
	if d.Reason != ReasonVirtualDefault {
		t.Errorf("reason = %q, want %q", d.Reason, ReasonVirtualDefault)
	}
	if d.Profile != def {
		t.Error("virtual session should use the default profile")
	}
}

func TestResolveAutoFingerprintMatch(t *testing.T) {
	dock := dockProfile()
	// Compute the fingerprint that ResolveAuto would produce for edp+dp1
	id := fingerprint.Compute(fingerprint.FromOutputs(discovered(edp(), dp1())))
	dock.Match = &profile.Match{ID: id}

	profiles := map[string]*profile.Profile{"dock": dock}
	d := ResolveAuto(discovered(edp(), dp1()), profiles, nil)
	if d.Reason != ReasonFingerprintMatch {
		t.Errorf("reason = %q, want %q", d.Reason, ReasonFingerprintMatch)
	}
	if d.Name != "dock" || d.Generated {
		t.Errorf("should match dock profile, got name=%q generated=%v", d.Name, d.Generated)
	}
}

func TestResolveAutoGeneratesWhenNoMatch(t *testing.T) {
	profiles := map[string]*profile.Profile{} // no profiles
	d := ResolveAuto(discovered(edp(), dp1()), profiles, nil)
	if d.Reason != ReasonAutoArranged {
		t.Errorf("reason = %q, want %q", d.Reason, ReasonAutoArranged)
	}
	if !d.Generated || d.Profile == nil {
		t.Error("should auto-arrange a generated profile")
	}
	if d.Profile.Layout != "extend" {
		t.Errorf("generated layout = %q, want extend", d.Profile.Layout)
	}
}
