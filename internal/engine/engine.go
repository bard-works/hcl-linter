package engine

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"sort"
	"sync"

	"github.com/hashicorp/hcl/v2/hclwrite"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/linter"
	"github.com/bard-works/hcl-linter/internal/rules"
)

// Engine is the single entry point for lint and fix operations.
type Engine struct {
	configLoader *config.Loader
	registry     *rules.Registry
}

type FixResult struct {
	File    string
	Changes int
	Content string
	Success bool
	Error   error
}

func New(loader *config.Loader) *Engine {
	reg := &rules.Registry{}
	reg.Register(rules.BlockOrderRule{})
	reg.Register(rules.ArrayFormatRule{})
	reg.Register(rules.NameValidationRule{})
	reg.Register(rules.DuplicatesRule{})
	reg.Register(rules.RequiredFieldsRule{})
	reg.Register(rules.RequiredBlocksRule{})
	reg.Register(rules.BlankLinesRule{})
	reg.Register(rules.DependencyPathsRule{})
	reg.Register(rules.IncludePathsRule{})
	reg.Register(rules.RemoteStateRule{})
	reg.Register(rules.HCLFunctionsRule{})
	reg.Register(rules.TerraformBlockRule{})
	reg.Register(rules.KeyValueRule{})
	reg.Register(rules.CountForEachRule{})
	reg.Register(rules.DependencyOutputsRule{})

	return &Engine{
		configLoader: loader,
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
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].File < allResults[j].File
	})
	return allResults
}

// --- Fix ---

func (e *Engine) FixFile(path string) (*FixResult, error) {
	cfg, err := e.configLoader.LoadForFile(path)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	result := &FixResult{File: path}

	parser := ast.NewParser()
	file, diags := parser.ParseFile(path)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse error: %s", diags.Error())
	}

	blocks := ast.GetTopLevelBlocks(file)
	contentStr := string(content)

	if cfg.BlockOrder != nil && cfg.BlockOrder.Enabled {
		newContent := rules.FixBlockOrder(contentStr, blocks, cfg.BlockOrder)
		if newContent != contentStr {
			contentStr = newContent
			result.Changes++
			parser = ast.NewParser()
			file, _ = parser.ParseContent([]byte(contentStr), path)
			blocks = ast.GetTopLevelBlocks(file)
		}
	}

	if cfg.NameValidation != nil && cfg.NameValidation.Enabled {
		newContent, changed := rules.FixNameValidation(contentStr, blocks, cfg.NameValidation)
		if changed {
			contentStr = newContent
			result.Changes++
			parser = ast.NewParser()
			file, _ = parser.ParseContent([]byte(contentStr), path)
			blocks = ast.GetTopLevelBlocks(file)
		}
	}

	if cfg.RequiredFields != nil {
		newContent, changed := rules.FixRequiredFields(contentStr, blocks, cfg.RequiredFields)
		if changed {
			contentStr = newContent
			result.Changes++
			parser = ast.NewParser()
			_, _ = parser.ParseContent([]byte(contentStr), path)
		}
	}

	if cfg.ArrayFormat != nil && cfg.ArrayFormat.Enabled {
		newContent, changes := rules.FixArrays(contentStr, cfg.ArrayFormat.Sort)
		if changes > 0 {
			contentStr = newContent
			result.Changes += changes
			parser = ast.NewParser()
			file, _ = parser.ParseContent([]byte(contentStr), path)
			blocks = ast.GetTopLevelBlocks(file)
		}
	}

	if cfg.BlankLines != nil && cfg.BlankLines.Enabled && cfg.BlankLines.WithinBlocks {
		attrs := ast.GetTopLevelAttributes(file)
		newContent, changes := rules.FixBlankLines(contentStr, blocks, attrs)
		if changes > 0 {
			contentStr = newContent
			result.Changes += changes
		}
	}

	if result.Changes > 0 {
		if err := os.WriteFile(path, []byte(contentStr), 0o644); err != nil {
			return nil, err
		}
	}

	result.Content = contentStr
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

func (e *Engine) FormatFixFile(path string) (*FixResult, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	result := &FixResult{File: path}

	parser := ast.NewParser()
	file, diags := parser.ParseFile(path)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse error: %s", diags.Error())
	}

	blocks := ast.GetTopLevelBlocks(file)
	contentStr := string(content)

	newContent := rules.FixBlockOrder(contentStr, blocks, defaultFormatBlockOrder)
	if newContent != contentStr {
		contentStr = newContent
		result.Changes++
		parser = ast.NewParser()
		file, _ = parser.ParseContent([]byte(contentStr), path)
		blocks = ast.GetTopLevelBlocks(file)
	}

	newContent2, changes := rules.FixArrays(contentStr, true) // --format always sorts
	if changes > 0 {
		contentStr = newContent2
		result.Changes += changes
		parser = ast.NewParser()
		file, _ = parser.ParseContent([]byte(contentStr), path)
		blocks = ast.GetTopLevelBlocks(file)
	}

	attrs := ast.GetTopLevelAttributes(file)
	newContent3, changes2 := rules.FixBlankLines(contentStr, blocks, attrs)
	if changes2 > 0 {
		contentStr = newContent3
		result.Changes += changes2
	}

	// Apply hclwrite.Format for final whitespace cleanup
	formattedBytes := hclwrite.Format([]byte(contentStr))
	if !bytes.Equal(formattedBytes, []byte(contentStr)) {
		contentStr = string(formattedBytes)
		result.Changes++
	}

	if result.Changes > 0 {
		if err := os.WriteFile(path, []byte(contentStr), 0o644); err != nil {
			return nil, err
		}
	}

	result.Content = contentStr
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
