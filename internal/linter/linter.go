package linter

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/hashicorp/hcl/v2"
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

	if cfg.Terragrunt != nil && cfg.Terragrunt.Enabled {
		checkTerragrunt(result, path, file, cfg.Terragrunt)
	}

	if cfg.TerragruntFunctions != nil && cfg.TerragruntFunctions.Enabled {
		checkTerragruntFunctions(result, path, file, cfg.TerragruntFunctions)
	}

	if cfg.TerraformBlock != nil && cfg.TerraformBlock.Enabled {
		checkTerraformBlock(result, file, cfg.TerraformBlock)
	}

	if cfg.KeyValue != nil && cfg.KeyValue.Enabled {
		checkKeyValueImpl(result, blocks, cfg.KeyValue)
	}

	if cfg.CountForEach != nil && cfg.CountForEach.Enabled {
		checkCountForEachImpl(result, blocks, cfg.CountForEach)
	}

	if cfg.DependencyOutputs != nil && cfg.DependencyOutputs.Enabled {
		checkDependencyOutputsImpl(result, blocks, path, cfg.DependencyOutputs, nil)
	}

	return result, nil
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

func checkTerragrunt(result *Result, filePath string, file *hcl.File, cfg *config.TerragruntConfig) {
	checkTerragruntImpl(result, filePath, file, cfg)
}

func checkTerragruntFunctions(result *Result, filePath string, file *hcl.File, cfg *config.TerragruntFunctionsConfig) {
	checkTerragruntFunctionsImpl(result, filePath, file, cfg)
}

func checkTerraformBlock(result *Result, file *hcl.File, cfg *config.TerraformBlockConfig) {
	checkTerraformBlockImpl(result, file, cfg)
}
