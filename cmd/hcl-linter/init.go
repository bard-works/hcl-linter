package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/bard-works/hcl-linter/internal/fsutil"
	"github.com/bard-works/hcl-linter/internal/termcolor"
)

var flagInitForce bool

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

// initOut is where init writes its output. Swappable in tests.
var initOut io.Writer = os.Stdout

func runInit(_ *cobra.Command, args []string) error {
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

	files := findHCLFiles(root)
	names := uniqueBasenames(files)

	configDir := filepath.Join(root, ".hcl-linter")
	proposed := proposedInitLayout(configDir, names)

	printInitSummary(root, files, names, proposed)

	if flagDryRun {
		printInitContents(proposed)
		return nil
	}

	if !flagInitForce {
		entries, _ := os.ReadDir(configDir)
		if len(entries) > 0 {
			fmt.Fprintf(initOut, "\n%s %s already contains:\n",
				termcolor.Warning("Refusing to overwrite:"), configDir)
			for _, e := range entries {
				fmt.Fprintf(initOut, "  %s\n", e.Name())
			}
			return errors.New(".hcl-linter/ is non-empty; use --force to overwrite")
		}
	}

	if err := writeInitFiles(configDir, proposed); err != nil {
		return err
	}

	fmt.Fprintf(initOut, "\n%s Wrote %d config file(s) to %s\n",
		termcolor.Success("OK:"), len(proposed), configDir)
	fmt.Fprintln(initOut, "Next steps:")
	fmt.Fprintln(initOut, "  hcl-linter validate-config")
	fmt.Fprintln(initOut, "  hcl-linter fix ./ --dry-run")
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

func printInitSummary(root string, files, names []string, proposed map[string]string) {
	fmt.Fprintf(initOut, "Scanning %s\n", root)
	fmt.Fprintf(initOut, "Found %d HCL/TF file(s)\n", len(files))
	if len(names) > 0 {
		fmt.Fprintln(initOut, "Unique filenames:")
		for _, n := range names {
			fmt.Fprintf(initOut, "  %s\n", n)
		}
	}
	fmt.Fprintln(initOut, "\nProposed layout:")
	for _, p := range sortedKeys(proposed) {
		rel, err := filepath.Rel(".", p)
		if err != nil {
			rel = p
		}
		fmt.Fprintf(initOut, "  %s\n", rel)
	}
}

func printInitContents(proposed map[string]string) {
	for _, p := range sortedKeys(proposed) {
		rel, err := filepath.Rel(".", p)
		if err != nil {
			rel = p
		}
		fmt.Fprintf(initOut, "\n--- %s ---\n", rel)
		fmt.Fprint(initOut, proposed[p])
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
