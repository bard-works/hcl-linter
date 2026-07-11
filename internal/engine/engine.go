package engine

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
	"sync"

	"github.com/hashicorp/hcl/v2/hclwrite"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/diag"
	"github.com/bard-works/hcl-linter/internal/fsutil"
	"github.com/bard-works/hcl-linter/internal/rules"
)

// Engine is the single entry point for lint and fix operations.
type Engine struct {
	configLoader *config.Loader
	registry     *rules.Registry

	// breaker is shared across every file processed by this engine instance, so
	// repeated filesystem failures in the dependency-resolving rules trip the
	// breaker once and fail fast for the remainder of the run.
	breaker *rules.CircuitBreaker

	// DryRun, when true, skips writing fix results to disk. FixResult.Content
	// still carries the proposed bytes so callers can diff them against the
	// original file.
	DryRun bool
}

type FixResult struct {
	File    string
	Changes int
	Content string
	Success bool
	Error   error
}

type EngineOption func(*Engine)

func WithRegistry(r *rules.Registry) EngineOption {
	return func(e *Engine) { e.registry = r }
}

type ParseError struct {
	File  string
	Cause string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error in %s: %s", e.File, e.Cause)
}

func New(loader *config.Loader, opts ...EngineOption) *Engine {
	e := &Engine{
		configLoader: loader,
		registry:     rules.DefaultRegistry(),
		breaker:      rules.NewCircuitBreaker(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// readFileStable errors if a Stat taken before and after the read shows the
// file was replaced or modified in between.
func readFileStable(path string) ([]byte, error) {
	preInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	postInfo, err := os.Stat(path)
	if err == nil && fileChangedBetweenStats(preInfo, postInfo) {
		return nil, fmt.Errorf("file modified during read: %s", path)
	}
	return content, nil
}

// fileChangedBetweenStats catches inode replacement (SameFile) and in-place
// writes that reuse the same inode (mtime/size).
func fileChangedBetweenStats(pre, post os.FileInfo) bool {
	return !os.SameFile(pre, post) || pre.ModTime() != post.ModTime() || pre.Size() != post.Size()
}

func (e *Engine) buildContext(path string) (*rules.Context, error) {
	cfg, err := e.configLoader.LoadForFile(path)
	if err != nil {
		return nil, err
	}

	content, err := readFileStable(path)
	if err != nil {
		return nil, err
	}

	// Parse the bytes already read; ParseFile would re-read from disk.
	parser := ast.NewParser()
	file, diags := parser.ParseContent(content, path)
	if diags.HasErrors() {
		return nil, &ParseError{File: path, Cause: diags.Error()}
	}

	return &rules.Context{
		FilePath: path,
		Content:  content,
		File:     file,
		Blocks:   ast.GetTopLevelBlocks(file),
		Attrs:    ast.GetTopLevelAttributes(file),
		Config:   cfg,
		Breaker:  e.breaker,
	}, nil
}

// --- Lint ---

func (e *Engine) LintFile(path string) (*diag.Result, error) {
	ctx, err := e.buildContext(path)
	if err != nil {
		return nil, err
	}

	result := &diag.Result{File: path, Issues: []diag.Issue{}}

	for _, rule := range e.registry.Enabled(ctx.Config) {
		result.Issues = append(result.Issues, rule.Check(ctx)...)
	}

	return result, nil
}

// runConcurrent applies fn to every path with at most maxConcurrency workers.
// When ctx is cancelled no new work is dispatched; results for paths never
// processed are omitted. In-flight files always run to completion so a fix is
// never abandoned halfway.
func runConcurrent[R any](ctx context.Context, paths []string, maxConcurrency int, fn func(string) R) []R {
	if maxConcurrency <= 0 {
		maxConcurrency = runtime.NumCPU()
	}

	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	ch := make(chan R, len(paths))

	for _, path := range paths {
		if ctx.Err() != nil {
			break
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			defer func() { <-sem }()
			ch <- fn(p)
		}(path)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	var out []R
	for r := range ch {
		out = append(out, r)
	}
	return out
}

func (e *Engine) LintFiles(ctx context.Context, paths []string, maxConcurrency int) []*diag.Result {
	allResults := runConcurrent(ctx, paths, maxConcurrency, func(p string) (result *diag.Result) {
		defer func() {
			if r := recover(); r != nil {
				result = &diag.Result{File: p, Issues: []diag.Issue{{
					Severity: diag.SeverityError,
					Rule:     diag.RuleLinterError,
					Message:  fmt.Sprintf("internal error (recovered panic): %v", r),
				}}}
			}
		}()
		var err error
		result, err = e.LintFile(p)
		if err != nil {
			result = &diag.Result{
				File: p,
				Issues: []diag.Issue{{
					Severity: diag.SeverityError,
					Rule:     diag.RuleLinterError,
					Message:  err.Error(),
				}},
			}
		}
		return result
	})
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].File < allResults[j].File
	})
	return allResults
}

// --- Fix ---

// runFixPipeline runs all enabled Fixer rules in priority order, refreshing the
// parsed AST in ctx after each rule that makes changes.
func (e *Engine) runFixPipeline(ctx *rules.Context) (int, error) {
	total := 0
	for _, rule := range e.registry.Sorted() {
		if !rule.Enabled(ctx.Config) {
			continue
		}
		fixer, ok := rule.(rules.Fixer)
		if !ok {
			continue
		}
		n, err := fixer.Fix(ctx)
		if err != nil {
			return total, err
		}
		if n > 0 {
			total += n
			if err := e.refreshContext(ctx); err != nil {
				return total, err
			}
		}
	}
	return total, nil
}

func (e *Engine) refreshContext(ctx *rules.Context) error {
	parser := ast.NewParser()
	file, diags := parser.ParseContent(ctx.Content, ctx.FilePath)
	if diags.HasErrors() {
		return &ParseError{File: ctx.FilePath, Cause: diags.Error()}
	}
	ctx.File = file
	ctx.Blocks = ast.GetTopLevelBlocks(file)
	ctx.Attrs = ast.GetTopLevelAttributes(file)
	return nil
}

func (e *Engine) FixFile(path string) (*FixResult, error) {
	ctx, err := e.buildContext(path)
	if err != nil {
		return nil, err
	}

	changes, err := e.runFixPipeline(ctx)
	if err != nil {
		return nil, err
	}

	result := &FixResult{File: path, Changes: changes}
	if changes > 0 && !e.DryRun {
		if err := fsutil.WriteFileSafe(path, ctx.Content, 0o644); err != nil {
			return nil, err
		}
	}
	result.Content = string(ctx.Content)
	result.Success = true
	return result, nil
}

func (e *Engine) FixFiles(ctx context.Context, paths []string, maxConcurrency int) []*FixResult {
	return e.fixConcurrent(ctx, paths, maxConcurrency, e.FixFile)
}

// fixConcurrent runs one of the per-file fix functions over paths and returns
// the results sorted by file path.
func (e *Engine) fixConcurrent(
	ctx context.Context,
	paths []string,
	maxConcurrency int,
	fixFn func(string) (*FixResult, error),
) []*FixResult {
	allResults := runConcurrent(ctx, paths, maxConcurrency, func(p string) (result *FixResult) {
		defer func() {
			if r := recover(); r != nil {
				result = &FixResult{File: p, Error: fmt.Errorf("internal error (recovered panic): %v", r)}
			}
		}()
		var err error
		result, err = fixFn(p)
		if err != nil {
			result = &FixResult{File: p, Error: err}
		}
		return result
	})
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].File < allResults[j].File
	})
	return allResults
}

