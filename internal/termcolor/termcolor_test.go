package termcolor

import (
	"strings"
	"testing"
)

// resetAfter restores a known state so tests don't leak colour state between
// cases. Tests run sequentially so a shared reset is safe.
func resetAfter(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { _ = SetMode(ModeNever) })
}

func TestSetModeInvalid(t *testing.T) {
	resetAfter(t)
	if err := SetMode("purple"); err == nil {
		t.Fatal("expected error for invalid mode, got nil")
	}
}

func TestSetModeNever(t *testing.T) {
	resetAfter(t)
	if err := SetMode(ModeNever); err != nil {
		t.Fatalf("SetMode(never): %v", err)
	}
	if Enabled() {
		t.Fatal("expected Enabled()==false after SetMode(never)")
	}
}

func TestSetModeAlways(t *testing.T) {
	resetAfter(t)
	if err := SetMode(ModeAlways); err != nil {
		t.Fatalf("SetMode(always): %v", err)
	}
	if !Enabled() {
		t.Fatal("expected Enabled()==true after SetMode(always)")
	}
}

func TestSetModeAutoWithNoColor(t *testing.T) {
	resetAfter(t)
	t.Setenv("NO_COLOR", "1")
	if err := SetMode(ModeAuto); err != nil {
		t.Fatalf("SetMode(auto): %v", err)
	}
	if Enabled() {
		t.Fatal("expected Enabled()==false when NO_COLOR is set")
	}
}

// TestSetModeAlwaysOverridesNoColor is a regression: fatih/color bakes a
// per-instance noColor flag at New() time when NO_COLOR is set in env.
// SetMode must resync each cached *Color or "always" silently degrades to
// no colour when NO_COLOR is present. Also guards against the prior-test
// "never" state sticking: SetMode(always) must re-enable all cached colours.
func TestSetModeAlwaysOverridesNoColor(t *testing.T) {
	resetAfter(t)
	t.Setenv("NO_COLOR", "1")
	if err := SetMode(ModeNever); err != nil {
		t.Fatal(err)
	}
	if err := SetMode(ModeAlways); err != nil {
		t.Fatal(err)
	}
	if !Enabled() {
		t.Fatal("SetMode(always) must enable regardless of NO_COLOR")
	}
	got := Error("x")
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("expected ANSI escape when --color=always overrides NO_COLOR, got %q", got)
	}
}

func TestSetModeAutoWithDumbTerm(t *testing.T) {
	resetAfter(t)
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "dumb")
	if err := SetMode(ModeAuto); err != nil {
		t.Fatalf("SetMode(auto): %v", err)
	}
	if Enabled() {
		t.Fatal("expected Enabled()==false when TERM=dumb")
	}
}

func TestHelpersDisabledReturnVerbatim(t *testing.T) {
	resetAfter(t)
	if err := SetMode(ModeNever); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(string) string{
		"Error":    Error,
		"Warning":  Warning,
		"Success":  Success,
		"Path":     Path,
		"Rule":     Rule,
		"Location": Location,
	}
	const input = "hello"
	for name, fn := range cases {
		got := fn(input)
		if got != input {
			t.Errorf("%s disabled: got %q, want %q", name, got, input)
		}
	}
}

func TestHelpersEnabledWrapInAnsi(t *testing.T) {
	resetAfter(t)
	if err := SetMode(ModeAlways); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		fn     func(string) string
		opener string
	}{
		{"Error", Error, "\x1b[31;1m"},
		{"Warning", Warning, "\x1b[33m"},
		{"Success", Success, "\x1b[32m"},
		{"Path", Path, "\x1b[36;1m"},
		{"Rule", Rule, "\x1b[36m"},
		{"Location", Location, "\x1b[2m"},
	}
	const input = "hello"
	for _, c := range cases {
		got := c.fn(input)
		if !strings.HasPrefix(got, c.opener) {
			t.Errorf("%s: expected prefix %q, got %q", c.name, c.opener, got)
		}
		if !strings.Contains(got, input) {
			t.Errorf("%s: expected to contain %q, got %q", c.name, input, got)
		}
		if !strings.HasSuffix(got, "m") {
			t.Errorf("%s: expected ANSI terminator, got %q", c.name, got)
		}
	}
}

func TestHelpersDistinctColours(t *testing.T) {
	resetAfter(t)
	if err := SetMode(ModeAlways); err != nil {
		t.Fatal(err)
	}
	outputs := []string{
		Error("x"),
		Warning("x"),
		Success("x"),
		Path("x"),
		Rule("x"),
		Location("x"),
	}
	seen := make(map[string]bool, len(outputs))
	for _, o := range outputs {
		seen[o] = true
	}
	// Rule and Path differ only by Bold; all six strings should be distinct.
	if len(seen) != len(outputs) {
		t.Errorf("expected all six helpers to produce distinct ANSI output, got %d unique of %d: %q",
			len(seen), len(outputs), outputs)
	}
}

func TestEmptyStringDoesNotPanic(t *testing.T) {
	resetAfter(t)
	if err := SetMode(ModeAlways); err != nil {
		t.Fatal(err)
	}
	_ = Error("")
	_ = Warning("")
	_ = Success("")
	_ = Path("")
	_ = Rule("")
	_ = Location("")
}
