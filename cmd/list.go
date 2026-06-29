package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zgs225/displayctl/config"
	"github.com/zgs225/displayctl/internal/profile"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available display profiles",
		RunE:  runList,
	}
}

func runList(cmd *cobra.Command, args []string) error {
	summaries, err := profile.List(config.ConfigDir())
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tMODE\tDPI\tDEFAULT")
	for _, s := range summaries {
		defMark := ""
		if s.Default {
			defMark = "*"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.Name, s.Mode, s.DPI, defMark)
	}
	return w.Flush()
}
