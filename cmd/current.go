package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yuez/displayctl/internal/dpi"
	"github.com/yuez/displayctl/internal/xrandr"
)

func newCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Show current display mode and DPI",
		RunE:  runCurrent,
	}
}

func runCurrent(cmd *cobra.Command, args []string) error {
	output, err := xrandr.GetActiveOutput()
	if err != nil {
		return err
	}
	mode, err := xrandr.GetCurrentMode(output)
	if err != nil {
		return err
	}
	dpiVal, err := dpi.GetCurrentDPI()
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "output=%s mode=%s dpi=unknown\n", output, mode)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "output=%s mode=%s dpi=%d\n", output, mode, dpiVal)
	return nil
}
