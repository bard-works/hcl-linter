package linter

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/papaya/hcl-linter/internal/ast"
	"github.com/papaya/hcl-linter/internal/config"
)

type Linter struct {
	configLoader *config.Loader
}

func NewLinter(configLoader *config.Loader) *Linter {
	return &Linter{
		configLoader: configLoader,
	}
}

func (l *Linter) LintFile(path string) (*Result, error) {
	cfg, err := l.configLoader.LoadForFile(path)
	if err != nil {
		return nil, err
	}

	parser := ast.NewParser()
	file, diags := parser.ParseFile(path)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse error: %s", diags.Error())
	}

	result := &Result{
		File:   path,
		Issues: []Issue{},
	}

	blocks := ast.GetTopLevelBlocks(file)

	if cfg.BlockOrder != nil && cfg.BlockOrder.Enabled {
		l.checkBlockOrder(result, blocks, cfg.BlockOrder)
	}

	if cfg.ArrayFormat != nil && cfg.ArrayFormat.Enabled {
		l.checkArrayFormat(result, path, file)
	}

	if cfg.NameValidation != nil && cfg.NameValidation.Enabled {
		l.checkNameValidation(result, blocks, cfg.NameValidation)
	}

	if cfg.Duplicates != nil && cfg.Duplicates.Enabled {
		l.checkDuplicates(result, blocks, cfg.Duplicates)
	}

	if cfg.RequiredFields != nil {
		l.checkRequiredFields(result, blocks, cfg.RequiredFields)
	}

	if cfg.RequiredBlocks != nil && len(cfg.RequiredBlocks.Required) > 0 {
		l.checkRequiredBlocks(result, blocks, cfg.RequiredBlocks)
	}

	return result, nil
}

func (l *Linter) checkBlockOrder(result *Result, blocks []ast.BlockInfo, cfg *config.BlockOrderConfig) {
	orderMap := make(map[string]int)
	for i, name := range cfg.Order {
		orderMap[name] = i
	}

	var ordered []string

	for _, block := range blocks {
		if idx, ok := orderMap[block.Type]; ok {
			ordered = append(ordered, fmt.Sprintf("%s[%d]", block.Type, idx))
		}
	}

	expectedOrder := make([]string, 0, len(cfg.Order))
	expectedOrder = append(expectedOrder, cfg.Order...)

	seen := make(map[string]int)
	for _, block := range blocks {
		pos := len(cfg.Order)
		if idx, ok := orderMap[block.Type]; ok {
			pos = idx
		}

		if lastIdx, seenBefore := seen[block.Type]; seenBefore {
			if pos != lastIdx {
				result.Issues = append(result.Issues, Issue{
					Severity: SeverityError,
					Rule:     "block_order",
					Message:  fmt.Sprintf("block type %q appears out of order. Expected: %v", block.Type, expectedOrder),
					Location: block.Block.TypeRange,
				})
			}
		}
		seen[block.Type] = pos
	}

	if len(ordered) > 1 {
		for i := 1; i < len(ordered); i++ {
			curr := ordered[i]
			prev := ordered[i-1]

			currName := strings.Split(curr, "[")[0]
			prevName := strings.Split(prev, "[")[0]

			if orderMap[currName] < orderMap[prevName] {
				result.Issues = append(result.Issues, Issue{
					Severity: SeverityError,
					Rule:     "block_order",
					Message:  fmt.Sprintf("block %q should come before %q", currName, prevName),
				})
			}
		}
	}
}

func (l *Linter) checkArrayFormat(result *Result, _ string, file *hcl.File) {
	attrs, diags := file.Body.JustAttributes()
	if diags.HasErrors() {
		return
	}

	for _, attr := range attrs {
		if expr := attr.Expr; expr != nil {
			l.checkExpressionArrayFormat(result, attr.Name, expr)
		}
	}

	for _, block := range ast.GetTopLevelBlocks(file) {
		blockBody := block.Block.Body
		blockAttrs, _ := blockBody.JustAttributes()
		for name, attr := range blockAttrs {
			l.checkExpressionArrayFormat(result, fmt.Sprintf("%s.%s", block.Type, name), attr.Expr)
		}
	}
}

func (l *Linter) checkExpressionArrayFormat(result *Result, name string, expr hcl.Expression) {
	switch e := expr.(type) {
	case *hclsyntax.TupleConsExpr:
		l.checkTupleConsExpr(result, name, e)
	case *hclsyntax.ObjectConsExpr:
		for _, item := range e.Items {
			l.checkExpressionArrayFormat(result, name, item.ValueExpr)
		}
	}
}

