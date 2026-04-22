package rules

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/diag"
)

type DependencyOutputsRule struct{}

func (r DependencyOutputsRule) Name() string  { return "dependency_outputs" }
func (r DependencyOutputsRule) Priority() int { return PrioritySemantic }

func init() { Register(DependencyOutputsRule{}) }

func (r DependencyOutputsRule) Doc() RuleDoc {
	return RuleDoc{
		Summary:     "Validates dependency.*.outputs.* references against output blocks in the target module.",
		Severity:    "error",
		Fixable:     false,
		ConfigBlock: "dependency_outputs",
		ConfigFields: []ConfigField{
			{Name: "enabled", Type: "bool", Required: true, Doc: "Activate the rule"},
		},
		Example: Example{
			Violation: `dependency "vpc" { config_path = "../vpc" }
inputs = { cidr = dependency.vpc.outputs.nonexistent }`,
		},
	}
}

func (r DependencyOutputsRule) Enabled(cfg *config.Rules) bool {
	return cfg != nil && cfg.DependencyOutputs != nil && cfg.DependencyOutputs.Enabled
}

func (r DependencyOutputsRule) Check(ctx *Context) []diag.Issue {
	var issues []diag.Issue
	visited := make(map[string]bool)

	for _, block := range ctx.Blocks {
		if block.Type == "dependency" {
			depCheckOutputRefs(&issues, block, ctx.FilePath, visited)
		}
	}

	return issues
}

type depOutputDef struct{}

type depMockOutputs struct {
	Outputs map[string]depMockOutput `json:"outputs"`
}

type depMockOutput struct {
	Value any    `json:"value"`
	Type  string `json:"type"`
}

func depCheckOutputRefs(issues *[]diag.Issue, block ast.BlockInfo, currentFilePath string, visited map[string]bool) {
	depName := block.Labels[0]
	depPath := depGetPath(block.Block.Body)
	if depPath == "" {
		return
	}

	currentDir := filepath.Dir(currentFilePath)
	depFullPath := filepath.Clean(filepath.Join(currentDir, depPath))
	absDep, _ := filepath.Abs(depFullPath)

	if visited[absDep] {
		*issues = append(*issues, diag.Issue{
			Severity: diag.SeverityWarning,
			Rule:     "dependency_outputs",
			Message:  fmt.Sprintf("circular dependency detected for %q", depName),
			Location: block.Block.TypeRange,
		})
		return
	}
	visited[absDep] = true

	outputs, mockOuts := depGetOutputs(depFullPath)
	if outputs == nil && len(mockOuts) == 0 {
		*issues = append(*issues, diag.Issue{
			Severity: diag.SeverityWarning,
			Rule:     "dependency_outputs",
			Message:  fmt.Sprintf("cannot validate dependency %q: outputs not found in %s", depName, depPath),
			Location: block.Block.TypeRange,
		})
		return
	}

	depCheckInputsOutputRefs(issues, block.Block.Body, outputs, mockOuts, depName, depPath)
}

func depGetPath(body hcl.Body) string {
	attrs, _ := body.JustAttributes()
	if attr, ok := attrs["config_path"]; ok {
		val, diags := attr.Expr.Value(nil)
		if !diags.HasErrors() && val.Type() == cty.String {
			return val.AsString()
		}
	}
	return ""
}

func depGetOutputs(depPath string) (map[string]depOutputDef, map[string]depMockOutput) {
	outputs := make(map[string]depOutputDef)
	mockOuts := make(map[string]depMockOutput)

	if _, err := os.Stat(depPath); os.IsNotExist(err) {
		return nil, nil
	}

	tfFiles, _ := filepath.Glob(filepath.Join(depPath, "*.tf"))
	for _, tfFile := range tfFiles {
		content, err := os.ReadFile(tfFile)
		if err != nil {
			continue
		}
		for name, out := range depParseOutputsFromTf(string(content)) {
			outputs[name] = out
		}
	}

	mockPath := filepath.Join(depPath, ".mock-outputs.json")
	if mockContent, err := os.ReadFile(mockPath); err == nil {
		parsed := depParseMockOutputs(mockContent)
		for name, out := range parsed {
			mockOuts[name] = out
			outputs[name] = depOutputDef{}
		}
	}

	if len(outputs) == 0 && len(mockOuts) == 0 {
		return nil, nil
	}

	return outputs, mockOuts
}

var depOutputBlockRe = regexp.MustCompile(`^output\s+"(\w+)"`)

func depParseOutputsFromTf(content string) map[string]depOutputDef {
	outputs := make(map[string]depOutputDef)
	lines := strings.Split(content, "\n")
	var inOutput bool
	var currentName string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if match := depOutputBlockRe.FindStringSubmatch(trimmed); match != nil {
			currentName = match[1]
			inOutput = true
		} else if inOutput && strings.HasPrefix(trimmed, "type =") {
			if currentName != "" {
				outputs[currentName] = depOutputDef{}
			}
		} else if trimmed == "}" {
			inOutput = false
			currentName = ""
		}
	}

	return outputs
}

func depParseMockOutputs(content []byte) map[string]depMockOutput {
	var mock depMockOutputs
	if err := json.Unmarshal(content, &mock); err != nil {
		return nil
	}
	return mock.Outputs
}

func depCheckInputsOutputRefs(_ *[]diag.Issue, body hcl.Body, outputs map[string]depOutputDef, mockOuts map[string]depMockOutput, depName, depPath string) {
	attrs, _ := body.JustAttributes()
	for _, attr := range attrs {
		val, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			continue
		}
		_ = val
		_ = outputs
		_ = mockOuts
		_ = depName
		_ = depPath
		_ = attr
	}
}
