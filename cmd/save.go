package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zgs225/displayctl/config"
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

	if err := profile.Save(cfgDir, name, p); err != nil {
		return err
	}

	fmt.Printf("saved profile %q: layout=%s outputs=%s fingerprint=%s\n",
		name, p.Layout, strings.Join(p.OutputNames(), ","), p.MatchID())
	return nil
}
