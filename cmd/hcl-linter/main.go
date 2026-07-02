package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/diag"
	"github.com/bard-works/hcl-linter/internal/engine"
	"github.com/bard-works/hcl-linter/internal/termcolor"
)

// Version information, injected at build time via ldflags.
var (
	Version   = "dev"
	BuildDate = "unknown"
	GitCommit = "unknown"
)

// Exit codes. Diagnostics always go to stderr, results to stdout, so CI can
// rely on both the code and the streams.
const (
	exitOK       = 0 // no findings
	exitFindings = 1 // lint findings / dry-run drift / config issues
	exitUsage    = 2 // bad invocation, missing path, no config
	exitExec     = 3 // the tool itself failed (parse error, IO error, interrupted)
)

// findingsError marks failures caused by findings in the linted input (exit 1).
type findingsError struct{ msg string }

func (e *findingsError) Error() string { return e.msg }

func findingsErrorf(format string, args ...any) error {
	return &findingsError{msg: fmt.Sprintf(format, args...)}
}

// execError marks failures of the tool itself (exit 3).
type execError struct{ msg string }

func (e *execError) Error() string { return e.msg }

func execErrorf(format string, args ...any) error {
	return &execError{msg: fmt.Sprintf(format, args...)}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx))
}

// run executes the CLI and maps the returned error to an exit code. This is
// the only place errors are printed and the only caller of os.Exit lives in
// main, so deferred functions elsewhere always execute.
func run(ctx context.Context) int {
	err := newRootCmd().ExecuteContext(ctx)
	if err == nil {
		return exitOK
	}
	fmt.Fprintln(os.Stderr, err)

	var fe *findingsError
	if errors.As(err, &fe) {
		return exitFindings
	}
	var ee *execError
	if errors.As(err, &ee) {
		return exitExec
	}
	return exitUsage
}

// app carries per-invocation state: flag values bound by cobra and the output
// streams. out receives results (issues, diffs, summaries, explain output);
// errOut receives diagnostics (warnings, banners, progress). Tests construct
// an app directly with buffers instead of mutating globals.
type app struct {
	out    io.Writer
	errOut io.Writer

	verbose           bool
	configSrc         string
	filter            []string
	concurrency       int
	format            bool
	dryRun            bool
	color             string
	validateRecursive bool
	includeHidden     bool
	initForce         bool
}

func newApp() *app {
	return &app{
		out:    os.Stdout,
		errOut: os.Stderr,
		color:  termcolor.ModeAuto,
	}
}

// cmdContext returns the command's context, tolerating the nil command that
// tests pass when invoking handlers directly.
func cmdContext(cmd *cobra.Command) context.Context {
	if cmd == nil {
		return context.Background()
	}
	return cmd.Context()
}

