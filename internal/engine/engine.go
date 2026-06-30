package engine

import (
	"bytes"
	"errors"
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

func (e *ParseError) Unwrap() error {
	return errors.New(e.Cause)
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

func (e *Engine) buildContext(path string) (*rules.Context, error) {
	cfg, err := e.configLoader.LoadForFile(path)
	if err != nil {
		return nil, err
	}

	preInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	postInfo, err := os.Stat(path)
	if err == nil && !os.SameFile(preInfo, postInfo) {
		return nil, fmt.Errorf("file modified during read: %s", path)
	}

	parser := ast.NewParser()
	file, diags := parser.ParseFile(path)
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

func (e *Engine) LintFiles(paths []string, maxConcurrency int) []*diag.Result {
	if maxConcurrency <= 0 {
		maxConcurrency = runtime.NumCPU()
	}

	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	ch := make(chan *diag.Result, len(paths))

	for _, path := range paths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := e.LintFile(p)
			if err != nil {
				result = &diag.Result{
					File: p,
					Issues: []diag.Issue{{
						Severity: diag.SeverityError,
						Rule:     "linter_error",
						Message:  err.Error(),
					}},
				}
			}
			ch <- result
		}(path)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	var allResults []*diag.Result
	for r := range ch {
		allResults = append(allResults, r)
	}
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

func (e *Engine) FixFiles(paths []string, maxConcurrency int) []*FixResult {
	if maxConcurrency <= 0 {
		maxConcurrency = runtime.NumCPU()
	}

	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	results := make(chan *FixResult, len(paths))

	for _, path := range paths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := e.FixFile(p)
			if err != nil {
				result = &FixResult{File: p, Error: err}
			}
			results <- result
		}(path)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var allResults []*FixResult
	for r := range results {
		allResults = append(allResults, r)
	}
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
	preInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	postInfo, err := os.Stat(path)
	if err == nil && !os.SameFile(preInfo, postInfo) {
		return nil, fmt.Errorf("file modified during read: %s", path)
	}

	parser := ast.NewParser()
	file, diags := parser.ParseFile(path)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse error: %s", diags.Error())
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

func (e *Engine) FormatFixFiles(paths []string, maxConcurrency int) []*FixResult {
	if maxConcurrency <= 0 {
		maxConcurrency = runtime.NumCPU()
	}

	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	results := make(chan *FixResult, len(paths))

	for _, path := range paths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := e.FormatFixFile(p)
			if err != nil {
				result = &FixResult{File: p, Error: err}
			}
			results <- result
		}(path)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var allResults []*FixResult
	for r := range results {
		allResults = append(allResults, r)
	}
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].File < allResults[j].File
	})
	return allResults
}
