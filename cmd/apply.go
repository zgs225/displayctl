package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/yuez/displayctl/config"
	"github.com/yuez/displayctl/internal/dpi"
	"github.com/yuez/displayctl/internal/hook"
	"github.com/yuez/displayctl/internal/profile"
	"github.com/yuez/displayctl/internal/xrandr"
)

var modePattern = regexp.MustCompile(`^\d+x\d+$`)

func newApplyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "apply <profile|WxH|auto>",
		Short: "Apply a display profile, temporary mode, or the default profile",
		Args:  cobra.ExactArgs(1),
		RunE:  runApply,
	}
}

func runApply(cmd *cobra.Command, args []string) error {
	arg := args[0]
	cfgDir := config.ConfigDir()

	var p *profile.Profile
	var err error

	switch {
	case arg == "auto":
		p, err = profile.LoadDefault(cfgDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(4)
		}
	case modePattern.MatchString(arg):
		p = buildTempProfile(arg)
	default:
		p, err = profile.Load(cfgDir, arg)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}

	return applyProfile(cfgDir, p)
}

func buildTempProfile(mode string) *profile.Profile {
	tiers := true
	return &profile.Profile{
		Output: profile.OutputConfig{
			Name: "",
			Mode: mode,
			Rate: 0,
		},
		DPI: profile.DPIConfig{
			Tiers: &tiers,
		},
	}
}

func applyProfile(cfgDir string, p *profile.Profile) error {
	outputName := p.Output.Name
	if outputName == "" {
		name, err := xrandr.GetActiveOutput()
		if err != nil {
			return fmt.Errorf("detect active output: %w", err)
		}
		outputName = name
	}

	if p.Output.Mode != "current" {
		valid, err := xrandr.ValidateMode(outputName, p.Output.Mode)
		if err != nil {
			return fmt.Errorf("validate mode: %w", err)
		}
		if !valid {
			return fmt.Errorf("mode %s not supported on %s", p.Output.Mode, outputName)
		}
		if err := xrandr.SetMode(outputName, p.Output.Mode, p.Output.Rate); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(3)
		}
	}

	var dpiValue int
	if p.DPI.Value != nil {
		dpiValue = *p.DPI.Value
	} else if p.DPI.Tiers != nil && *p.DPI.Tiers {
		w, _, err := xrandr.GetScreenSize()
		if err != nil {
			return fmt.Errorf("get screen size: %w", err)
		}
		dpiValue = dpi.CalculateFromTiers(w)
	}

	if dpiValue > 0 {
		if err := dpi.SetXftDPI(dpiValue); err != nil {
			return fmt.Errorf("set xft dpi: %w", err)
		}
		if err := dpi.WriteRofiDPI(dpiValue); err != nil {
			fmt.Fprintf(os.Stderr, "warning: write rofi-dpi.rasi: %v\n", err)
		}
		fmt.Printf("output=%s mode=%s dpi=%d\n", outputName, p.Output.Mode, dpiValue)
	} else {
		fmt.Printf("output=%s mode=%s dpi=unchanged\n", outputName, p.Output.Mode)
	}

	hookEnv := map[string]string{
		"DISPLAYCTL_OUTPUT": outputName,
		"DISPLAYCTL_MODE":   p.Output.Mode,
		"DISPLAYCTL_DPI":    strconv.Itoa(dpiValue),
	}
	if err := hook.RunPostSwitch(cfgDir, hookEnv); err != nil {
		fmt.Fprintf(os.Stderr, "warning: post-switch hooks: %v\n", err)
	}

	return nil
}
