package cmd

import (
	"fmt"

	"github.com/zgs225/displayctl/internal/profile"
)

// applyProfileV2 applies a v2 multi-output profile. Implemented in the layout
// planning increment; this stub keeps the build green during incremental TDD.
func applyProfileV2(cfgDir string, p *profile.Profile) error {
	return fmt.Errorf("v2 multi-output apply not yet implemented")
}
