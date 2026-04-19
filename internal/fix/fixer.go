package fix

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"sync"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/rules"
)

type Fixer struct {
	configLoader *config.Loader
}

func NewFixer(configLoader *config.Loader) *Fixer {
	return &Fixer{
		configLoader: configLoader,
	}
}

type FixResult struct {
	File    string
	Changes int
	Content string
	Success bool
	Error   error
}

func (f *Fixer) FixFile(path string) (*FixResult, error) {
	cfg, err := f.configLoader.LoadForFile(path)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	result := &FixResult{
		File:    path,
		Changes: 0,
	}

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
		newContent, changes := fixBlankLinesWithinBlocks(contentStr, blocks, attrs)
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

func (f *Fixer) PreviewFix(path string) (string, error) {
	cfg, err := f.configLoader.LoadForFile(path)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	result := string(content)

	if cfg.BlockOrder != nil && cfg.BlockOrder.Enabled {
		parser := ast.NewParser()
		file, diags := parser.ParseFile(path)
		if !diags.HasErrors() {
			blocks := ast.GetTopLevelBlocks(file)
			result = rules.FixBlockOrder(result, blocks, cfg.BlockOrder)
		}
	}

	if cfg.NameValidation != nil && cfg.NameValidation.Enabled {
		parser := ast.NewParser()
		if f, diags := parser.ParseContent([]byte(result), path); !diags.HasErrors() {
			blocks := ast.GetTopLevelBlocks(f)
			result, _ = rules.FixNameValidation(result, blocks, cfg.NameValidation)
		}
	}

	if cfg.ArrayFormat != nil && cfg.ArrayFormat.Enabled {
		result, _ = rules.FixArrays(result, cfg.ArrayFormat.Sort)
	}

	return result, nil
}

func (f *Fixer) FixFiles(paths []string, maxConcurrency int) []*FixResult {
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

			result, err := f.FixFile(p)
			if err != nil {
				result = &FixResult{
					File:  p,
					Error: err,
				}
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
	return allResults
}

var defaultFormatBlockOrder = &config.BlockOrderConfig{
	Enabled: true,
	Order:   []string{"include", "locals", "terraform", "dependency", "inputs"},
}

func (f *Fixer) FormatFixFile(path string) (*FixResult, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	result := &FixResult{
		File:    path,
		Changes: 0,
	}

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
	newContent3, changes2 := fixBlankLinesWithinBlocks(contentStr, blocks, attrs)
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

func (f *Fixer) FormatFixFiles(paths []string, maxConcurrency int) []*FixResult {
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

			result, err := f.FormatFixFile(p)
			if err != nil {
				result = &FixResult{
					File:  p,
					Error: err,
				}
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
	return allResults
}

var _ = hclsyntax.TupleConsExpr{}