// --- Format fix (opinionated defaults, no config required) ---

var defaultFormatBlockOrder = &config.BlockOrderConfig{
	Enabled: true,
	Order:   []string{"include", "locals", "terraform", "dependency", "inputs"},
}

func defaultFormatConfig() *config.Rules {
	return &config.Rules{
		BlockOrder:  defaultFormatBlockOrder,
		ArrayFormat: &config.ArrayFormatConfig{Enabled: true, Sort: true},
		BlankLines:  &config.BlankLinesConfig{Enabled: true, WithinBlocks: true},
	}
}

func (e *Engine) FormatFixFile(path string) (*FixResult, error) {
	content, err := readFileStable(path)
	if err != nil {
		return nil, err
	}

	parser := ast.NewParser()
	file, diags := parser.ParseContent(content, path)
	if diags.HasErrors() {
		return nil, &ParseError{File: path, Cause: diags.Error()}
	}

	ctx := &rules.Context{
		FilePath: path,
		Content:  content,
		File:     file,
		Blocks:   ast.GetTopLevelBlocks(file),
		Attrs:    ast.GetTopLevelAttributes(file),
		Config:   defaultFormatConfig(),
		Breaker:  e.breaker,
	}

	changes, err := e.runFixPipeline(ctx)
	if err != nil {
		return nil, err
	}

	// Apply hclwrite.Format for final whitespace cleanup
	formattedBytes := hclwrite.Format(ctx.Content)
	if !bytes.Equal(formattedBytes, ctx.Content) {
		ctx.Content = formattedBytes
		changes++
	}

	result := &FixResult{File: path, Changes: changes}
	if changes > 0 && !e.DryRun {
		if err := fsutil.WriteFileSafe(path, ctx.Content, 0o644); err != nil {
			return nil, err
		}
	}
	result.Content = string(ctx.Content)
	result.Success = true
	return result, nil
}

func (e *Engine) FormatFixFiles(ctx context.Context, paths []string, maxConcurrency int) []*FixResult {
	return e.fixConcurrent(ctx, paths, maxConcurrency, e.FormatFixFile)
}
