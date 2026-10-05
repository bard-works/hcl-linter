package engine_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclwrite"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/engine"
	"github.com/bard-works/hcl-linter/internal/rules"
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
	//nolint:gosec // G703: dst is inside t.TempDir(), src is a repo demo file.
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

// After fix, check must not report anything an auto-fixable rule is
// responsible for; otherwise users are told to fix what fix cannot.
func TestCheckIsCleanForFixableRulesAfterFix(t *testing.T) {
	fixable := fixableRules()
	// name_validation only fixes separators (my-vpc -> my_vpc); renaming
	// camelCase labels would break every reference to them, so it is
	// deliberately partial.
	delete(fixable, "name_validation")
	for src, cfgDir := range demoTargets(t) {
		t.Run(filepath.ToSlash(src), func(t *testing.T) {
			loader := config.NewLoader(cfgDir)
			target := copyToTemp(t, src)
			if _, err := os.Stat(cfgDir); err != nil || !loader.HasConfigForFile(target) {
				t.Skip("no config for this demo")
			}
			eng := engine.New(loader)
			if _, err := eng.FixFile(target); err != nil {
				t.Fatalf("fix: %v", err)
			}
			res, err := eng.LintFile(target)
			if err != nil {
				t.Fatalf("lint: %v", err)
			}
			for _, is := range res.Issues {
				if fixable[is.Source] {
					fixed, _ := os.ReadFile(target)
					t.Errorf("%s still reported after fix: %s\n%s", is.Source, is.Message, fixed)
				}
			}
		})
	}
}

// The reverse of the test above: when fix would change a file, check must
// say why. Otherwise `check` passes in CI while `fix --dry-run` fails.
func TestFixChangesOnlyWhatCheckReports(t *testing.T) {
	fixable := fixableRules()
	for src, cfgDir := range demoTargets(t) {
		t.Run(filepath.ToSlash(src), func(t *testing.T) {
			loader := config.NewLoader(cfgDir)
			if _, err := os.Stat(cfgDir); err != nil || !loader.HasConfigForFile(src) {
				t.Skip("no config for this demo")
			}
			assertFixImpliesFinding(t, engine.New(loader), copyToTemp(t, src), fixable)
		})
	}
}

// Shapes the array and blank-line fixers handle specially; check and fix must
// agree on each in both directions.
func TestCheckAndFixAgreeOnEdgeShapes(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".hcl-linter")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `rules {
  array_format {
    enabled = true
  }
  blank_lines {
    enabled       = true
    within_blocks = true
  }
}
`
	if err := os.WriteFile(filepath.Join(cfgDir, "default.hcl"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"trailing comment":       "a = [\"x\", \"y\"] # keep\n",
		"one-line object":        "inputs = { azs = [\"a\", \"b\"] }\n",
		"multiline no comma":     "b = [\n  \"x\",\n  \"y\"\n]\n",
		"multiline two per line": "b = [\n  \"x\", \"y\",\n  \"z\",\n]\n",
		"inline in object":       "inputs = {\n  azs = [\"a\", \"b\"]\n}\n",
		"suppressed inline":      "# hcl-linter:ignore array_format\na = [\"x\", \"y\"]\n",
		"suppressed block":       "# hcl-linter:ignore blank_lines\nlocals {\n\n  a = 1\n}\n",
		"clean":                  "a = [\n  \"x\",\n  \"y\",\n]\n",
	}
	loader := config.NewLoader(cfgDir)
	fixable := fixableRules()
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "terragrunt.hcl")
			if err := os.WriteFile(target, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			eng := engine.New(loader)
			assertFixImpliesFinding(t, eng, target, fixable)
			if _, err := eng.FixFile(target); err != nil {
				t.Fatalf("fix: %v", err)
			}
			res, err := eng.LintFile(target)
			if err != nil {
				t.Fatalf("lint: %v", err)
			}
			for _, is := range res.Issues {
				if fixable[is.Source] {
					fixed, _ := os.ReadFile(target)
					t.Errorf("%s still reported after fix: %s\n%s", is.Source, is.Message, fixed)
				}
			}
		})
	}
}

func fixableRules() map[string]bool {
	fixable := map[string]bool{}
	for _, r := range rules.DefaultRegistry().All() {
		if _, ok := r.(rules.Fixer); ok {
			fixable[r.Name()] = true
		}
	}
	return fixable
}

// assertFixImpliesFinding fails when a dry-run fix would change target but
// check reports nothing from a fixable rule.
func assertFixImpliesFinding(t *testing.T, eng *engine.Engine, target string, fixable map[string]bool) {
	t.Helper()
	eng.DryRun = true
	defer func() { eng.DryRun = false }()
	fix, err := eng.FixFile(target)
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if fix.Changes == 0 {
		return
	}
	res, err := eng.LintFile(target)
	if err != nil {
		t.Fatalf("lint: %v", err)
	}
	for _, is := range res.Issues {
		if fixable[is.Source] {
			return
		}
	}
	original, _ := os.ReadFile(target)
	t.Errorf("fix would change the file but check reports nothing fixable\n--- file ---\n%s\n--- fix ---\n%s",
		original, fix.Content)
}

// The demo corpus is documentation: every *_valid file must lint clean and
// every *_violation file must report something with its own config.
func TestDemoFilesMatchTheirNames(t *testing.T) {
	for src, cfgDir := range demoTargets(t) {
		name := filepath.Base(src)
		wantClean := strings.Contains(name, "_valid.")
		wantIssues := strings.Contains(name, "_violation.")
		if !wantClean && !wantIssues {
			continue
		}
		t.Run(filepath.ToSlash(src), func(t *testing.T) {
			loader := config.NewLoader(cfgDir)
			if _, err := os.Stat(cfgDir); err != nil || !loader.HasConfigForFile(src) {
				t.Fatalf("demo has no matching config in %s", cfgDir)
			}
			res, err := engine.New(loader).LintFile(src)
			if err != nil {
				t.Fatal(err)
			}
			if wantClean && len(res.Issues) > 0 {
				t.Errorf("valid demo reports issues: %+v", res.Issues)
			}
			if wantIssues && len(res.Issues) == 0 {
				t.Error("violation demo reports nothing")
			}
		})
	}
}

// fix must not make a terraform-fmt-clean file fail `terraform fmt -check`.
func TestFixKeepsFmtCleanFilesClean(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".hcl-linter")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(cfgDir, "default.hcl"),
		[]byte(`extends = "builtin:terragrunt"`+"\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "terragrunt.hcl")
	src := "inputs = {\n  a   = \"x\"\n  azs = [\"a\", \"b\"]\n  b   = \"y\"\n}\n"
	if !bytes.Equal(hclwrite.Format([]byte(src)), []byte(src)) {
		t.Fatal("fixture must start fmt-clean")
	}
	if err := os.WriteFile(target, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := engine.New(config.NewLoader(cfgDir)).FixFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if res.Changes == 0 {
		t.Fatal("expected array_format to change the file")
	}
	got := []byte(res.Content)
	if !bytes.Equal(hclwrite.Format(got), got) {
		t.Errorf("fix left the file failing terraform fmt:\n%s", got)
	}
}
