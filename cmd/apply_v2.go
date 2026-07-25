package cmd

import (
	"fmt"
	"os"

	"github.com/zgs225/displayctl/internal/dpi"
	"github.com/zgs225/displayctl/internal/hook"
	"github.com/zgs225/displayctl/internal/layout"
	"github.com/zgs225/displayctl/internal/profile"
	"github.com/zgs225/displayctl/internal/xrandr"
)

// applyProfileV2 applies a v2 multi-output profile: discover current outputs,
// build a concrete xrandr plan, apply it, set the global DPI, run hooks.
func applyProfileV2(cfgDir string, p *profile.Profile) error {
	disc, err := xrandr.Discover()
	if err != nil {
		return fmt.Errorf("discover outputs: %w", err)
	}

	plan, err := layout.Build(p, disc)
	if err != nil {
		return err
	}

	if len(plan.Args) > 0 {
		if err := xrandr.Apply(plan.Args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(3)
		}
	}

	if plan.DPI > 0 {
		if err := dpi.SetXrandrDPI(plan.DPI); err != nil {
			fmt.Fprintf(os.Stderr, "warning: set xrandr dpi: %v\n", err)
		}
		if err := dpi.SetXftDPI(plan.DPI); err != nil {
			return fmt.Errorf("set xft dpi: %w", err)
		}
		if err := dpi.SetXcursorSize(dpi.CalculateXcursorSize(plan.DPI)); err != nil {
			fmt.Fprintf(os.Stderr, "warning: set xcursor size: %v\n", err)
		}
		fmt.Printf("layout=%s reference=%s mode=%s dpi=%d\n", p.Layout, plan.Reference, plan.Mode, plan.DPI)
	} else {
		fmt.Printf("layout=%s dpi=unchanged\n", p.Layout)
	}

	if err := hook.RunPostSwitch(cfgDir, plan.HookEnv); err != nil {
		fmt.Fprintf(os.Stderr, "warning: post-switch hooks: %v\n", err)
	}

	return nil
}