func (l *Linter) checkTupleConsExpr(result *Result, name string, tuple *hclsyntax.TupleConsExpr) {
	for _, item := range tuple.Exprs {
		if !isStringLiteral(item) {
			return
		}
	}

	if len(tuple.Exprs) >= 2 {
		result.Issues = append(result.Issues, Issue{
			Severity:     SeverityError,
			Rule:         "array_format",
			Message:      fmt.Sprintf("array %q should be multiline (2+ items)", name),
			Location:     tuple.Range(),
			SuggestedFix: "multiline array format",
		})
	}
}

func isStringLiteral(expr hcl.Expression) bool {
	switch e := expr.(type) {
	case *hclsyntax.LiteralValueExpr:
		return true
	case *hclsyntax.TemplateExpr:
		return true
	case *hclsyntax.TemplateWrapExpr:
		return isStringLiteral(e.Wrapped)
	}
	return false
}

func (l *Linter) checkNameValidation(result *Result, blocks []ast.BlockInfo, cfg *config.NameValidationConfig) {
	pattern := cfg.Pattern
	if pattern == "" {
		pattern = `^[a-z][a-z0-9_]*$`
	}

	regex := regexp.MustCompile(pattern)

	blockSet := make(map[string]bool)
	for _, b := range cfg.Blocks {
		blockSet[b] = true
	}

	for _, block := range blocks {
		if !blockSet[block.Type] {
			continue
		}

		for _, label := range block.Labels {
			if !regex.MatchString(label) {
				result.Issues = append(result.Issues, Issue{
					Severity: SeverityError,
					Rule:     "name_validation",
					Message:  fmt.Sprintf("%s label %q does not match pattern %q", block.Type, label, pattern),
					Location: block.Block.TypeRange,
				})
			}
		}
	}
}

func (l *Linter) checkDuplicates(result *Result, blocks []ast.BlockInfo, cfg *config.DuplicatesConfig) {
	blockSet := make(map[string]bool)
	for _, b := range cfg.Blocks {
		blockSet[b] = true
	}

	for _, blockType := range cfg.Blocks {
		if !blockSet[blockType] {
			continue
		}

		seen := make(map[string]int)
		for i, block := range blocks {
			if block.Type != blockType {
				continue
			}

			if len(block.Labels) == 0 {
				continue
			}

			label := block.Labels[0]
			if _, exists := seen[label]; exists {
				result.Issues = append(result.Issues, Issue{
					Severity: SeverityError,
					Rule:     "duplicates",
					Message:  fmt.Sprintf("duplicate %s block with label %q", blockType, label),
					Location: blocks[i].Block.TypeRange,
				})
			} else {
				seen[label] = i
			}
		}
	}
}

func (l *Linter) checkRequiredFields(result *Result, blocks []ast.BlockInfo, cfg *config.RequiredFieldsConfig) {
	for _, block := range blocks {
		if block.Type == "include" {
			if cfg.Include != nil && cfg.Include.Expose {
				attrs := ast.GetBlockAttributes(block.Block.Body)
				if _, ok := attrs["expose"]; !ok {
					result.Issues = append(result.Issues, Issue{
						Severity:     SeverityError,
						Rule:         "required_fields",
						Message:      fmt.Sprintf("include block %q missing required field 'expose'", block.Labels),
						Location:     block.Block.TypeRange,
						SuggestedFix: "expose = true",
					})
				}
			}
		}
	}
}

func (l *Linter) checkRequiredBlocks(result *Result, blocks []ast.BlockInfo, cfg *config.RequiredBlocksConfig) {
	blockCounts := make(map[string]int)
	for _, block := range blocks {
		blockCounts[block.Type]++
	}

	for _, req := range cfg.Required {
		count := blockCounts[req.Type]

		if req.Count == "once" && count != 1 {
			result.Issues = append(result.Issues, Issue{
				Severity: SeverityError,
				Rule:     "required_blocks",
				Message:  req.Error,
				Location: hcl.Range{
					Filename: result.File,
					Start:    hcl.Pos{Line: 1, Column: 1},
					End:      hcl.Pos{Line: 1, Column: 1},
				},
			})
		}
	}
}

type LintResult struct {
	Result *Result
	Error  error
}

func (l *Linter) LintFiles(paths []string, maxConcurrency int) []*Result {
	if maxConcurrency <= 0 {
		maxConcurrency = runtime.NumCPU()
	}

	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	results := make(chan *Result, len(paths))

	for _, path := range paths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := l.LintFile(p)
			if err != nil {
				result = &Result{
					File: p,
					Issues: []Issue{{
						Severity: SeverityError,
						Rule:     "linter_error",
						Message:  err.Error(),
					}},
				}
			}
			results <- result
		}(path)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var allResults []*Result
	for r := range results {
		allResults = append(allResults, r)
	}
	return allResults
}
