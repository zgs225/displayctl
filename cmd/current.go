package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zgs225/displayctl/internal/dpi"
	"github.com/zgs225/displayctl/internal/layout"
	"github.com/zgs225/displayctl/internal/xrandr"
)

func newCurrentCmd() *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{
		Use:   "current",
		Short: "Show current display mode and DPI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCurrent(cmd, jsonOut)
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, "output JSON for scripting")
	return c
}

type currentOutputJSON struct {
	Name    string `json:"name"`
	Mode    string `json:"mode"`
	Pos     string `json:"pos"`
	Primary bool   `json:"primary"`
}

type currentJSON struct {
	Layout  string              `json:"layout"`
	DPI     int                 `json:"dpi"`
	Outputs []currentOutputJSON `json:"outputs"`
}

func runCurrent(cmd *cobra.Command, jsonOut bool) error {
	disc, err := xrandr.Discover()
	if err != nil {
		return err
	}

	dpiVal, derr := dpi.GetCurrentDPI()
	if derr != nil {
		dpiVal = 0
	}

	if jsonOut {
		out := currentJSON{
			Layout:  layout.Capture(disc).Layout,
			DPI:     dpiVal,
			Outputs: []currentOutputJSON{},
		}
		for _, o := range disc {
			if o.Connected && o.Active() {
				out.Outputs = append(out.Outputs, currentOutputJSON{
					Name:    o.Name,
					Mode:    o.Mode,
					Pos:     fmt.Sprintf("%dx%d", o.PosX, o.PosY),
					Primary: o.Primary,
				})
			}
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
	}

	// Human-readable text output.
	cap := layout.Capture(disc)
	fmt.Fprintf(cmd.OutOrStdout(), "layout=%s dpi=%d\n", cap.Layout, dpiVal)
	for _, name := range cap.OutputNames() {
		cfg := cap.Outputs[name]
		prim := ""
		if cfg.Primary {
			prim = " primary"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "output=%s mode=%s pos=%s%s\n", name, cfg.Mode, cfg.Pos, prim)
	}
	return nil
}
