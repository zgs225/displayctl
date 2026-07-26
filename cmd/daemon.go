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
	"github.com/zgs225/displayctl/internal/fingerprint"
	"github.com/zgs225/displayctl/internal/hook"
	"github.com/zgs225/displayctl/internal/randr"
	"github.com/zgs225/displayctl/internal/xrandr"
)

func newDaemonCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Watch RandR events: re-apply auto on hotplug, refresh DPI on resize",
		RunE:  runDaemon,
	}
}

// lastFingerprint gates hotplug re-application: applying a layout does not
// change the connected-output set, so the fingerprint stays stable and we avoid
// re-applying in a loop. Only a genuine plug/unplug changes it.
var lastFingerprint string

func runDaemon(cmd *cobra.Command, args []string) error {
	display := os.Getenv("DISPLAY")
	if display == "" {
		return fmt.Errorf("DISPLAY not set")
	}

	// Seed the fingerprint so the first event after startup (i3 already ran
	// `apply auto`) does not spuriously re-apply.
	if disc, err := xrandr.Discover(); err == nil {
		lastFingerprint = fingerprint.Compute(fingerprint.FromOutputs(disc))
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	debounce := make(chan struct{}, 1)

	go func() {
		for range debounce {
			time.Sleep(300 * time.Millisecond)
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
	cfgDir := config.ConfigDir()

	disc, err := xrandr.Discover()
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon: discover: %v\n", err)
		return
	}

	id := fingerprint.Compute(fingerprint.FromOutputs(disc))
	if id != lastFingerprint {
		// Hardware combination changed (plug/unplug) → re-run auto selection.
		lastFingerprint = id
		decision, err := resolveAuto(cfgDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "daemon: auto: %v\n", err)
			return
		}
		announceAuto(decision)
		if err := applyProfile(cfgDir, decision.Profile); err != nil {
			fmt.Fprintf(os.Stderr, "daemon: apply: %v\n", err)
		}
		return
	}

	// Fingerprint unchanged → a pure resize (e.g. XRDP client resize) → just
	// refresh the global DPI off the primary active output.
	refreshDPI(cfgDir, disc)
}

func refreshDPI(cfgDir string, disc []xrandr.Output) {
	width := 0
	var refName string
	for _, o := range disc {
		if o.Connected && o.Active() && o.Primary {
			width, refName = o.ModeWidth(), o.Name
			break
		}
	}
	if width == 0 {
		for _, o := range disc {
			if o.Connected && o.Active() {
				width, refName = o.ModeWidth(), o.Name
				break
			}
		}
	}
	if width == 0 {
		fmt.Fprintln(os.Stderr, "daemon: no active output to refresh DPI from")
		return
	}

	dpiValue := dpi.CalculateFromTiers(width)
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
	fmt.Fprintf(os.Stderr, "daemon: resize output=%s dpi=%d\n", refName, dpiValue)

	hookEnv := map[string]string{
		"DISPLAYCTL_OUTPUT":       refName,
		"DISPLAYCTL_MODE":         "",
		"DISPLAYCTL_DPI":          strconv.Itoa(dpiValue),
		"DISPLAYCTL_XCURSOR_SIZE": strconv.Itoa(xcursorSize),
	}
	if err := hook.RunPostSwitch(cfgDir, hookEnv); err != nil {
		fmt.Fprintf(os.Stderr, "daemon: post-switch hooks: %v\n", err)
	}
}
