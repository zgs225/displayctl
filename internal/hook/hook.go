package hook

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func RunPostSwitch(dir string, env map[string]string) error {
	hookDir := filepath.Join(dir, "post-switch.d")
	entries, err := os.ReadDir(hookDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read post-switch.d: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.Mode()&0111 == 0 {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		path := filepath.Join(hookDir, name)
		cmd := exec.Command(path)
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "hook %s failed: %s: %s\n", name, err, strings.TrimSpace(string(out)))
		} else {
			fmt.Fprintf(os.Stderr, "hook: %s completed\n", name)
		}
	}
	return nil
}
