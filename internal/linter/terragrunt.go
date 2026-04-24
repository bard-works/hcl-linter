package linter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func checkTerragruntImpl(result *Result, filePath string, file *hcl.File, cfg *config.TerragruntConfig) {
	blocks := ast.GetTopLevelBlocks(file)

	if cfg.DependencyPathExists {
		checkDependencyPathsImpl(result, filePath, blocks)
	}
	if cfg.IncludePathExists {
		checkIncludePathsImpl(result, filePath, blocks)
	}
	if cfg.RemoteStateConfig {
		checkRemoteStateConfigImpl(result, blocks)
	}
}

func checkDependencyPathsImpl(result *Result, filePath string, blocks []ast.BlockInfo) {
	fileDir := filepath.Dir(filePath)

	for _, block := range blocks {
		if block.Type != "dependency" {
			continue
		}

		attrs := ast.GetBlockAttributes(block.Block.Body)
		if configPath, ok := attrs["config_path"]; ok {
			if pathStr := getStringValue(configPath); pathStr != "" {
				resolvedPath := resolvePath(fileDir, pathStr)
				if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
					result.Issues = append(result.Issues, Issue{
						Severity: SeverityError,
						Rule:     "dependency_path_exists",
						Message:  fmt.Sprintf("dependency %q: config_path %q does not exist", block.Labels[0], pathStr),
						Location: configPath.Range(),
					})
				}
			}
		}
	}
}

func checkIncludePathsImpl(result *Result, filePath string, blocks []ast.BlockInfo) {
	fileDir := filepath.Dir(filePath)

	for _, block := range blocks {
		if block.Type != "include" {
			continue
		}

		attrs := ast.GetBlockAttributes(block.Block.Body)
		if path, ok := attrs["path"]; ok {
			if pathStr := getStringValue(path); pathStr != "" {
				resolvedPath := resolvePath(fileDir, pathStr)
				if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
					result.Issues = append(result.Issues, Issue{
						Severity: SeverityError,
						Rule:     "include_path_exists",
						Message:  fmt.Sprintf("include path %q does not exist", pathStr),
						Location: path.Range(),
					})
				}
			}
		}
	}
}

func checkRemoteStateConfigImpl(result *Result, blocks []ast.BlockInfo) {
	for _, block := range blocks {
		if block.Type != "terraform" {
			continue
		}

		nestedBlocks := block.Block.Body.Blocks
		var hasRemoteState bool
		var remoteStateBlock *hclsyntax.Block

		for _, nested := range nestedBlocks {
			if nested.Type == "remote_state" {
				hasRemoteState = true
				remoteStateBlock = nested
				break
			}
		}

		if !hasRemoteState {
			continue
		}

		attrs := ast.GetBlockAttributes(remoteStateBlock.Body)
		if backend, ok := attrs["backend"]; ok {
			backendStr := getStringValue(backend)
			if backendStr == "" {
				result.Issues = append(result.Issues, Issue{
					Severity: SeverityError,
					Rule:     "remote_state_config",
					Message:  "remote_state block missing required 'backend' attribute",
					Location: remoteStateBlock.TypeRange,
				})
			}
		} else {
			result.Issues = append(result.Issues, Issue{
				Severity: SeverityError,
				Rule:     "remote_state_config",
				Message:  "remote_state block missing required 'backend' attribute",
				Location: remoteStateBlock.TypeRange,
			})
		}
	}
}

func getStringValue(expr hcl.Expression) string {
	val, diags := expr.Value(nil)
	if diags.HasErrors() {
		return ""
	}
	return val.AsString()
}

func resolvePath(baseDir, inputPath string) string {
	if filepath.IsAbs(inputPath) {
		return inputPath
	}
	return filepath.Join(baseDir, inputPath)
}

func checkTerragruntFunctionsImpl(result *Result, filePath string, file *hcl.File, cfg *config.TerragruntFunctionsConfig) {
	fileDir := filepath.Dir(filePath)

	if cfg.FindInParentFoldersExists || cfg.GetEnvHasDefault {
		walkAndCheckFunctions(result, fileDir, file, cfg)
	}
}

func walkAndCheckFunctions(result *Result, fileDir string, file *hcl.File, cfg *config.TerragruntFunctionsConfig) {
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return
	}

	_ = hclsyntax.Walk(body, &functionCheckWalker{
		result:  result,
		fileDir: fileDir,
		cfg:     cfg,
	})
}

type functionCheckWalker struct {
	result  *Result
	fileDir string
	cfg     *config.TerragruntFunctionsConfig
}

func (w *functionCheckWalker) Enter(node hclsyntax.Node) hcl.Diagnostics {
	funcCall, ok := node.(*hclsyntax.FunctionCallExpr)
	if !ok {
		return nil
	}

	switch funcCall.Name {
	case "find_in_parent_folders":
		if w.cfg.FindInParentFoldersExists {
			checkFindInParentFolders(w.result, w.fileDir, funcCall)
		}
	case "get_env":
		if w.cfg.GetEnvHasDefault {
			checkGetEnvHasDefault(w.result, funcCall)
		}
	}

	return nil
}

func (w *functionCheckWalker) Exit(_ hclsyntax.Node) hcl.Diagnostics {
	return nil
}

func checkFindInParentFolders(result *Result, fileDir string, funcCall *hclsyntax.FunctionCallExpr) {
	if len(funcCall.Args) == 0 {
		defaultFile := "terragrunt.hcl"
		path := findInParent(fileDir, defaultFile)
		if path == "" {
			result.Issues = append(result.Issues, Issue{
				Severity: SeverityError,
				Rule:     "find_in_parent_folders_exists",
				Message:  "find_in_parent_folders() could not find terragrunt.hcl in parent directories",
				Location: funcCall.Range(),
			})
		}
		return
	}

	firstArg := funcCall.Args[0]
	filename := getStringValue(firstArg)
	if filename == "" {
		return
	}

	path := findInParent(fileDir, filename)
	if path == "" {
		result.Issues = append(result.Issues, Issue{
			Severity: SeverityError,
			Rule:     "find_in_parent_folders_exists",
			Message:  fmt.Sprintf("find_in_parent_folders(%q) could not find file in parent directories", filename),
			Location: funcCall.Range(),
		})
	}
}

func checkGetEnvHasDefault(result *Result, funcCall *hclsyntax.FunctionCallExpr) {
	if len(funcCall.Args) < 2 {
		result.Issues = append(result.Issues, Issue{
			Severity: SeverityWarning,
			Rule:     "get_env_has_default",
			Message:  "get_env() should have a default value as second argument",
			Location: funcCall.Range(),
		})
	}
}

func findInParent(dir, filename string) string {
	current := dir
	for {
		testPath := filepath.Join(current, filename)
		if _, err := os.Stat(testPath); err == nil {
			return testPath
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return ""
}
