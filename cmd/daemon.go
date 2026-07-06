package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/zgs225/displayctl/config"
	"github.com/zgs225/displayctl/internal/dpi"
	"github.com/zgs225/displayctl/internal/hook"
	"github.com/zgs225/displayctl/internal/randr"
	"github.com/zgs225/displayctl/internal/xrandr"
)

func newDaemonCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Run daemon that watches RandR events and auto-refreshes DPI",
		RunE:  runDaemon,
	}
}

func runDaemon(cmd *cobra.Command, args []string) error {
	display := os.Getenv("DISPLAY")
	if display == "" {
		return fmt.Errorf("DISPLAY not set")
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	debounce := make(chan struct{}, 1)

	go func() {
		for range debounce {
			time.Sleep(200 * time.Millisecond)
			handleScreenChange()
		}
	}()

	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "displayctl daemon: shutting down")
		os.Exit(0)
	}()

	fmt.Fprintln(os.Stderr, "displayctl daemon: watching RandR events on", display)
	return randr.Watch(display, func(width, height int) {
		select {
		case debounce <- struct{}{}:
		default:
		}
	})
}

func handleScreenChange() {
	output, err := xrandr.GetActiveOutput()
	if err != nil {
		w, _, screenErr := xrandr.GetScreenSizeFromScreenLine()
		if screenErr != nil {
			fmt.Fprintf(os.Stderr, "daemon: get active output: %v; Screen line fallback: %v\n", err, screenErr)
			return
		}
		dpiValue := dpi.CalculateFromTiers(w)
		if err := dpi.SetXrandrDPI(dpiValue); err != nil {
			fmt.Fprintf(os.Stderr, "daemon: set xrandr dpi: %v\n", err)
		}
		if err := dpi.SetXftDPI(dpiValue); err != nil {
			fmt.Fprintf(os.Stderr, "daemon: set xft dpi: %v\n", err)
			return
		}
		xcursorSize := dpi.CalculateXcursorSize(dpiValue)
		if err := dpi.SetXcursorSize(xcursorSize); err != nil {
			fmt.Fprintf(os.Stderr, "daemon: set xcursor size: %v\n", err)
		}
		fmt.Fprintf(os.Stderr, "daemon: mode= Screen-fallback dpi=%d\n", dpiValue)
		return
	}

	mode, err := xrandr.GetCurrentMode(output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon: get current mode: %v\n", err)
		return
	}

	w, _, err := xrandr.GetScreenSize()
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon: get screen size: %v\n", err)
		return
	}

	dpiValue := dpi.CalculateFromTiers(w)
	if err := dpi.SetXrandrDPI(dpiValue); err != nil {
		fmt.Fprintf(os.Stderr, "daemon: set xrandr dpi: %v\n", err)
	}
	if err := dpi.SetXftDPI(dpiValue); err != nil {
		fmt.Fprintf(os.Stderr, "daemon: set xft dpi: %v\n", err)
		return
	}
	xcursorSize := dpi.CalculateXcursorSize(dpiValue)
	if err := dpi.SetXcursorSize(xcursorSize); err != nil {
		fmt.Fprintf(os.Stderr, "daemon: set xcursor size: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "daemon: output=%s mode=%s dpi=%d\n", output, mode, dpiValue)

	cfgDir := config.ConfigDir()
	hookEnv := map[string]string{
		"DISPLAYCTL_OUTPUT":       output,
		"DISPLAYCTL_MODE":         mode,
		"DISPLAYCTL_DPI":          strconv.Itoa(dpiValue),
		"DISPLAYCTL_XCURSOR_SIZE": strconv.Itoa(dpi.CalculateXcursorSize(dpiValue)),
	}
	if err := hook.RunPostSwitch(cfgDir, hookEnv); err != nil {
		fmt.Fprintf(os.Stderr, "daemon: post-switch hooks: %v\n", err)
	}
}
