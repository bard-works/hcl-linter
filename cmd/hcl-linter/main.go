package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/fix"
	"github.com/bard-works/hcl-linter/internal/linter"
	"github.com/spf13/cobra"
)

var (
	flagVerbose     bool
	flagConfigSrc   string
	flagFilter      []string
	flagConcurrency int

	Version   = "dev"
	BuildDate = "unknown"
	GitCommit = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "hcl-linter",
		Short: "A configurable linter for Terragrunt HCL files",
		Run: func(cmd *cobra.Command, _ []string) {
			_ = cmd.Help()
		},
	}

	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Show detailed output")
	rootCmd.PersistentFlags().StringVarP(&flagConfigSrc, "config-source", "c", "", "Config source: explicit path, or auto-detect from cwd/home/project")
	rootCmd.PersistentFlags().StringArrayVar(&flagFilter, "filter", nil, "Filter files by name pattern (glob supported, can be specified multiple times)")
	rootCmd.PersistentFlags().IntVar(&flagConcurrency, "concurrency", 0, "Max number of concurrent workers (0 = auto-detect based on CPU count, or use HCL_LINTER_MAX_CONCURRENCY env var)")

	lintCmd := &cobra.Command{
		Use:   "lint [path]",
		Short: "Lint HCL files",
		Args:  cobra.ExactArgs(1),
		RunE:  runLint,
	}
	checkCmd := &cobra.Command{
		Use:   "check [path]",
		Short: "Check HCL files (exit 1 if issues found)",
		Args:  cobra.ExactArgs(1),
		RunE:  runCheck,
	}
	fixCmd := &cobra.Command{
		Use:   "fix [path]",
		Short: "Auto-fix HCL files",
		Args:  cobra.ExactArgs(1),
		RunE:  runFix,
	}
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("hcl-linter %s\n", Version)
			fmt.Printf("Build date: %s\n", BuildDate)
			fmt.Printf("Git commit: %s\n", GitCommit)
		},
	}

	rootCmd.AddCommand(lintCmd, checkCmd, fixCmd, versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func getLoader() (*config.Loader, *config.ConfigResult) {
	return config.LoadConfigDirWithResult(flagConfigSrc)
}

func runLint(cmd *cobra.Command, args []string) error {
	return run(cmd, args, false, false)
}

func runCheck(cmd *cobra.Command, args []string) error {
	err := runLintModeWithExitCode(cmd, args)
	if err != nil {
		return err
	}
	return nil
}

func runLintModeWithExitCode(_ *cobra.Command, args []string) error {
	path := args[0]

	loader, configResult := getLoader()

	if configResult.Source == config.ConfigSourceNone {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", configResult.WarningMsg)
	} else {
		fmt.Printf("Using config: %s (%s)\n", configResult.SourcePath, configResult.Source.String())
		if configResult.WarningMsg != "" {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", configResult.WarningMsg)
		}
	}

	if loader == nil {
		loader = &config.Loader{}
	}

	var files []string
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path error: %w", err)
	}

	if info.IsDir() {
		files = findHCLFiles(path)
	} else {
		files = []string{path}
	}

	if flagVerbose {
		fmt.Printf("Found %d files to lint\n", len(files))
	}

	if len(flagFilter) > 0 {
		files = filterFiles(files)
	}

	l := linter.NewLinter(loader)

	var filesToLint []string
	for _, file := range files {
		hasSpecificConfig := loader.HasSpecificConfigForFile(file)
		if !hasSpecificConfig {
			relPath, _ := filepath.Rel(".", file)
			if relPath == "" {
				relPath = file
			}
			if !loader.HasConfigForFile(file) {
				fmt.Printf("Warning: No config found for %s, skipping\n", relPath)
				continue
			}
			fmt.Printf("Warning: No specific config for %s, using defaults\n", relPath)
		}
		filesToLint = append(filesToLint, file)
	}

	maxConcurrency := flagConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = config.GetMaxConcurrency(nil)
	}
	if flagVerbose && len(filesToLint) > 1 {
		fmt.Printf("Linting %d files with concurrency %d\n", len(filesToLint), maxConcurrency)
	}

	allResults := l.LintFiles(filesToLint, maxConcurrency)

	hasErrors := false
	for _, result := range allResults {
		if flagVerbose || len(result.Issues) > 0 {
			relPath, err := filepath.Rel(".", result.File)
			if err != nil {
				relPath = result.File
			}
			fmt.Printf("\n%s:\n", relPath)
			for _, issue := range result.Issues {
				severity := issue.Severity
				if issue.Severity == linter.SeverityError {
					hasErrors = true
				}
				fmt.Printf("  [%s] %s: %s\n", severity, issue.Rule, issue.Message)
				if flagVerbose && issue.Location.Filename != "" {
					fmt.Printf("    at %s:%d\n", issue.Location.Filename, issue.Location.Start.Line)
				}
			}
		}
	}

	if !flagVerbose {
		for _, result := range allResults {
			if len(result.Issues) > 0 {
				for _, issue := range result.Issues {
					fmt.Printf("  [%s] %s: %s\n", issue.Severity, issue.Rule, issue.Message)
				}
			}
		}
	}

	totalIssues := 0
	for _, r := range allResults {
		totalIssues += len(r.Issues)
	}

	if totalIssues > 0 {
		fmt.Printf("\nTotal: %d issue(s) in %d file(s)\n", totalIssues, len(allResults))
	} else {
		fmt.Println("All files pass!")
	}

	if hasErrors {
		fmt.Println("lint check failed")
		os.Exit(1)
	}

	return nil
}