func newRootCmd() *cobra.Command {
	a := newApp()

	rootCmd := &cobra.Command{
		Use:   "hcl-linter",
		Short: "A configurable HCL linter with built-in rule sets for Terragrunt and Terraform",
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return termcolor.SetMode(a.color)
		},
		Run: func(cmd *cobra.Command, _ []string) {
			_ = cmd.Help()
		},
		// run() in main is the single error printer; without this cobra would
		// print every RunE error a second time.
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	rootCmd.PersistentFlags().BoolVarP(&a.verbose, "verbose", "v", false, "Show detailed output")
	rootCmd.PersistentFlags().
		StringVarP(&a.configSrc, "config-source", "c", "", "Config source: explicit path, or auto-detect from cwd/home/project")
	rootCmd.PersistentFlags().
		StringArrayVar(&a.filter, "filter", nil, "Filter files by name pattern (glob supported, can be specified multiple times)")
	rootCmd.PersistentFlags().
		IntVar(&a.concurrency, "concurrency", 0, "Max number of concurrent workers (0 = auto-detect based on CPU count, or use HCL_LINTER_MAX_CONCURRENCY env var)")
	rootCmd.PersistentFlags().StringVar(&a.color, "color", termcolor.ModeAuto, "Colour output: auto, always, never")
	rootCmd.PersistentFlags().
		BoolVar(&a.includeHidden, "include-hidden", false, "Include files in hidden directories (directories starting with .)")

	lintCmd := &cobra.Command{
		Use:   "lint [path]",
		Short: "Lint HCL files",
		Args:  cobra.ExactArgs(1),
		RunE:  a.runLint,
	}
	checkCmd := &cobra.Command{
		Use:   "check [path]",
		Short: "Check HCL files (exit 1 if issues found)",
		Args:  cobra.ExactArgs(1),
		RunE:  a.runCheck,
	}
	fixCmd := &cobra.Command{
		Use:   "fix [path]",
		Short: "Auto-fix HCL files",
		Args:  cobra.ExactArgs(1),
		RunE:  a.runFix,
	}
	fixCmd.Flags().
		BoolVar(&a.format, "format", false, "Apply default formatting (block ordering, array formatting, blank line normalization) without requiring config rules")
	fixCmd.Flags().
		BoolVar(&a.dryRun, "dry-run", false, "Show what fix would change (unified diff) without writing; exit non-zero if any changes needed")
	validateConfigCmd := &cobra.Command{
		Use:   "validate-config [path]",
		Short: "Validate .hcl-linter config files for unknown rules and misconfigurations",
		Args:  cobra.MaximumNArgs(1),
		RunE:  a.runValidateConfig,
	}
	validateConfigCmd.Flags().
		BoolVar(&a.validateRecursive, "recursive", false, "Validate every .hcl-linter/ directory found under the target path")
	initCmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Bootstrap a .hcl-linter/ config directory for the project",
		Args:  cobra.MaximumNArgs(1),
		RunE:  a.runInit,
	}
	initCmd.Flags().BoolVar(&a.initForce, "force", false, "Overwrite existing .hcl-linter/ contents")
	initCmd.Flags().BoolVar(&a.dryRun, "dry-run", false, "Print proposed files to stdout without writing")
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Fprintf(a.out, "hcl-linter %s\n", Version)
			fmt.Fprintf(a.out, "Build date: %s\n", BuildDate)
			fmt.Fprintf(a.out, "Git commit: %s\n", GitCommit)
		},
	}

	rootCmd.AddCommand(lintCmd, checkCmd, fixCmd, validateConfigCmd, initCmd, a.newExplainCmd(), versionCmd)
	return rootCmd
}

func (a *app) getLoader() (*config.Loader, *config.ConfigResult) {
	return config.LoadConfigDirWithResult(a.configSrc)
}

func (a *app) runLint(cmd *cobra.Command, args []string) error {
	path := args[0]
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("path error: %w", err)
	}
	loader, _ := a.loadConfig()
	eng := engine.New(loader)
	filesToLint := a.filterFilesByConfig(loader, nil, path)
	if len(filesToLint) == 0 {
		return nil
	}
	a.listFilesVerbose(filesToLint)
	allResults := eng.LintFiles(cmdContext(cmd), filesToLint, a.resolveConcurrency())
	if err := interrupted(cmd); err != nil {
		return err
	}
	_, execFailures := a.printLintResults(allResults)
	if execFailures > 0 {
		return execErrorf("%d file(s) could not be linted", execFailures)
	}
	return nil
}

func (a *app) runCheck(cmd *cobra.Command, args []string) error {
	path := args[0]
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("path error: %w", err)
	}
	loader, _ := a.loadConfig()
	eng := engine.New(loader)
	filesToLint := a.filterFilesByConfig(loader, nil, path)
	if len(filesToLint) == 0 {
		return nil
	}
	a.listFilesVerbose(filesToLint)
	allResults := eng.LintFiles(cmdContext(cmd), filesToLint, a.resolveConcurrency())
	if err := interrupted(cmd); err != nil {
		return err
	}
	hasErrors, execFailures := a.printLintResults(allResults)
	if execFailures > 0 {
		return execErrorf("%d file(s) could not be linted", execFailures)
	}
	if hasErrors {
		return findingsErrorf("lint check failed")
	}
	return nil
}

// interrupted reports a cancelled command context as an execution error.
func interrupted(cmd *cobra.Command) error {
	if err := cmdContext(cmd).Err(); err != nil {
		return execErrorf("interrupted: %v", err)
	}
	return nil
}

