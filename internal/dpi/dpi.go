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

func SetXftDPI(dpi int) error {
	cmd := exec.Command("xrdb", "-merge")
	cmd.Stdin = strings.NewReader(fmt.Sprintf("Xft.dpi: %d", dpi))
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
