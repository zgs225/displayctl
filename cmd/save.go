package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zgs225/displayctl/config"
	"github.com/zgs225/displayctl/internal/dpi"
	"github.com/zgs225/displayctl/internal/fingerprint"
	"github.com/zgs225/displayctl/internal/layout"
	"github.com/zgs225/displayctl/internal/profile"
	"github.com/zgs225/displayctl/internal/xrandr"
)

func newSaveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "save <name>",
		Short: "Save the current display state as a named profile",
		Args:  cobra.ExactArgs(1),
		RunE:  runSave,
	}
}

func runSave(cmd *cobra.Command, args []string) error {
	name := args[0]
	cfgDir := config.ConfigDir()

	disc, err := xrandr.Discover()
	if err != nil {
		return fmt.Errorf("discover outputs: %w", err)
	}

	p := layout.Capture(disc)
	if len(p.Outputs) == 0 {
		return fmt.Errorf("no active outputs to save")
	}

	// Physical profiles pin the DPI they were captured at instead of keeping
	// width tiers: X11 has one global DPI, and a tier would re-render a saved
	// layout differently as soon as the reference width changes. Virtual
	// sessions (no EDID) keep tiers — their resolution is unknown until a
	// client connects.
	refs := fingerprint.FromOutputs(disc)
	if !fingerprint.HasVirtual(refs) {
		fixed := config.LoadSettings().FixedDPI()
		if cur, err := dpi.GetCurrentDPI(); err == nil && cur > 0 {
			fixed = cur
		}
		if fixed > 0 {
			v := fixed
			p.DPI = profile.DPIConfig{Value: &v, Reference: p.DPI.Reference}
		}
		// Also record the name-independent fingerprint so the profile keeps
		// matching when the dock re-enumerates the same panel under another
		// output name (DP-1 ↔ DP-1-1).
		if id := fingerprint.ComputeEDIDs(refs); id != "" && p.Match != nil {
			p.Match.IDs = append(p.Match.IDs, id)
		}
	}

	if err := profile.Save(cfgDir, name, p); err != nil {
		return err
	}

	fmt.Printf("saved profile %q: layout=%s outputs=%s fingerprint=%s\n",
		name, p.Layout, strings.Join(p.OutputNames(), ","), strings.Join(p.MatchIDs(), ","))
	return nil
}
