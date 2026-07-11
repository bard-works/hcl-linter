package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/bard-works/hcl-linter/internal/fsutil"
	"github.com/bard-works/hcl-linter/internal/termcolor"
)

const defaultInitTemplate = `rules {
  block_order {
    enabled = true
    order   = ["include", "locals", "terraform", "dependency", "inputs"]
  }

  array_format {
    enabled             = true
    multiline_threshold = 2
  }

  blank_lines {
    enabled       = true
    within_blocks = true
  }
}
`

const extendsInitTemplate = `extends = "default"

rules {
}
`

func (a *app) runInit(_ *cobra.Command, args []string) error {
	root := "."
	if len(args) == 1 {
		root = args[0]
	}

	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("path error: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("init target must be a directory: %s", root)
	}

	files := a.findHCLFiles(root)
	names := uniqueBasenames(files)

	configDir := filepath.Join(root, ".hcl-linter")
	proposed := proposedInitLayout(configDir, names)

	a.printInitSummary(root, files, names, proposed)

	if a.dryRun {
		a.printInitContents(proposed)
		return nil
	}

	if !a.initForce {
		entries, _ := os.ReadDir(configDir)
		if len(entries) > 0 {
			fmt.Fprintf(a.out, "\n%s %s already contains:\n",
				termcolor.Warning("Refusing to overwrite:"), configDir)
			for _, e := range entries {
				fmt.Fprintf(a.out, "  %s\n", e.Name())
			}
			return errors.New(".hcl-linter/ is non-empty; use --force to overwrite")
		}
	}

	if err := writeInitFiles(configDir, proposed); err != nil {
		return err
	}

	fmt.Fprintf(a.out, "\n%s Wrote %d config file(s) to %s\n",
		termcolor.Success("OK:"), len(proposed), configDir)
	fmt.Fprintln(a.out, "Next steps:")
	fmt.Fprintln(a.out, "  hcl-linter validate-config")
	fmt.Fprintln(a.out, "  hcl-linter fix ./ --dry-run")
	return nil
}

func uniqueBasenames(files []string) []string {
	seen := map[string]bool{}
	var names []string
	for _, f := range files {
		b := filepath.Base(f)
		if seen[b] {
			continue
		}
		seen[b] = true
		names = append(names, b)
	}
	sort.Strings(names)
	return names
}

// proposedInitLayout returns a map of absolute config-file path to content.
// default.hcl is always included; each unique non-default basename gets a
// thin `extends = "default"` override file.
func proposedInitLayout(configDir string, names []string) map[string]string {
	layout := map[string]string{
		filepath.Join(configDir, "default.hcl"): defaultInitTemplate,
	}
	for _, n := range names {
		if n == "default.hcl" {
			continue
		}
		layout[filepath.Join(configDir, n)] = extendsInitTemplate
	}
	return layout
}

func (a *app) printInitSummary(root string, files, names []string, proposed map[string]string) {
	fmt.Fprintf(a.out, "Scanning %s\n", root)
	fmt.Fprintf(a.out, "Found %d HCL/TF file(s)\n", len(files))
	if len(names) > 0 {
		fmt.Fprintln(a.out, "Unique filenames:")
		for _, n := range names {
			fmt.Fprintf(a.out, "  %s\n", n)
		}
	}
	fmt.Fprintln(a.out, "\nProposed layout:")
	for _, p := range sortedKeys(proposed) {
		fmt.Fprintf(a.out, "  %s\n", relPath(p))
	}
}

func (a *app) printInitContents(proposed map[string]string) {
	for _, p := range sortedKeys(proposed) {
		fmt.Fprintf(a.out, "\n--- %s ---\n", relPath(p))
		fmt.Fprint(a.out, proposed[p])
	}
}

func writeInitFiles(configDir string, proposed map[string]string) error {
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	for _, path := range sortedKeys(proposed) {
		if err := fsutil.WriteFileSafe(path, []byte(proposed[path]), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