func runFix(cmd *cobra.Command, args []string) error {
	return run(cmd, args, false, true)
}

func run(_ *cobra.Command, args []string, checkMode, fixMode bool) error {
	path := args[0]

	loader, configResult := getLoader()

	if configResult.Source == config.ConfigSourceNone {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", configResult.WarningMsg)
	} else {
		fmt.Printf("Using config: %s (%s)\n", configResult.SourcePath, configResult.Source.String())
		if configResult.WarningMsg != "" {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", configResult.WarningMsg)
		}
	}

	if loader == nil {
		loader = &config.Loader{}
	}

	var files []string
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path error: %w", err)
	}

	if info.IsDir() {
		files = findHCLFiles(path)
	} else {
		files = []string{path}
	}

	if flagVerbose {
		fmt.Printf("Found %d files to lint\n", len(files))
	}

	if len(flagFilter) > 0 {
		files = filterFiles(files)
	}

	if fixMode {
		return runFixMode(loader, files)
	}

	return runLintMode(loader, files, checkMode)
}

func filterFiles(files []string) []string {
	var matched []string
	for _, f := range files {
		filename := filepath.Base(f)
		if matchesFilter(filename) {
			matched = append(matched, f)
		}
	}

	if len(matched) > 0 {
		fmt.Println("\nMatched files:")
		for _, f := range matched {
			fmt.Printf("  %s\n", f)
		}
	}

	return matched
}

func matchesFilter(filename string) bool {
	if len(flagFilter) == 0 {
		return true
	}

	for _, pattern := range flagFilter {
		if matchPattern(pattern, filename) {
			return true
		}
	}
	return false
}

func matchPattern(pattern, filename string) bool {
	if strings.Contains(pattern, "*") {
		return matchGlob(filename, pattern)
	}
	return filename == pattern
}

func matchGlob(filename, pattern string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 2 {
		prefix := parts[0]
		suffix := parts[1]
		return strings.HasPrefix(filename, prefix) && strings.HasSuffix(filename, suffix)
	}
	if strings.HasPrefix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(filename, suffix)
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(filename, prefix)
	}
	return filename == pattern
}

