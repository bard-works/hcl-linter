package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/diag"
	"github.com/bard-works/hcl-linter/internal/engine"
	"github.com/bard-works/hcl-linter/internal/termcolor"
)

var (
	flagVerbose           bool
	flagConfigSrc         string
	flagFilter            []string
	flagConcurrency       int
	flagFormat            bool
	flagDryRun            bool
	flagColor             string
	flagValidateRecursive bool
	flagIncludeHidden     bool

	Version   = "dev"
	BuildDate = "unknown"
	GitCommit = "unknown"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "hcl-linter",
		Short: "A configurable HCL linter with built-in rule sets for Terragrunt and Terraform",
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return termcolor.SetMode(flagColor)
		},
		Run: func(cmd *cobra.Command, _ []string) {
			_ = cmd.Help()
		},
	}

	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Show detailed output")
	rootCmd.PersistentFlags().StringVarP(&flagConfigSrc, "config-source", "c", "", "Config source: explicit path, or auto-detect from cwd/home/project")
	rootCmd.PersistentFlags().StringArrayVar(&flagFilter, "filter", nil, "Filter files by name pattern (glob supported, can be specified multiple times)")
	rootCmd.PersistentFlags().IntVar(&flagConcurrency, "concurrency", 0, "Max number of concurrent workers (0 = auto-detect based on CPU count, or use HCL_LINTER_MAX_CONCURRENCY env var)")
	rootCmd.PersistentFlags().StringVar(&flagColor, "color", termcolor.ModeAuto, "Colour output: auto, always, never")
	rootCmd.PersistentFlags().BoolVar(&flagIncludeHidden, "include-hidden", false, "Include files in hidden directories (directories starting with .)")

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
		Use:           "fix [path]",
		Short:         "Auto-fix HCL files",
		Args:          cobra.ExactArgs(1),
		RunE:          runFix,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	fixCmd.Flags().BoolVar(&flagFormat, "format", false, "Apply default formatting (block ordering, array formatting, blank line normalization) without requiring config rules")
	fixCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Show what fix would change (unified diff) without writing; exit non-zero if any changes needed")
	validateConfigCmd := &cobra.Command{
		Use:   "validate-config [path]",
		Short: "Validate .hcl-linter config files for unknown rules and misconfigurations",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runValidateConfig,
	}
	validateConfigCmd.Flags().BoolVar(&flagValidateRecursive, "recursive", false, "Validate every .hcl-linter/ directory found under the target path")
	initCmd := &cobra.Command{
		Use:           "init [path]",
		Short:         "Bootstrap a .hcl-linter/ config directory for the project",
		Args:          cobra.MaximumNArgs(1),
		RunE:          runInit,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	initCmd.Flags().BoolVar(&flagInitForce, "force", false, "Overwrite existing .hcl-linter/ contents")
	initCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Print proposed files to stdout without writing")
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("hcl-linter %s\n", Version)
			fmt.Printf("Build date: %s\n", BuildDate)
			fmt.Printf("Git commit: %s\n", GitCommit)
		},
	}

	rootCmd.AddCommand(lintCmd, checkCmd, fixCmd, validateConfigCmd, initCmd, newExplainCmd(), versionCmd)
	return rootCmd
}

func getLoader() (*config.Loader, *config.ConfigResult) {
	return config.LoadConfigDirWithResult(flagConfigSrc)
}

func runLint(_ *cobra.Command, args []string) error {
	path := args[0]
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("path error: %w", err)
	}
	loader, _ := loadConfig(path)
	eng := engine.New(loader)
	filesToLint := filterFilesByConfig(loader, nil, path)
	if len(filesToLint) == 0 {
		return nil
	}
	if flagVerbose {
		fmt.Println("Checking files:")
		for _, f := range filesToLint {
			rel, _ := filepath.Rel(".", f)
			if rel == "" {
				rel = f
			}
			fmt.Printf("  %s\n", rel)
		}
	}
	maxConcurrency := resolveConcurrency()
	allResults := eng.LintFiles(filesToLint, maxConcurrency)
	printLintResults(allResults, false)
	return nil
}

func runCheck(_ *cobra.Command, args []string) error {
	path := args[0]
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("path error: %w", err)
	}
	loader, _ := loadConfig(path)
	eng := engine.New(loader)
	filesToLint := filterFilesByConfig(loader, nil, path)
	if len(filesToLint) == 0 {
		return nil
	}
	if flagVerbose {
		fmt.Println("Checking files:")
		for _, f := range filesToLint {
			rel, _ := filepath.Rel(".", f)
			if rel == "" {
				rel = f
			}
			fmt.Printf("  %s\n", rel)
		}
	}
	maxConcurrency := resolveConcurrency()
	allResults := eng.LintFiles(filesToLint, maxConcurrency)
	hasErrors := printLintResults(allResults, true)
	if hasErrors {
		return errors.New("lint check failed")
	}
	return nil
}

