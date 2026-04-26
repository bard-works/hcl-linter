package linter

import (
	"runtime"
	"sync"

	"github.com/bard-works/hcl-linter/internal/config"
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
	_, err := l.configLoader.LoadForFile(path)
	if err != nil {
		return nil, err
	}

	return &Result{
		File:   path,
		Issues: []Issue{},
	}, nil
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
