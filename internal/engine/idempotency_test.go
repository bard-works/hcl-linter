package engine_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/engine"
)

// demoTargets returns every demo file under docs/demo together with the
// .hcl-linter/ directory that configures it.
func demoTargets(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join("..", "..", "docs", "demo")
	targets := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".hcl-linter" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".hcl") || strings.HasSuffix(path, ".tf") {
			targets[path] = filepath.Join(filepath.Dir(path), ".hcl-linter")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) < 20 {
		t.Fatalf("expected the demo corpus, found only %d files", len(targets))
	}
	return targets
}

// copyToTemp copies a demo file so fixes never touch the repository.
func copyToTemp(t *testing.T, src string) string {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), filepath.Base(src))
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return dst
}

// A fix must reach its final form in one run: `fix` followed by
// `fix --dry-run` in CI may never report drift.
func TestFixIsIdempotentOnDemoCorpus(t *testing.T) {
	for src, cfgDir := range demoTargets(t) {
		t.Run(filepath.ToSlash(src), func(t *testing.T) {
			if _, err := os.Stat(cfgDir); err != nil {
				t.Skip("no config for this demo")
			}
			loader := config.NewLoader(cfgDir)
			eng := engine.New(loader)
			target := copyToTemp(t, src)
			if !loader.HasConfigForFile(target) {
				t.Skip("no matching config file")
			}
			assertSecondRunIsNoop(t, target, eng.FixFile)
		})
	}
}

func TestFormatIsIdempotentOnDemoCorpus(t *testing.T) {
	for src := range demoTargets(t) {
		t.Run(filepath.ToSlash(src), func(t *testing.T) {
			eng := engine.New(&config.Loader{})
			assertSecondRunIsNoop(t, copyToTemp(t, src), eng.FormatFixFile)
		})
	}
}

func assertSecondRunIsNoop(t *testing.T, target string, fix func(string) (*engine.FixResult, error)) {
	t.Helper()
	if _, err := fix(target); err != nil {
		t.Fatalf("first run: %v", err)
	}
	first, _ := os.ReadFile(target)
	res, err := fix(target)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	second, _ := os.ReadFile(target)
	if res.Changes != 0 || string(first) != string(second) {
		t.Errorf("second run changed the file (%d changes)\n--- after first run ---\n%s\n--- after second run ---\n%s",
			res.Changes, first, second)
	}
}