// listFilesVerbose prints the file list to the diagnostic stream when
// --verbose is on.
func (a *app) listFilesVerbose(files []string) {
	if !a.verbose {
		return
	}
	fmt.Fprintln(a.errOut, "Checking files:")
	for _, f := range files {
		fmt.Fprintf(a.errOut, "  %s\n", relPath(f))
	}
}

// relPath renders path relative to the working directory when possible.
func relPath(path string) string {
	rel, err := filepath.Rel(".", path)
	if err != nil || rel == "" {
		return path
	}
	return rel
}

func (a *app) loadConfig() (*config.Loader, *config.ConfigResult) {
	loader, configResult := a.getLoader()
	if configResult.Source == config.ConfigSourceNone {
		fmt.Fprintf(a.errOut, "%s %s\n", termcolor.Warning("Warning:"), configResult.WarningMsg)
	} else {
		fmt.Fprintf(a.errOut, "Using config: %s (%s)\n", configResult.SourcePath, configResult.Source.String())
		if configResult.WarningMsg != "" {
			fmt.Fprintf(a.errOut, "%s %s\n", termcolor.Warning("Warning:"), configResult.WarningMsg)
		}
		a.warnConfigIssues(configResult.SourcePath)
	}
	if loader == nil {
		loader = &config.Loader{}
	}
	return loader, configResult
}

func (a *app) resolveFiles(path string) []string {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	if info.IsDir() {
		return a.findHCLFiles(path)
	}
	return []string{path}
}

func (a *app) filterFilesByConfig(loader *config.Loader, configResult *config.ConfigResult, path string) []string {
	files := a.resolveFiles(path)
	if len(a.filter) > 0 {
		files = a.filterFiles(files)
	}
	var filesToProcess []string
	for _, file := range files {
		hasSpecificConfig := loader.HasSpecificConfigForFile(file)
		rel := relPath(file)
		if !loader.HasConfigForFile(file) {
			if configResult != nil && configResult.Source != config.ConfigSourceNone {
				fmt.Fprintf(a.errOut, "%s No config found for %s, skipping\n", termcolor.Warning("Warning:"), rel)
			}
			continue
		}
		if !hasSpecificConfig {
			fmt.Fprintf(a.errOut, "%s No specific config for %s, using defaults\n", termcolor.Warning("Warning:"), rel)
		}
		filesToProcess = append(filesToProcess, file)
	}
	return filesToProcess
}

func (a *app) resolveConcurrency() int {
	if a.concurrency > 0 {
		return a.concurrency
	}
	return config.GetMaxConcurrency(nil)
}

// printLintResults renders all issues to the result stream and returns whether
// any error-severity issues were found and how many files failed to lint at
// all (parse/IO failures surfaced as linter_error).
func (a *app) printLintResults(allResults []*diag.Result) (hasErrors bool, execFailures int) {
	for _, result := range allResults {
		for _, issue := range result.Issues {
			if issue.Severity == diag.SeverityError {
				hasErrors = true
			}
			if issue.Rule == "linter_error" {
				execFailures++
			}
			a.printIssue(result.File, issue)
		}
	}

	totalIssues := 0
	for _, r := range allResults {
		totalIssues += len(r.Issues)
	}

	fmt.Fprintln(a.out, strings.Repeat("-", 40))
	if totalIssues > 0 {
		fmt.Fprintf(a.out, "Total: %d issue(s) in %d file(s)\n", totalIssues, len(allResults))
	} else {
		fmt.Fprintln(a.out, termcolor.Success("All files pass!"))
	}
	return hasErrors, execFailures
}

func (a *app) runFix(cmd *cobra.Command, args []string) error {
	if a.format {
		return a.runFormatMode(cmd, args)
	}
	path := args[0]
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("path error: %w", err)
	}
	loader, _ := a.loadConfig()
	eng := engine.New(loader)
	eng.DryRun = a.dryRun
	filesToFix := a.filterFilesByConfig(loader, nil, path)
	if len(filesToFix) == 0 {
		return nil
	}
	maxConcurrency := a.resolveConcurrency()
	if a.verbose && len(filesToFix) > 1 {
		fmt.Fprintf(a.errOut, "Fixing %d files with concurrency %d\n", len(filesToFix), maxConcurrency)
	}
	results := eng.FixFiles(cmdContext(cmd), filesToFix, maxConcurrency)
	if err := interrupted(cmd); err != nil {
		return err
	}
	totalChanges, wouldChange := a.handleFixResults(results, a.dryRun)
	return a.finalizeFixRun(totalChanges, wouldChange, a.dryRun)
}

