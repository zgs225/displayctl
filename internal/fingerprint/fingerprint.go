// Package fingerprint identifies a hardware display combination so that
// displayctl can auto-select the right profile when monitors are plugged or
// unplugged. The identity is derived from each connected output's name and
// EDID; virtual outputs (XRDP/Sunshine's Virtual-1) have no EDID and fall back
// to their name, which keeps the original single-output design working.
package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/zgs225/displayctl/internal/xrandr"
)

// Ref identifies one connected output for fingerprinting purposes.
type Ref struct {
	Name string // xrandr output name, e.g. "eDP-1"
	EDID string // full EDID hex string; "" for virtual/edid-less outputs
}

// Label returns a compact, human-readable identifier for the ref:
// "eDP-1:ab12cd34" for physical outputs, bare "Virtual-1" for virtual ones.
func (r Ref) Label() string {
	if r.EDID == "" {
		return r.Name
	}
	sum := sha256.Sum256([]byte(r.EDID))
	return r.Name + ":" + hex.EncodeToString(sum[:])[:8]
}

// Compute returns a stable, order-independent fingerprint id for a set of refs.
func Compute(refs []Ref) string {
	labels := make([]string, len(refs))
	for i, r := range refs {
		labels[i] = r.Label()
	}
	sort.Strings(labels)
	sum := sha256.Sum256([]byte(strings.Join(labels, ";")))
	return hex.EncodeToString(sum[:])[:16]
}

// FromOutputs builds fingerprint refs from connected xrandr outputs, skipping
// disconnected ones.
func FromOutputs(outs []xrandr.Output) []Ref {
	var refs []Ref
	for _, o := range outs {
		if !o.Connected {
			continue
		}
		refs = append(refs, Ref{Name: o.Name, EDID: o.EDID})
	}
	return refs
}

// HasVirtual reports whether any ref lacks an EDID, indicating a virtual X
// session (XRDP/Sunshine) where EDID-based matching is impossible.
func HasVirtual(refs []Ref) bool {
	for _, r := range refs {
		if r.EDID == "" {
			return true
		}
	}
	return false
}
