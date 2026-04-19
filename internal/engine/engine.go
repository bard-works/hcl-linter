package engine

import (
	"fmt"
	"os"
	"runtime"
	"sync"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/fix"
	"github.com/bard-works/hcl-linter/internal/linter"
	"github.com/bard-works/hcl-linter/internal/rules"
)

// Engine is the single entry point for lint and fix operations.
type Engine struct {
	configLoader *config.Loader
	linter       *linter.Linter
	fixer        *fix.Fixer
	registry     *rules.Registry
}

func New(loader *config.Loader) *Engine {
	reg := &rules.Registry{}
	reg.Register(rules.BlockOrderRule{})
	reg.Register(rules.ArrayFormatRule{})
	reg.Register(rules.NameValidationRule{})
	reg.Register(rules.DuplicatesRule{})
	reg.Register(rules.RequiredFieldsRule{})
	reg.Register(rules.RequiredBlocksRule{})

	return &Engine{
		configLoader: loader,
		linter:       linter.NewLinter(loader),
		fixer:        fix.NewFixer(loader),
		registry:     reg,
	}
}

func (e *Engine) buildContext(path string) (*rules.Context, error) {
	cfg, err := e.configLoader.LoadForFile(path)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	parser := ast.NewParser()
	file, diags := parser.ParseFile(path)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse error: %s", diags.Error())
	}

	return &rules.Context{
		FilePath: path,
		Content:  content,
		File:     file,
		Blocks:   ast.GetTopLevelBlocks(file),
		Attrs:    ast.GetTopLevelAttributes(file),
		Config:   cfg,
	}, nil
}

// --- Lint ---

func (e *Engine) LintFile(path string) (*linter.Result, error) {
	ctx, err := e.buildContext(path)
	if err != nil {
		return nil, err
	}

	result := &linter.Result{File: path, Issues: []linter.Issue{}}

	for _, rule := range e.registry.Enabled(ctx.Config) {
		result.Issues = append(result.Issues, rule.Check(ctx)...)
	}

	// Delegate remaining (unmigrated) rules to the old linter.
	oldResult, err := e.linter.LintFile(path)
	if err != nil {
		return nil, err
	}
	result.Issues = append(result.Issues, oldResult.Issues...)

	return result, nil
}

func (e *Engine) LintFiles(paths []string, maxConcurrency int) []*linter.Result {
	if maxConcurrency <= 0 {
		maxConcurrency = runtime.NumCPU()
	}

	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	ch := make(chan *linter.Result, len(paths))

	for _, path := range paths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := e.LintFile(p)
			if err != nil {
				result = &linter.Result{
					File: p,
					Issues: []linter.Issue{{
						Severity: linter.SeverityError,
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

	var allResults []*linter.Result
	for r := range ch {
		allResults = append(allResults, r)
	}
	return allResults
}

// --- Fix ---

func (e *Engine) FixFile(path string) (*fix.FixResult, error) {
	return e.fixer.FixFile(path)
}

func (e *Engine) FixFiles(paths []string, maxConcurrency int) []*fix.FixResult {
	return e.fixer.FixFiles(paths, maxConcurrency)
}

// --- Format fix (opinionated defaults, no config required) ---

func (e *Engine) FormatFixFile(path string) (*fix.FixResult, error) {
	return e.fixer.FormatFixFile(path)
}

func (e *Engine) FormatFixFiles(paths []string, maxConcurrency int) []*fix.FixResult {
	return e.fixer.FormatFixFiles(paths, maxConcurrency)
}