func (a *app) runFormatMode(cmd *cobra.Command, args []string) error {
	path := args[0]
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("path error: %w", err)
	}
	loader, configResult := a.loadConfig()
	eng := engine.New(loader)
	eng.DryRun = a.dryRun
	files := a.filterFilesByConfig(loader, configResult, path)
	if len(files) == 0 {
		return nil
	}
	maxConcurrency := a.resolveConcurrency()
	if a.verbose && len(files) > 1 {
		fmt.Fprintf(a.errOut, "Formatting %d files with concurrency %d\n", len(files), maxConcurrency)
	}
	results := eng.FormatFixFiles(cmdContext(cmd), files, maxConcurrency)
	if err := interrupted(cmd); err != nil {
		return err
	}
	totalChanges, wouldChange := a.handleFixResults(results, a.dryRun)
	return a.finalizeFixRun(totalChanges, wouldChange, a.dryRun)
}

func (a *app) filterFiles(files []string) []string {
	var matched []string
	for _, f := range files {
		if a.matchesFilter(filepath.Base(f)) {
			matched = append(matched, f)
		}
	}

	if len(matched) > 0 {
		fmt.Fprintln(a.errOut, "\nMatched files:")
		for _, f := range matched {
			fmt.Fprintf(a.errOut, "  %s\n", f)
		}
	}

	return matched
}

func (a *app) matchesFilter(filename string) bool {
	if len(a.filter) == 0 {
		return true
	}

	for _, pattern := range a.filter {
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

func (a *app) runValidateConfig(cmd *cobra.Command, args []string) error {
	if a.validateRecursive {
		return a.runValidateConfigRecursive(cmd, args)
	}

	var configDir string
	if len(args) == 1 {
		configDir = args[0]
	}

	_, result := config.LoadConfigDirWithResult(configDir)
	if result.Source == config.ConfigSourceNone {
		return errors.New("no config directory found")
	}

	fmt.Fprintf(a.errOut, "Validating config: %s\n", result.SourcePath)

	issues := config.ValidateDir(result.SourcePath)
	if len(issues) == 0 {
		fmt.Fprintln(a.out, termcolor.Success("Config OK"))
		return nil
	}

	for _, issue := range issues {
		fmt.Fprintf(a.out, "  %s %s\n", termcolor.Error("error:"), issue)
	}
	return findingsErrorf("%d config issue(s) found", len(issues))
}

func (a *app) runValidateConfigRecursive(_ *cobra.Command, args []string) error {
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
		fmt.Fprintf(a.errOut, "Validating config: %s\n", dir)
		issues := config.ValidateDir(dir)
		if len(issues) == 0 {
			fmt.Fprintln(a.out, "  "+termcolor.Success("OK"))
			continue
		}
		totalIssues += len(issues)
		for _, issue := range issues {
			fmt.Fprintf(a.out, "  %s %s\n", termcolor.Error("error:"), issue)
		}
	}

	if totalIssues == 0 {
		return nil
	}
	return findingsErrorf("%d config issue(s) found across %d directory(s)", totalIssues, len(dirs))
}

// findHCLLinterDirs walks root and returns every `.hcl-linter/` directory
// found, sorted by path. Does not descend into `.hcl-linter/` itself or
// other hidden sibling directories (except the .hcl-linter/ match).
func findHCLLinterDirs(root string) []string {
	var dirs []string
	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip unreadable paths, don't abort walk
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if name == ".hcl-linter" {
			dirs = append(dirs, path)
			return filepath.SkipDir
		}
		if path != root && strings.HasPrefix(name, ".") {
			return filepath.SkipDir
		}
		return nil
	}
	_ = filepath.WalkDir(root, walkFn)
	return dirs
}

