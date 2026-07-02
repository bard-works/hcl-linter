// Package termcolor provides semantic colour helpers for CLI output.
// Helpers return the input string verbatim when colour is disabled.
package termcolor

import (
	"fmt"
	"os"
	"sync"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
)

const (
	ModeAuto   = "auto"
	ModeAlways = "always"
	ModeNever  = "never"
)

var (
	mu      sync.RWMutex
	enabled bool

	errCol  = color.New(color.FgRed, color.Bold)
	warnCol = color.New(color.FgYellow)
	okCol   = color.New(color.FgGreen)
	pathCol = color.New(color.FgCyan, color.Bold)
	ruleCol = color.New(color.FgCyan)
	locCol  = color.New(color.Faint)

	// cached *Color instances reused across calls. fatih/color bakes a
	// per-instance noColor flag at New() time if NO_COLOR was set, so we
	// must re-sync each instance whenever SetMode changes the effective
	// state - otherwise the global color.NoColor toggle is ignored.
	cachedColours = []*color.Color{errCol, warnCol, okCol, pathCol, ruleCol, locCol}
)

// SetMode configures colour output. Valid modes: "auto", "always", "never".
// In auto mode colour is enabled only when both stdout and stderr are TTYs
// (helpers colour text destined for either stream), TERM is not "dumb", and
// NO_COLOR is unset (per https://no-color.org).
func SetMode(mode string) error {
	mu.Lock()
	defer mu.Unlock()

	switch mode {
	case ModeAuto:
		enabled = autoDetect()
	case ModeAlways:
		enabled = true
	case ModeNever:
		enabled = false
	default:
		return fmt.Errorf("invalid color mode %q: must be auto, always, or never", mode)
	}
	color.NoColor = !enabled
	for _, c := range cachedColours {
		if enabled {
			c.EnableColor()
		} else {
			c.DisableColor()
		}
	}
	return nil
}

// Enabled reports whether colour output is currently on.
func Enabled() bool {
	mu.RLock()
	defer mu.RUnlock()
	return enabled
}

func autoDetect() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return isatty.IsTerminal(os.Stdout.Fd()) && isatty.IsTerminal(os.Stderr.Fd())
}

func wrap(c *color.Color, s string) string {
	if !Enabled() {
		return s
	}
	return c.Sprint(s)
}

// Error wraps s in red+bold. Used for error severity labels and error prefixes.
func Error(s string) string { return wrap(errCol, s) }

// Warning wraps s in yellow. Used for warning severity labels and warning prefixes.
func Warning(s string) string { return wrap(warnCol, s) }

// Success wraps s in green. Used for "pass" / "OK" / "no changes" messages.
func Success(s string) string { return wrap(okCol, s) }

// Path wraps s in cyan+bold. Used for file path headers.
func Path(s string) string { return wrap(pathCol, s) }

// Rule wraps s in cyan. Used for rule names inside issue lines.
func Rule(s string) string { return wrap(ruleCol, s) }

// Location wraps s in faint. Used for "at file:line" location suffixes.
func Location(s string) string { return wrap(locCol, s) }
