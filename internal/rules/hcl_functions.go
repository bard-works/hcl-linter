package rules

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/diag"
)

// HCLFunctionsRule validates HCL function calls. Currently recognizes
// Terragrunt's `find_in_parent_folders` and `get_env`.
type HCLFunctionsRule struct{}

func (r HCLFunctionsRule) Name() string  { return "hcl_functions" }
func (r HCLFunctionsRule) Priority() int { return PrioritySemantic }

func init() { Register(HCLFunctionsRule{}) }

func (r HCLFunctionsRule) Enabled(cfg *config.Rules) bool {
	return cfg != nil && cfg.HCLFunctions != nil && cfg.HCLFunctions.Enabled
}

func (r HCLFunctionsRule) Check(ctx *Context) []diag.Issue {
	cfg := ctx.Config.HCLFunctions
	if !cfg.FindInParentFoldersExists && !cfg.GetEnvHasDefault {
		return nil
	}

	var issues []diag.Issue
	fileDir := filepath.Dir(ctx.FilePath)

	body, ok := ctx.File.Body.(*hclsyntax.Body)
	if !ok {
		return nil
	}

	_ = hclsyntax.Walk(body, &functionCallWalker{
		issues:  &issues,
		fileDir: fileDir,
		cfg:     cfg,
	})

	return issues
}

type functionCallWalker struct {
	issues  *[]diag.Issue
	fileDir string
	cfg     *config.HCLFunctionsConfig
}

func (w *functionCallWalker) Enter(node hclsyntax.Node) hcl.Diagnostics {
	funcCall, ok := node.(*hclsyntax.FunctionCallExpr)
	if !ok {
		return nil
	}

	switch funcCall.Name {
	case "find_in_parent_folders":
		if w.cfg.FindInParentFoldersExists {
			checkFindInParentFolders(w.issues, w.fileDir, funcCall)
		}
	case "get_env":
		if w.cfg.GetEnvHasDefault {
			checkGetEnvHasDefault(w.issues, funcCall)
		}
	}

	return nil
}

func (w *functionCallWalker) Exit(_ hclsyntax.Node) hcl.Diagnostics { return nil }

func checkFindInParentFolders(issues *[]diag.Issue, fileDir string, funcCall *hclsyntax.FunctionCallExpr) {
	defaultFile := "terragrunt.hcl"
	var filename string

	if len(funcCall.Args) == 0 {
		filename = defaultFile
	} else {
		filename = hclStringValue(funcCall.Args[0])
		if filename == "" {
			return
		}
	}

	if findFileInAncestors(fileDir, filename) == "" {
		msg := fmt.Sprintf("find_in_parent_folders(%q) could not find file in parent directories", filename)
		if len(funcCall.Args) == 0 {
			msg = "find_in_parent_folders() could not find terragrunt.hcl in parent directories"
		}
		*issues = append(*issues, diag.Issue{
			Severity: diag.SeverityError,
			Rule:     "find_in_parent_folders_exists",
			Message:  msg,
			Location: funcCall.Range(),
		})
	}
}

func checkGetEnvHasDefault(issues *[]diag.Issue, funcCall *hclsyntax.FunctionCallExpr) {
	if len(funcCall.Args) < 2 {
		*issues = append(*issues, diag.Issue{
			Severity: diag.SeverityWarning,
			Rule:     "get_env_has_default",
			Message:  "get_env() should have a default value as second argument",
			Location: funcCall.Range(),
		})
	}
}

func findFileInAncestors(dir, filename string) string {
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