// warnConfigIssues prints config validation warnings to the diagnostic stream.
// Called at startup for lint/fix so users see config problems even without
// running validate-config explicitly.
func (a *app) warnConfigIssues(configDir string) {
	if configDir == "" {
		return
	}
	issues := config.ValidateDir(configDir)
	for _, issue := range issues {
		fmt.Fprintf(a.errOut, "%s %s\n", termcolor.Warning("Config warning:"), issue)
	}
}

// printIssue renders a single lint issue as `path:line:col: [sev] rule: message`
// so results are grep- and editor-friendly. Issues without a recorded location
// fall back to the result's file path.
func (a *app) printIssue(file string, issue diag.Issue) {
	label := fmt.Sprintf("[%s]", issue.Severity)
	if issue.Severity == diag.SeverityError {
		label = termcolor.Error(label)
	} else {
		label = termcolor.Warning(label)
	}

	location := relPath(file)
	if issue.Location.Filename != "" {
		location = fmt.Sprintf("%s:%d:%d",
			relPath(issue.Location.Filename), issue.Location.Start.Line, issue.Location.Start.Column)
	}
	fmt.Fprintf(a.out, "%s: %s %s: %s\n",
		termcolor.Path(location), label, termcolor.Rule(issue.Rule), issue.Message)
}

// printFixResult renders a single fix result (error or change count) and
// returns the change count contributed to the running total.
func (a *app) printFixResult(result *engine.FixResult) int {
	rel := relPath(result.File)
	if result.Error != nil {
		fmt.Fprintf(a.out, "%s %s: %v\n", termcolor.Error("Error fixing"), rel, result.Error)
		return 0
	}
	if result.Changes > 0 {
		fmt.Fprintf(a.out, "Fixed %s: %d change(s)\n", rel, result.Changes)
		return result.Changes
	}
	return 0
}

// handleFixResults prints either the standard written-file summary or, in
// dry-run mode, a unified diff per file. Returns (totalChanges, wouldChange):
// totalChanges is the number of files reported as changed; wouldChange is true
// iff dry-run found at least one file that differs.
func (a *app) handleFixResults(results []*engine.FixResult, dryRun bool) (totalChanges int, wouldChange bool) {
	for _, result := range results {
		rel := relPath(result.File)

		if result.Error != nil {
			fmt.Fprintf(a.out, "%s %s: %v\n", termcolor.Error("Error fixing"), rel, result.Error)
			continue
		}

		if !dryRun {
			totalChanges += a.printFixResult(result)
			continue
		}

		before, err := os.ReadFile(result.File)
		if err != nil {
			fmt.Fprintf(a.out, "%s %s: %v\n", termcolor.Error("Error reading"), rel, err)
			continue
		}
		if a.printDiff(rel, string(before), result.Content) {
			wouldChange = true
			totalChanges++
		}
	}
	return totalChanges, wouldChange
}

// finalizeFixRun prints the trailing summary (or dry-run error) common to
// runFix and runFormatMode.
func (a *app) finalizeFixRun(totalChanges int, wouldChange, dryRun bool) error {
	if dryRun {
		if wouldChange {
			return findingsErrorf("dry run: %d file(s) would be changed", totalChanges)
		}
		fmt.Fprintln(a.out, termcolor.Success("No changes needed"))
		return nil
	}
	if totalChanges > 0 {
		fmt.Fprintf(a.out, "\nTotal: %d change(s) applied\n", totalChanges)
	} else {
		fmt.Fprintln(a.out, termcolor.Success("No changes needed"))
	}
	return nil
}

func (a *app) findHCLFiles(root string) []string {
	var files []string

	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip unreadable paths, don't abort walk
		}

		if d.IsDir() {
			// Never skip the walk root itself: `lint .` (whose entry is named
			// ".") and explicitly targeted hidden directories must be walked.
			if path != root && !a.includeHidden && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasSuffix(path, ".hcl") || strings.HasSuffix(path, ".tf") {
			files = append(files, path)
		}

		return nil
	}

	_ = filepath.WalkDir(root, walkFn)
	return files
}