func loadConfig(_ string) (*config.Loader, *config.ConfigResult) {
	loader, configResult := getLoader()
	if configResult.Source == config.ConfigSourceNone {
		fmt.Fprintf(os.Stderr, "%s %s\n", termcolor.Warning("Warning:"), configResult.WarningMsg)
	} else {
		fmt.Printf("Using config: %s (%s)\n", configResult.SourcePath, configResult.Source.String())
		if configResult.WarningMsg != "" {
			fmt.Fprintf(os.Stderr, "%s %s\n", termcolor.Warning("Warning:"), configResult.WarningMsg)
		}
		warnConfigIssues(configResult.SourcePath)
	}
	if loader == nil {
		loader = &config.Loader{}
	}
	return loader, configResult
}

func resolveFiles(path string) []string {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	if info.IsDir() {
		return findHCLFiles(path)
	}
	return []string{path}
}

func filterFilesByConfig(loader *config.Loader, configResult *config.ConfigResult, path string) []string {
	files := resolveFiles(path)
	if len(flagFilter) > 0 {
		files = filterFiles(files)
	}
	var filesToProcess []string
	for _, file := range files {
		hasSpecificConfig := loader.HasSpecificConfigForFile(file)
		relPath, _ := filepath.Rel(".", file)
		if relPath == "" {
			relPath = file
		}
		if !loader.HasConfigForFile(file) {
			if configResult != nil && configResult.Source != config.ConfigSourceNone {
				fmt.Printf("%s No config found for %s, skipping\n", termcolor.Warning("Warning:"), relPath)
			}
			continue
		}
		if !hasSpecificConfig {
			fmt.Printf("%s No specific config for %s, using defaults\n", termcolor.Warning("Warning:"), relPath)
		}
		filesToProcess = append(filesToProcess, file)
	}
	return filesToProcess
}

func resolveConcurrency() int {
	if flagConcurrency > 0 {
		return flagConcurrency
	}
	return config.GetMaxConcurrency(nil)
}

func printLintResults(allResults []*diag.Result, _ bool) (hasErrors bool) {
	for _, result := range allResults {
		if flagVerbose || len(result.Issues) > 0 {
			relPath, err := filepath.Rel(".", result.File)
			if err != nil {
				relPath = result.File
			}
			fmt.Printf("\n%s:\n", termcolor.Path(relPath))
			for _, issue := range result.Issues {
				if issue.Severity == diag.SeverityError {
					hasErrors = true
				}
				printIssue(issue)
			}
		}
	}

	totalIssues := 0
	for _, r := range allResults {
		totalIssues += len(r.Issues)
	}

	fmt.Println(strings.Repeat("-", 40))
	if totalIssues > 0 {
		fmt.Printf("Total: %d issue(s) in %d file(s)\n", totalIssues, len(allResults))
	} else {
		fmt.Println(termcolor.Success("All files pass!"))
	}
	return hasErrors
}

func runFix(cmd *cobra.Command, args []string) error {
	if flagFormat {
		return runFormatMode(cmd, args)
	}
	path := args[0]
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("path error: %w", err)
	}
	loader, _ := loadConfig(path)
	eng := engine.New(loader)
	eng.DryRun = flagDryRun
	filesToFix := filterFilesByConfig(loader, nil, path)
	if len(filesToFix) == 0 {
		return nil
	}
	maxConcurrency := resolveConcurrency()
	if flagVerbose && len(filesToFix) > 1 {
		fmt.Printf("Fixing %d files with concurrency %d\n", len(filesToFix), maxConcurrency)
	}
	results := eng.FixFiles(filesToFix, maxConcurrency)
	totalChanges, wouldChange := handleFixResults(results, flagDryRun)
	return finalizeFixRun(totalChanges, wouldChange, flagDryRun)
}

func runFormatMode(_ *cobra.Command, args []string) error {
	path := args[0]
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("path error: %w", err)
	}
	loader, configResult := loadConfig(path)
	eng := engine.New(loader)
	eng.DryRun = flagDryRun
	files := filterFilesByConfig(loader, configResult, path)
	if len(files) == 0 {
		return nil
	}
	maxConcurrency := resolveConcurrency()
	if flagVerbose && len(files) > 1 {
		fmt.Printf("Formatting %d files with concurrency %d\n", len(files), maxConcurrency)
	}
	results := eng.FormatFixFiles(files, maxConcurrency)
	totalChanges, wouldChange := handleFixResults(results, flagDryRun)
	return finalizeFixRun(totalChanges, wouldChange, flagDryRun)
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

func runValidateConfig(_ *cobra.Command, args []string) error {
	if flagValidateRecursive {
		return runValidateConfigRecursive(args)
	}

	var configDir string
	if len(args) == 1 {
		configDir = args[0]
	}

	loader, result := config.LoadConfigDirWithResult(configDir)
	if result.Source == config.ConfigSourceNone {
		return errors.New("no config directory found")
	}
	_ = loader

	fmt.Printf("Validating config: %s\n", result.SourcePath)

	issues := config.ValidateDir(result.SourcePath)
	if len(issues) == 0 {
		fmt.Println(termcolor.Success("Config OK"))
		return nil
	}

	for _, issue := range issues {
		fmt.Fprintf(os.Stderr, "  %s %s\n", termcolor.Error("error:"), issue)
	}
	return fmt.Errorf("%d config issue(s) found", len(issues))
}

