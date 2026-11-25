package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zgs225/displayctl/config"
	"github.com/zgs225/displayctl/internal/fingerprint"
	"github.com/zgs225/displayctl/internal/profile"
	"github.com/zgs225/displayctl/internal/xrandr"
)

func newListCmd() *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{
		Use:   "list",
		Short: "List available display profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, jsonOut)
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, "output JSON for scripting")
	return c
}

type profileJSON struct {
	Name    string   `json:"name"`
	Layout  string   `json:"layout"`
	Outputs []string `json:"outputs"`
	DPI     string   `json:"dpi"`
	Default bool     `json:"default"`
	Match   bool     `json:"match"`
}

func runList(cmd *cobra.Command, jsonOut bool) error {
	cfgDir := config.ConfigDir()
	summaries, err := profile.List(cfgDir)
	if err != nil {
		return err
	}

	// Current hardware fingerprints (name+EDID and name-independent EDID set),
	// used to mark the matching profile. Output names flip between dock
	// enumerations (DP-1 ↔ DP-1-1), so both must be considered.
	currentIDs := []string{}
	if disc, derr := xrandr.Discover(); derr == nil {
		refs := fingerprint.FromOutputs(disc)
		currentIDs = append(currentIDs, fingerprint.Compute(refs), fingerprint.ComputeEDIDs(refs))
	}
	matchesCurrent := func(s profile.ProfileSummary) bool {
		return profile.MatchAny(s.MatchIDs, currentIDs)
	}

	if jsonOut {
		out := make([]profileJSON, 0, len(summaries))
		for _, s := range summaries {
			out = append(out, profileJSON{
				Name:    s.Name,
				Layout:  s.Layout,
				Outputs: splitCSV(s.Outputs),
				DPI:     s.DPI,
				Default: s.Default,
				Match:   matchesCurrent(s),
			})
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tLAYOUT\tOUTPUTS\tDPI\tDEFAULT\tMATCH")
	for _, s := range summaries {
		defMark := ""
		if s.Default {
			defMark = "*"
		}
		matchMark := ""
		if matchesCurrent(s) {
			matchMark = "★"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", s.Name, s.Layout, s.Outputs, s.DPI, defMark, matchMark)
	}
	return w.Flush()
}

func splitCSV(s string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(s, ",")
}
