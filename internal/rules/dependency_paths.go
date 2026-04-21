package rules

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/diag"
)

// DependencyPathsRule validates that `config_path` attributes on `dependency`
// blocks resolve to existing directories.
type DependencyPathsRule struct{}

func (r DependencyPathsRule) Name() string { return "dependency_paths" }

func (r DependencyPathsRule) Enabled(cfg *config.Rules) bool {
	return cfg != nil && cfg.DependencyPaths != nil && cfg.DependencyPaths.Enabled
}

func (r DependencyPathsRule) Check(ctx *Context) []diag.Issue {
	var issues []diag.Issue
	fileDir := filepath.Dir(ctx.FilePath)

	for _, block := range ctx.Blocks {
		if block.Type != "dependency" {
			continue
		}
		attrs := ast.GetBlockAttributes(block.Block.Body)
		configPath, ok := attrs["config_path"]
		if !ok {
			continue
		}
		pathStr := hclStringValue(configPath)
		if pathStr == "" {
			continue
		}
		resolved := resolveRelativePath(fileDir, pathStr)
		if _, err := os.Stat(resolved); os.IsNotExist(err) {
			label := ""
			if len(block.Labels) > 0 {
				label = block.Labels[0]
			}
			issues = append(issues, diag.Issue{
				Severity: diag.SeverityError,
				Rule:     "dependency_path_exists",
				Message:  fmt.Sprintf("dependency %q: config_path %q does not exist", label, pathStr),
				Location: configPath.Range(),
			})
		}
	}

	return issues
}