func runLintMode(loader *config.Loader, files []string, checkMode bool) error {
	l := linter.NewLinter(loader)

	var filesToLint []string
	for _, file := range files {
		hasSpecificConfig := loader.HasSpecificConfigForFile(file)
		if !hasSpecificConfig {
			relPath, _ := filepath.Rel(".", file)
			if relPath == "" {
				relPath = file
			}
			if !loader.HasConfigForFile(file) {
				fmt.Printf("Warning: No config found for %s, skipping\n", relPath)
				continue
			}
			fmt.Printf("Warning: No specific config for %s, using defaults\n", relPath)
		}
		filesToLint = append(filesToLint, file)
	}

	maxConcurrency := flagConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = config.GetMaxConcurrency(nil)
	}
	if flagVerbose && len(filesToLint) > 1 {
		fmt.Printf("Linting %d files with concurrency %d\n", len(filesToLint), maxConcurrency)
	}

	allResults := l.LintFiles(filesToLint, maxConcurrency)

	hasErrors := false
	for _, result := range allResults {
		if flagVerbose || len(result.Issues) > 0 {
			relPath, err := filepath.Rel(".", result.File)
			if err != nil {
				relPath = result.File
			}
			fmt.Printf("\n%s:\n", relPath)
			for _, issue := range result.Issues {
				severity := issue.Severity
				if issue.Severity == linter.SeverityError {
					hasErrors = true
				}
				fmt.Printf("  [%s] %s: %s\n", severity, issue.Rule, issue.Message)
				if flagVerbose && issue.Location.Filename != "" {
					fmt.Printf("    at %s:%d\n", issue.Location.Filename, issue.Location.Start.Line)
				}
			}
		}
	}

	if !flagVerbose && !checkMode {
		for _, result := range allResults {
			if len(result.Issues) > 0 {
				fmt.Println(result.Summary())
			}
		}
	}

	totalIssues := 0
	for _, r := range allResults {
		totalIssues += len(r.Issues)
	}

	if totalIssues > 0 {
		fmt.Printf("\nTotal: %d issue(s) in %d file(s)\n", totalIssues, len(allResults))
	} else {
		fmt.Println("All files pass!")
	}

	if checkMode && hasErrors {
		return errors.New("lint check failed")
	}

	return nil
}

func runFixMode(loader *config.Loader, files []string) error {
	fixer := fix.NewFixer(loader)

	var filesToFix []string
	for _, file := range files {
		hasSpecificConfig := loader.HasSpecificConfigForFile(file)
		if !loader.HasConfigForFile(file) {
			relPath, _ := filepath.Rel(".", file)
			if relPath == "" {
				relPath = file
			}
			fmt.Printf("Warning: No config found for %s, skipping\n", relPath)
			continue
		}
		if !hasSpecificConfig {
			relPath, _ := filepath.Rel(".", file)
			if relPath == "" {
				relPath = file
			}
			fmt.Printf("Warning: No specific config for %s, using defaults\n", relPath)
		}
		filesToFix = append(filesToFix, file)
	}

	maxConcurrency := flagConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = config.GetMaxConcurrency(nil)
	}
	if flagVerbose && len(filesToFix) > 1 {
		fmt.Printf("Fixing %d files with concurrency %d\n", len(filesToFix), maxConcurrency)
	}

	results := fixer.FixFiles(filesToFix, maxConcurrency)

	totalChanges := 0
	for _, result := range results {
		relPath, _ := filepath.Rel(".", result.File)
		if relPath == "" {
			relPath = result.File
		}
		if result.Error != nil {
			fmt.Printf("Error fixing %s: %v\n", relPath, result.Error)
			continue
		}
		if result.Changes > 0 {
			fmt.Printf("Fixed %s: %d change(s)\n", relPath, result.Changes)
			totalChanges += result.Changes
		}
	}

	if totalChanges > 0 {
		fmt.Printf("\nTotal: %d change(s) applied\n", totalChanges)
	} else {
		fmt.Println("No changes needed")
	}

	return nil
}

func findHCLFiles(root string) []string {
	var files []string

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasSuffix(path, ".hcl") || strings.HasSuffix(path, ".tf") {
			files = append(files, path)
		}

		return nil
	}

	_ = filepath.Walk(root, walkFn)
	return files
}
