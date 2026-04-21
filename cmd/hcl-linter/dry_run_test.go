package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/bard-works/hcl-linter/internal/termcolor"
)

// captureOutput redirects both os.Stdout and diffOut into a single buffer.
// Returns the buffer and a flush function the test must call before reading
// the buffer (it closes the stdout pipe so the drain goroutine finishes).
// The flush is also registered via t.Cleanup as a safety net.
func captureOutput(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()

	buf := &bytes.Buffer{}
	mu := &sync.Mutex{}

	prevDiffOut := diffOut
	diffOut = &syncWriter{w: buf, mu: mu}

	prevStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	done := make(chan struct{})
	go func() {
		defer close(done)
		data, _ := io.ReadAll(r)
		mu.Lock()
		buf.Write(data)
		mu.Unlock()
	}()

	var once sync.Once
	flush := func() {
		once.Do(func() {
			_ = w.Close()
			<-done
			os.Stdout = prevStdout
			diffOut = prevDiffOut
		})
	}
	t.Cleanup(flush)
	return buf, flush
}

type syncWriter struct {
	w  *bytes.Buffer
	mu *sync.Mutex
}

func (sw *syncWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.w.Write(p)
}

// writeFile is a convenience wrapper for tests.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const blankLinesConfig = `rules {
  blank_lines {
    enabled       = true
    within_blocks = true
  }
}`

// misformattedContent has consecutive blank lines inside a block that
// blank_lines fix will collapse.
const misformattedContent = "locals {\n\n\n  foo = \"bar\"\n}\n"

// cleanContent is already in canonical form — fix produces no changes.
const cleanContent = "locals {\n  foo = \"bar\"\n}\n"

func TestFixDryRunPrintsDiffForMisformattedFile(t *testing.T) {
	resetFlags(t)
	if err := termcolor.SetMode(termcolor.ModeNever); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	t.Cleanup(func() { _ = termcolor.SetMode(termcolor.ModeAuto) })

	root := setupProject(t, misformattedContent, blankLinesConfig)
	flagConfigSrc = filepath.Join(root, ".hcl-linter")
	flagDryRun = true

	target := filepath.Join(root, "terragrunt.hcl")
	originalBytes, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}

	buf, flush := captureOutput(t)
	runErr := runFix(nil, []string{root})
	flush()
	out := buf.String()

	if runErr == nil {
		t.Fatal("expected dry-run error, got nil")
	}
	if !strings.Contains(runErr.Error(), "dry run:") {
		t.Errorf("expected error to mention 'dry run:', got: %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "1 file(s)") {
		t.Errorf("expected error to mention '1 file(s)', got: %v", runErr)
	}

	if !strings.Contains(out, "--- a/") {
		t.Errorf("expected diff '--- a/' header in output, got:\n%s", out)
	}
	if !strings.Contains(out, "+++ b/") {
		t.Errorf("expected diff '+++ b/' header in output, got:\n%s", out)
	}
	if !strings.Contains(out, "-") || !strings.Contains(out, "@@") {
		t.Errorf("expected diff hunk in output, got:\n%s", out)
	}

	afterBytes, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(originalBytes, afterBytes) {
		t.Error("file on disk was modified during dry-run")
	}
}

func TestFixDryRunCleanFile(t *testing.T) {
	resetFlags(t)
	if err := termcolor.SetMode(termcolor.ModeNever); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	t.Cleanup(func() { _ = termcolor.SetMode(termcolor.ModeAuto) })

	root := setupProject(t, cleanContent, blankLinesConfig)
	flagConfigSrc = filepath.Join(root, ".hcl-linter")
	flagDryRun = true

	target := filepath.Join(root, "terragrunt.hcl")
	originalBytes, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}

	buf, flush := captureOutput(t)
	runErr := runFix(nil, []string{root})
	flush()
	out := buf.String()

	if runErr != nil {
		t.Errorf("expected nil error on clean file, got: %v", runErr)
	}
	if !strings.Contains(out, "No changes needed") {
		t.Errorf("expected 'No changes needed' in output, got:\n%s", out)
	}

	afterBytes, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(originalBytes, afterBytes) {
		t.Error("file on disk was modified during dry-run")
	}
}

func TestFixDryRunMultipleFiles(t *testing.T) {
	resetFlags(t)
	if err := termcolor.SetMode(termcolor.ModeNever); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	t.Cleanup(func() { _ = termcolor.SetMode(termcolor.ModeAuto) })

	root := t.TempDir()
	configDir := filepath.Join(root, ".hcl-linter")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(configDir, "default.hcl"), blankLinesConfig)

	writeFile(t, filepath.Join(root, "a.hcl"), misformattedContent)
	writeFile(t, filepath.Join(root, "b.hcl"), misformattedContent)
	writeFile(t, filepath.Join(root, "c.hcl"), cleanContent)

	flagConfigSrc = configDir
	flagDryRun = true

	buf, flush := captureOutput(t)
	runErr := runFix(nil, []string{root})
	flush()
	out := buf.String()

	if runErr == nil {
		t.Fatal("expected dry-run error, got nil")
	}
	if !strings.Contains(runErr.Error(), "2 file(s)") {
		t.Errorf("expected error to mention '2 file(s)', got: %v", runErr)
	}

	if strings.Count(out, "--- a/") != 2 {
		t.Errorf("expected two '--- a/' headers in diff output, got %d:\n%s",
			strings.Count(out, "--- a/"), out)
	}
	if strings.Count(out, "+++ b/") != 2 {
		t.Errorf("expected two '+++ b/' headers in diff output, got %d:\n%s",
			strings.Count(out, "+++ b/"), out)
	}

	for _, name := range []string{"a.hcl", "b.hcl", "c.hcl"} {
		after, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		want := misformattedContent
		if name == "c.hcl" {
			want = cleanContent
		}
		if string(after) != want {
			t.Errorf("%s was modified during dry-run", name)
		}
	}
}

func TestFixDryRunWithFormat(t *testing.T) {
	resetFlags(t)
	if err := termcolor.SetMode(termcolor.ModeNever); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	t.Cleanup(func() { _ = termcolor.SetMode(termcolor.ModeAuto) })

	root := t.TempDir()
	target := filepath.Join(root, "terragrunt.hcl")
	writeFile(t, target, misformattedContent)

	flagConfigSrc = root
	flagFormat = true
	flagDryRun = true

	originalBytes, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}

	buf, flush := captureOutput(t)
	runErr := runFix(nil, []string{root})
	flush()
	out := buf.String()

	if runErr == nil {
		t.Fatal("expected dry-run error, got nil")
	}
	if !strings.Contains(runErr.Error(), "dry run:") {
		t.Errorf("expected error to mention 'dry run:', got: %v", runErr)
	}
	if !strings.Contains(out, "--- a/") {
		t.Errorf("expected diff '--- a/' header in output, got:\n%s", out)
	}

	afterBytes, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(originalBytes, afterBytes) {
		t.Error("file on disk was modified during dry-run --format")
	}
}
