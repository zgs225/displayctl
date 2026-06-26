package xrandr

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

func runXrandr(args ...string) (string, error) {
	cmd := exec.Command("xrandr", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("xrandr %s: %w: %s", strings.Join(args, " "), err, string(out))
	}
	return string(out), nil
}

func GetActiveOutput() (string, error) {
	out, err := runXrandr("--current")
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`^(\S+)\s+connected\s+primary\s+\d+x\d+`)
	for line := range strings.SplitSeq(out, "\n") {
		if m := re.FindStringSubmatch(line); m != nil {
			return m[1], nil
		}
	}
	re2 := regexp.MustCompile(`^(\S+)\s+connected\s+\d+x\d+`)
	for line := range strings.SplitSeq(out, "\n") {
		if m := re2.FindStringSubmatch(line); m != nil {
			return m[1], nil
		}
	}
	return "", fmt.Errorf("no connected output found")
}

func GetCurrentMode(output string) (string, error) {
	out, err := runXrandr("--current")
	if err != nil {
		return "", err
	}
	pattern := fmt.Sprintf(`^%s\s+connected\s+(?:primary\s+)?(\d+x\d+)`, regexp.QuoteMeta(output))
	re := regexp.MustCompile(pattern)
	for line := range strings.SplitSeq(out, "\n") {
		if m := re.FindStringSubmatch(line); m != nil {
			return m[1], nil
		}
	}
	return "", fmt.Errorf("output %s not found or not active", output)
}

func SetMode(output, mode string, rate int) error {
	args := []string{"--output", output, "--mode", mode}
	if rate > 0 {
		args = append(args, "--rate", strconv.Itoa(rate))
	}
	_, err := runXrandr(args...)
	return err
}

func ValidateMode(output, mode string) (bool, error) {
	out, err := runXrandr("--current")
	if err != nil {
		return false, err
	}
	inOutputBlock := false
	for line := range strings.SplitSeq(out, "\n") {
		if strings.HasPrefix(line, output+" ") {
			inOutputBlock = true
			continue
		}
		if inOutputBlock {
			if line == "" || (!strings.HasPrefix(line, "   ") && !strings.HasPrefix(line, "\t")) {
				break
			}
			fields := strings.Fields(line)
			if len(fields) > 0 && fields[0] == mode {
				return true, nil
			}
		}
	}
	return false, nil
}

func GetScreenSize() (int, int, error) {
	output, err := GetActiveOutput()
	if err != nil {
		return 0, 0, err
	}
	mode, err := GetCurrentMode(output)
	if err != nil {
		return 0, 0, err
	}
	parts := strings.SplitN(mode, "x", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid mode format: %s", mode)
	}
	w, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid width: %s", parts[0])
	}
	h, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid height: %s", parts[1])
	}
	return w, h, nil
}
