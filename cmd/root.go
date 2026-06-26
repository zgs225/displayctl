package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "displayctl",
		Short: "Manage xrandr display modes and DPI settings",
	}
	root.AddCommand(newApplyCmd())
	root.AddCommand(newListCmd())
	return root
}

func Execute() {
	root := NewRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