func runValidateConfigRecursive(args []string) error {
	root := "."
	if len(args) == 1 {
		root = args[0]
	}

	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("path error: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("--recursive target must be a directory: %s", root)
	}

	dirs := findHCLLinterDirs(root)
	if len(dirs) == 0 {
		return fmt.Errorf("no .hcl-linter/ directories found under %s", root)
	}

	var totalIssues int
	for _, dir := range dirs {
		fmt.Printf("Validating config: %s\n", dir)
		issues := config.ValidateDir(dir)
		if len(issues) == 0 {
			fmt.Println("  " + termcolor.Success("OK"))
			continue
		}
		totalIssues += len(issues)
		for _, issue := range issues {
			fmt.Fprintf(os.Stderr, "  %s %s\n", termcolor.Error("error:"), issue)
		}
	}

	if totalIssues == 0 {
		return nil
	}
	return fmt.Errorf("%d config issue(s) found across %d directory(s)", totalIssues, len(dirs))
}

// findHCLLinterDirs walks root and returns every `.hcl-linter/` directory
// found, sorted by path. Does not descend into `.hcl-linter/` itself or
// other hidden sibling directories (except the .hcl-linter/ match).
func findHCLLinterDirs(root string) []string {
	var dirs []string
	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		name := info.Name()
		if name == ".hcl-linter" {
			dirs = append(dirs, path)
			return filepath.SkipDir
		}
		if path != root && strings.HasPrefix(name, ".") {
			return filepath.SkipDir
		}
		return nil
	}
	_ = filepath.Walk(root, walkFn)
	return dirs
}

// warnConfigIssues prints config validation warnings to stderr. Called at
// startup for lint/fix so users see config problems even without running
// validate-config explicitly.
func warnConfigIssues(configDir string) {
	if configDir == "" {
		return
	}
	issues := config.ValidateDir(configDir)
	for _, issue := range issues {
		fmt.Fprintf(os.Stderr, "%s %s\n", termcolor.Warning("Config warning:"), issue)
	}
}

// printIssue renders a single lint issue with severity colouring and an
// optional faint location suffix in verbose mode.
func printIssue(issue diag.Issue) {
	label := fmt.Sprintf("[%s]", issue.Severity)
	if issue.Severity == diag.SeverityError {
		label = termcolor.Error(label)
	} else {
		label = termcolor.Warning(label)
	}
	fmt.Printf("  %s %s: %s\n", label, termcolor.Rule(issue.Rule), issue.Message)
	if flagVerbose && issue.Location.Filename != "" {
		fmt.Printf("    %s\n", termcolor.Location(
			fmt.Sprintf("at %s:%d", issue.Location.Filename, issue.Location.Start.Line)))
	}
}

// printFixResult renders a single fix result (error or change count) and
// returns the change count contributed to the running total.
func printFixResult(result *engine.FixResult) int {
	relPath, _ := filepath.Rel(".", result.File)
	if relPath == "" {
		relPath = result.File
	}
	if result.Error != nil {
		fmt.Printf("%s %s: %v\n", termcolor.Error("Error fixing"), relPath, result.Error)
		return 0
	}
	if result.Changes > 0 {
		fmt.Printf("Fixed %s: %d change(s)\n", relPath, result.Changes)
		return result.Changes
	}
	return 0
}

// handleFixResults prints either the standard written-file summary or, in
// dry-run mode, a unified diff per file. Returns (totalChanges, wouldChange):
// totalChanges is the number of files reported as changed; wouldChange is true
// iff dry-run found at least one file that differs.
func handleFixResults(results []*engine.FixResult, dryRun bool) (totalChanges int, wouldChange bool) {
	for _, result := range results {
		relPath, _ := filepath.Rel(".", result.File)
		if relPath == "" {
			relPath = result.File
		}

		if result.Error != nil {
			fmt.Printf("%s %s: %v\n", termcolor.Error("Error fixing"), relPath, result.Error)
			continue
		}

		if !dryRun {
			totalChanges += printFixResult(result)
			continue
		}

		before, err := os.ReadFile(result.File)
		if err != nil {
			fmt.Printf("%s %s: %v\n", termcolor.Error("Error reading"), relPath, err)
			continue
		}
		if printDiff(relPath, string(before), result.Content) {
			wouldChange = true
			totalChanges++
		}
	}
	return totalChanges, wouldChange
}

// finalizeFixRun prints the trailing summary (or dry-run error) common to
// runFixMode and runFormatMode.
func finalizeFixRun(totalChanges int, wouldChange, dryRun bool) error {
	if dryRun {
		if wouldChange {
			return fmt.Errorf("dry run: %d file(s) would be changed", totalChanges)
		}
		fmt.Println(termcolor.Success("No changes needed"))
		return nil
	}
	if totalChanges > 0 {
		fmt.Printf("\nTotal: %d change(s) applied\n", totalChanges)
	} else {
		fmt.Println(termcolor.Success("No changes needed"))
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
			if !flagIncludeHidden && strings.HasPrefix(info.Name(), ".") {
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
