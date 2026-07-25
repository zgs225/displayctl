package fingerprint

import (
	"testing"

	"github.com/zgs225/displayctl/internal/xrandr"
)

func TestComputeOrderIndependent(t *testing.T) {
	a := Ref{Name: "eDP-1", EDID: "edidA"}
	b := Ref{Name: "DP-1", EDID: "edidB"}
	if Compute([]Ref{a, b}) != Compute([]Ref{b, a}) {
		t.Error("Compute must be order-independent")
	}
}

func TestComputeDeterministic(t *testing.T) {
	refs := []Ref{{Name: "eDP-1", EDID: "edidA"}, {Name: "DP-1", EDID: "edidB"}}
	if Compute(refs) != Compute(refs) {
		t.Error("Compute must be deterministic")
	}
}

func TestComputeDistinctSets(t *testing.T) {
	single := []Ref{{Name: "eDP-1", EDID: "edidA"}}
	dual := []Ref{{Name: "eDP-1", EDID: "edidA"}, {Name: "DP-1", EDID: "edidB"}}
	if Compute(single) == Compute(dual) {
		t.Error("different hardware sets must produce different fingerprints")
	}
}

func TestComputeEDIDSensitive(t *testing.T) {
	// Same names, different EDID (different physical monitor on same port)
	a := []Ref{{Name: "DP-1", EDID: "edidA"}}
	b := []Ref{{Name: "DP-1", EDID: "edidB"}}
	if Compute(a) == Compute(b) {
		t.Error("different EDID on same output must produce different fingerprints")
	}
}

func TestLabel(t *testing.T) {
	phys := Ref{Name: "eDP-1", EDID: "00ffffffffffff0006af"}
	if got := phys.Label(); got == "eDP-1" || len(got) <= len("eDP-1:") {
		t.Errorf("physical label should carry an edid suffix, got %q", got)
	}
	virt := Ref{Name: "Virtual-1", EDID: ""}
	if got := virt.Label(); got != "Virtual-1" {
		t.Errorf("virtual label = %q, want bare name Virtual-1", got)
	}
}

func TestFromOutputsSkipsDisconnected(t *testing.T) {
	outs := []xrandr.Output{
		{Name: "eDP-1", Connected: true, EDID: "edidA"},
		{Name: "DP-1", Connected: true, EDID: "edidB"},
		{Name: "DP-2", Connected: false},
	}
	refs := FromOutputs(outs)
	if len(refs) != 2 {
		t.Fatalf("FromOutputs returned %d refs, want 2 (disconnected skipped)", len(refs))
	}
}

func TestHasVirtual(t *testing.T) {
	if HasVirtual([]Ref{{Name: "eDP-1", EDID: "x"}}) {
		t.Error("all-edid set should not be virtual")
	}
	if !HasVirtual([]Ref{{Name: "eDP-1", EDID: "x"}, {Name: "Virtual-1", EDID: ""}}) {
		t.Error("set containing an edid-less output should be virtual")
	}
}
