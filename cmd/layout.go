package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zgs225/displayctl/config"
	"github.com/zgs225/displayctl/internal/layout"
	"github.com/zgs225/displayctl/internal/profile"
	"github.com/zgs225/displayctl/internal/xrandr"
)

func newLayoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "layout <extend|mirror|single <output>>",
		Short: "Apply a quick layout (extend/mirror/single) without a saved profile",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  runLayout,
	}
}

func runLayout(cmd *cobra.Command, args []string) error {
	cfgDir := config.ConfigDir()

	disc, err := xrandr.Discover()
	if err != nil {
		return fmt.Errorf("discover outputs: %w", err)
	}

	var p *profile.Profile
	switch args[0] {
	case "extend":
		p = layout.AutoArrange(disc)
	case "mirror":
		p = layout.Mirror(disc)
	case "single":
		if len(args) < 2 {
			return fmt.Errorf("single requires an output name, e.g. `displayctl layout single eDP-1`")
		}
		p, err = layout.Single(disc, args[1])
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown layout %q (want extend, mirror, or single)", args[0])
	}

	return applyProfileV2(cfgDir, p)
}
