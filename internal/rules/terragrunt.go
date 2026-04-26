package rules

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/linter"
)

type TerragruntRule struct{}

func (r TerragruntRule) Name() string { return "terragrunt" }

func (r TerragruntRule) Enabled(cfg *config.Rules) bool {
	return cfg != nil && cfg.Terragrunt != nil && cfg.Terragrunt.Enabled
}

func (r TerragruntRule) Check(ctx *Context) []linter.Issue {
	cfg := ctx.Config.Terragrunt
	var issues []linter.Issue

	if cfg.DependencyPathExists {
		checkTerragruntDependencyPaths(&issues, ctx.FilePath, ctx.Blocks)
	}
	if cfg.IncludePathExists {
		checkTerragruntIncludePaths(&issues, ctx.FilePath, ctx.Blocks)
	}
	if cfg.RemoteStateConfig {
		checkTerragruntRemoteStateConfig(&issues, ctx.File)
	}

	return issues
}

func checkTerragruntDependencyPaths(issues *[]linter.Issue, filePath string, blocks []ast.BlockInfo) {
	fileDir := filepath.Dir(filePath)

	for _, block := range blocks {
		if block.Type != "dependency" {
			continue
		}

		attrs := ast.GetBlockAttributes(block.Block.Body)
		if configPath, ok := attrs["config_path"]; ok {
			if pathStr := hclStringValue(configPath); pathStr != "" {
				resolvedPath := terragruntResolvePath(fileDir, pathStr)
				if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
					*issues = append(*issues, linter.Issue{
						Severity: linter.SeverityError,
						Rule:     "dependency_path_exists",
						Message:  fmt.Sprintf("dependency %q: config_path %q does not exist", block.Labels[0], pathStr),
						Location: configPath.Range(),
					})
				}
			}
		}
	}
}

func checkTerragruntIncludePaths(issues *[]linter.Issue, filePath string, blocks []ast.BlockInfo) {
	fileDir := filepath.Dir(filePath)

	for _, block := range blocks {
		if block.Type != "include" {
			continue
		}

		attrs := ast.GetBlockAttributes(block.Block.Body)
		if path, ok := attrs["path"]; ok {
			if pathStr := hclStringValue(path); pathStr != "" {
				resolvedPath := terragruntResolvePath(fileDir, pathStr)
				if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
					*issues = append(*issues, linter.Issue{
						Severity: linter.SeverityError,
						Rule:     "include_path_exists",
						Message:  fmt.Sprintf("include path %q does not exist", pathStr),
						Location: path.Range(),
					})
				}
			}
		}
	}
}

func checkTerragruntRemoteStateConfig(issues *[]linter.Issue, file *hcl.File) {
	syntaxBody, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return
	}

	for _, block := range syntaxBody.Blocks {
		if block.Type != "terraform" {
			continue
		}

		var remoteStateBlock *hclsyntax.Block
		for _, nested := range block.Body.Blocks {
			if nested.Type == "remote_state" {
				remoteStateBlock = nested
				break
			}
		}

		if remoteStateBlock == nil {
			continue
		}

		attrs := ast.GetBlockAttributes(remoteStateBlock.Body)
		backend, ok := attrs["backend"]
		if !ok || hclStringValue(backend) == "" {
			*issues = append(*issues, linter.Issue{
				Severity: linter.SeverityError,
				Rule:     "remote_state_config",
				Message:  "remote_state block missing required 'backend' attribute",
				Location: remoteStateBlock.TypeRange,
			})
		}
	}
}

func hclStringValue(expr hcl.Expression) string {
	val, diags := expr.Value(nil)
	if diags.HasErrors() {
		return ""
	}
	return val.AsString()
}

func terragruntResolvePath(baseDir, inputPath string) string {
	if filepath.IsAbs(inputPath) {
		return inputPath
	}
	return filepath.Join(baseDir, inputPath)
}
