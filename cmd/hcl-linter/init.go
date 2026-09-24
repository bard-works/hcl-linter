package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bard-works/hcl-linter/internal/fsutil"
	"github.com/bard-works/hcl-linter/internal/termcolor"
)

// defaultInitTemplate is the fallback config for files without their own
// config. %s is the builtin preset chosen for the project.
const defaultInitTemplate = `# Fallback rules for any file without its own config in this directory.
# builtin:%[1]s is the recommended rule set shipped with hcl-linter; list it
# with ` + "`hcl-linter explain`" + `. To change a rule, redefine its block
# below - a block here replaces the preset's block of the same name.
extends = "builtin:%[1]s"

rules {
}
`

// extendsInitTemplate is a per-filename config; %s is what it extends.
const extendsInitTemplate = `extends = "%s"

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
	fmt.Fprintln(a.out, "  hcl-linter check .              # see what the rules report")
	fmt.Fprintln(a.out, "  hcl-linter fix . --dry-run     # preview auto-fixes")
	fmt.Fprintln(a.out, "  hcl-linter explain             # list rules you can enable")
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

// isTerragruntProject reports whether any target is a Terragrunt file.
func isTerragruntProject(names []string) bool {
	for _, n := range names {
		if n == "terragrunt.hcl" || n == "root.hcl" {
			return true
		}
	}
	return false
}

// proposedInitLayout returns a map of absolute config-file path to content.
// Config files are keyed by target name without extension, which is how the
// loader looks them up: main.tf -> main.hcl. default.hcl extends the preset
// matching the project; each other name gets a thin override. In a Terragrunt
// project, Terraform files extend builtin:terraform directly because the
// Terragrunt preset's block order makes no sense for them.
func proposedInitLayout(configDir string, names []string) map[string]string {
	terragrunt := isTerragruntProject(names)
	preset := "terraform"
	if terragrunt {
		preset = "terragrunt"
	}
	layout := map[string]string{
		filepath.Join(configDir, "default.hcl"): fmt.Sprintf(defaultInitTemplate, preset),
	}

	// A foo.hcl and foo.tf share foo.hcl; the .hcl file's needs win.
	stems := map[string]string{}
	for _, n := range names {
		stem := strings.TrimSuffix(n, filepath.Ext(n))
		if stem == "default" || stem == "" {
			continue
		}
		if prev, ok := stems[stem]; ok && filepath.Ext(prev) == ".hcl" {
			continue
		}
		stems[stem] = n
	}
	for stem, n := range stems {
		base := "default"
		if terragrunt && filepath.Ext(n) == ".tf" {
			base = "builtin:terraform"
		}
		layout[filepath.Join(configDir, stem+".hcl")] = fmt.Sprintf(extendsInitTemplate, base)
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
