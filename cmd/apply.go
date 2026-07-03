package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/zgs225/displayctl/config"
	"github.com/zgs225/displayctl/internal/dpi"
	"github.com/zgs225/displayctl/internal/hook"
	"github.com/zgs225/displayctl/internal/profile"
	"github.com/zgs225/displayctl/internal/xrandr"
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
	screenFallback := false

	if outputName == "" {
		name, err := xrandr.GetActiveOutput()
		if err != nil {
			if p.Output.Mode == "current" {
				screenFallback = true
			} else {
				return fmt.Errorf("detect active output: %w", err)
			}
		} else {
			outputName = name
		}
	}

	resolvedMode := p.Output.Mode

	if screenFallback {
		w, h, err := xrandr.GetScreenSizeFromScreenLine()
		if err != nil {
			return fmt.Errorf("get screen size from Screen line: %w", err)
		}
		resolvedMode = fmt.Sprintf("%dx%d", w, h)
	} else {
		if p.Output.Mode == "auto" {
			maxMode, err := xrandr.GetMaxMode(outputName)
			if err != nil {
				return fmt.Errorf("auto mode: %w", err)
			}
			resolvedMode = maxMode
			p.Output.Mode = maxMode
		}

		if p.Output.Mode == "current" {
			currentMode, err := xrandr.GetCurrentMode(outputName)
			if err != nil {
				return fmt.Errorf("get current mode: %w", err)
			}
			resolvedMode = currentMode
		}

		if resolvedMode != "current" {
			valid, err := xrandr.ValidateMode(outputName, resolvedMode)
			if err != nil {
				return fmt.Errorf("validate mode: %w", err)
			}
			if !valid {
				return fmt.Errorf("mode %s not supported on %s", resolvedMode, outputName)
			}
			if err := xrandr.SetMode(outputName, resolvedMode, p.Output.Rate); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(3)
			}
		}
	}

	var dpiValue int
	if p.DPI.Value != nil {
		dpiValue = *p.DPI.Value
	} else if p.DPI.Tiers != nil && *p.DPI.Tiers {
		var w int
		var err error
		if screenFallback {
			w, _, err = xrandr.GetScreenSizeFromScreenLine()
		} else {
			w, _, err = xrandr.GetScreenSize()
		}
		if err != nil {
			return fmt.Errorf("get screen size: %w", err)
		}
		dpiValue = dpi.CalculateFromTiers(w)
	}

	if dpiValue > 0 {
		if err := dpi.SetXftDPI(dpiValue); err != nil {
			return fmt.Errorf("set xft dpi: %w", err)
		}
		fmt.Printf("output=%s mode=%s dpi=%d\n", outputName, resolvedMode, dpiValue)
	} else {
		fmt.Printf("output=%s mode=%s dpi=unchanged\n", outputName, resolvedMode)
	}

	hookEnv := map[string]string{
		"DISPLAYCTL_OUTPUT": outputName,
		"DISPLAYCTL_MODE":   resolvedMode,
		"DISPLAYCTL_DPI":    strconv.Itoa(dpiValue),
	}
	if err := hook.RunPostSwitch(cfgDir, hookEnv); err != nil {
		fmt.Fprintf(os.Stderr, "warning: post-switch hooks: %v\n", err)
	}

	return nil
}
