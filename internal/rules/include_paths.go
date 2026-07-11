package rules

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/diag"
)

// IncludePathsRule validates that `path` attributes on `include` blocks
// resolve to existing files or directories.
type IncludePathsRule struct{}

func (r IncludePathsRule) Name() string  { return "include_paths" }
func (r IncludePathsRule) Priority() int { return PrioritySemantic }

func init() { Register(IncludePathsRule{}) }

func (r IncludePathsRule) Doc() RuleDoc {
	return RuleDoc{
		Summary:     "Validates that path on include blocks resolves to an existing file or directory.",
		Severity:    "error",
		Fixable:     false,
		ConfigBlock: "include_paths",
		ConfigFields: []ConfigField{
			{Name: "enabled", Type: "bool", Required: true, Doc: "Activate the rule"},
		},
		Example: Example{
			Violation: `include "root" { path = "../missing/root.hcl" }`,
		},
	}
}

func (r IncludePathsRule) Enabled(cfg *config.Rules) bool {
	return cfg != nil && cfg.IncludePaths != nil && cfg.IncludePaths.Enabled
}

func (r IncludePathsRule) Check(ctx *Context) []diag.Issue {
	var issues []diag.Issue
	fileDir := filepath.Dir(ctx.FilePath)

	if !ctx.Breaker.Allow() {
		return issues
	}

	for _, block := range ctx.Blocks {
		if block.Type != "include" {
			continue
		}
		attrs := ast.GetBlockAttributes(block.Block.Body)
		path, ok := attrs["path"]
		if !ok {
			continue
		}
		pathStr := hclStringValue(path)
		if pathStr == "" {
			continue
		}
		resolved := resolveRelativePath(fileDir, pathStr)
		if _, err := ctx.Breaker.Stat(resolved); os.IsNotExist(err) {
			issues = append(issues, diag.Issue{
				Severity: diag.SeverityError,
				Rule:     "include_path_exists",
				Message:  fmt.Sprintf("include path %q does not exist", pathStr),
				Location: path.Range(),
			})
		}
	}

	return issues
}
