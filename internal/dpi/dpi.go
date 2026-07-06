package dpi

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

func CalculateFromTiers(width int) int {
	switch {
	case width >= 3000:
		return 192
	case width >= 2700:
		return 168
	case width >= 2000:
		return 144
	default:
		return 96
	}
}

func SetXrandrDPI(dpi int) error {
	cmd := exec.Command("xrandr", "--dpi", strconv.Itoa(dpi))
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("xrandr --dpi %d: %w", dpi, err)
	}
	return nil
}

func SetXftDPI(dpi int) error {
	cmd := exec.Command("xrdb", "-merge")
	cmd.Stdin = strings.NewReader(fmt.Sprintf("Xft.dpi: %d", dpi))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("xrdb -merge: %w", err)
	}
	return nil
}

func CalculateXcursorSize(dpi int) int {
	return 24 * dpi / 96
}

func SetXcursorSize(size int) error {
	cmd := exec.Command("xrdb", "-merge")
	cmd.Stdin = strings.NewReader(fmt.Sprintf("Xcursor.size: %d", size))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("xrdb -merge: %w", err)
	}
	return nil
}

func GetCurrentDPI() (int, error) {
	out, err := exec.Command("xrdb", "-query").Output()
	if err != nil {
		return 0, fmt.Errorf("xrdb -query: %w", err)
	}
	re := regexp.MustCompile(`Xft\.dpi:\s+(\d+)`)
	m := re.FindSubmatch(out)
	if m == nil {
		return 0, fmt.Errorf("Xft.dpi not found in xrdb")
	}
	return strconv.Atoi(string(m[1]))
}
