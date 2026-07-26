package xrandr

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Mode is a single resolution mode advertised by an output.
type Mode struct {
	Name      string    // e.g. "2560x1600"
	Preferred bool      // marked with '+' (display's native/preferred mode)
	Current   bool      // marked with '*' (currently active mode)
	Rates     []float64 // available refresh rates
}

// Output is the parsed state of one xrandr output.
type Output struct {
	Name      string
	Connected bool
	Primary   bool
	Mode      string // active mode "WxH", or "" if the output is inactive
	PosX      int
	PosY      int
	WidthMM   int
	HeightMM  int
	Modes     []Mode
	EDID      string // concatenated hex string, "" when unavailable
}

// Active reports whether the output currently drives a mode.
func (o Output) Active() bool { return o.Mode != "" }

// ModeWidth returns the width in pixels of the active mode, or 0 when inactive
// or unparseable.
func (o Output) ModeWidth() int {
	parts := strings.SplitN(o.Mode, "x", 2)
	if len(parts) != 2 {
		return 0
	}
	w, _ := strconv.Atoi(parts[0])
	return w
}

// PreferredMode returns the mode marked '+' (preferred/native), falling back to
// the first listed mode. Returns "" when the output has no modes.
func (o Output) PreferredMode() string {
	for _, m := range o.Modes {
		if m.Preferred {
			return m.Name
		}
	}
	if len(o.Modes) > 0 {
		return o.Modes[0].Name
	}
	return ""
}

var (
	headerRe   = regexp.MustCompile(`^(\S+)\s+(connected|disconnected)\b(.*)$`)
	modePosRe  = regexp.MustCompile(`(\d+x\d+)\+(\d+)\+(\d+)`)
	mmRe       = regexp.MustCompile(`(\d+)mm x (\d+)mm`)
	modeNameRe = regexp.MustCompile(`^\d+x\d+$`)
	hexRe      = regexp.MustCompile(`^[0-9a-f]+$`)
	screenRe   = regexp.MustCompile(`Screen \d+:.*current (\d+) x (\d+)`)
)

// Parse parses the output of `xrandr --current` or `xrandr --verbose` into a
// structured slice of outputs. EDID is only populated for verbose input.
func Parse(text string) ([]Output, error) {
	var outs []Output
	var cur *Output

	// EDID accumulation state
	collecting := false
	var edidBuf strings.Builder

	flushEDID := func() {
		if collecting && cur != nil {
			cur.EDID = edidBuf.String()
		}
		collecting = false
		edidBuf.Reset()
	}

	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)

		// EDID collection (verbose only)
		if collecting {
			if hexRe.MatchString(trimmed) {
				edidBuf.WriteString(trimmed)
				continue
			}
			flushEDID()
		}

		if m := headerRe.FindStringSubmatch(line); m != nil {
			flushEDID()
			rest := m[3]
			o := Output{
				Name:      m[1],
				Connected: m[2] == "connected",
				Primary:   strings.Contains(rest, " primary "),
			}
			if pm := modePosRe.FindStringSubmatch(rest); pm != nil {
				o.Mode = pm[1]
				o.PosX, _ = strconv.Atoi(pm[2])
				o.PosY, _ = strconv.Atoi(pm[3])
			}
			if mm := mmRe.FindStringSubmatch(rest); mm != nil {
				o.WidthMM, _ = strconv.Atoi(mm[1])
				o.HeightMM, _ = strconv.Atoi(mm[2])
			}
			outs = append(outs, o)
			cur = &outs[len(outs)-1]
			continue
		}

		if trimmed == "EDID:" {
			collecting = true
			edidBuf.Reset()
			continue
		}

		// Mode line: leading whitespace, first field is WxH
		if cur != nil && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) {
			fields := strings.Fields(line)
			if len(fields) >= 2 && modeNameRe.MatchString(fields[0]) {
				mode := Mode{Name: fields[0]}
				for _, rateTok := range fields[1:] {
					if strings.Contains(rateTok, "*") {
						mode.Current = true
					}
					if strings.Contains(rateTok, "+") {
						mode.Preferred = true
					}
					clean := strings.Trim(rateTok, "*+")
					if r, err := strconv.ParseFloat(clean, 64); err == nil {
						mode.Rates = append(mode.Rates, r)
					}
				}
				cur.Modes = append(cur.Modes, mode)
			}
		}
	}
	flushEDID()

	return outs, nil
}

// ScreenSize extracts the total framebuffer size from the "Screen N:" line.
func ScreenSize(text string) (int, int, error) {
	m := screenRe.FindStringSubmatch(text)
	if m == nil {
		return 0, 0, fmt.Errorf("Screen line not found in xrandr output")
	}
	w, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid width: %s", m[1])
	}
	h, err := strconv.Atoi(m[2])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid height: %s", m[2])
	}
	return w, h, nil
}
