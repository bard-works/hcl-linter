package rules

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/linter"
)

type TerragruntFunctionsRule struct{}

func (r TerragruntFunctionsRule) Name() string { return "terragrunt_functions" }

func (r TerragruntFunctionsRule) Enabled(cfg *config.Rules) bool {
	return cfg != nil && cfg.TerragruntFunctions != nil && cfg.TerragruntFunctions.Enabled
}

func (r TerragruntFunctionsRule) Check(ctx *Context) []linter.Issue {
	cfg := ctx.Config.TerragruntFunctions
	if !cfg.FindInParentFoldersExists && !cfg.GetEnvHasDefault {
		return nil
	}

	var issues []linter.Issue
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
	issues  *[]linter.Issue
	fileDir string
	cfg     *config.TerragruntFunctionsConfig
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

func checkFindInParentFolders(issues *[]linter.Issue, fileDir string, funcCall *hclsyntax.FunctionCallExpr) {
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
		msg := "find_in_parent_folders() could not find terragrunt.hcl in parent directories"
		if filename != defaultFile {
			msg = fmt.Sprintf("find_in_parent_folders(%q) could not find file in parent directories", filename)
		}
		*issues = append(*issues, linter.Issue{
			Severity: linter.SeverityError,
			Rule:     "find_in_parent_folders_exists",
			Message:  msg,
			Location: funcCall.Range(),
		})
	}
}

func checkGetEnvHasDefault(issues *[]linter.Issue, funcCall *hclsyntax.FunctionCallExpr) {
	if len(funcCall.Args) < 2 {
		*issues = append(*issues, linter.Issue{
			Severity: linter.SeverityWarning,
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
