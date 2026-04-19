package linter

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
)

type outputDef struct{}

type mockOutputs struct {
	Outputs map[string]mockOutput `json:"outputs"`
}

type mockOutput struct {
	Value any    `json:"value"`
	Type  string `json:"type"`
}

func checkDependencyOutputsImpl(result *Result, blocks []ast.BlockInfo, filePath string, cfg *config.DependencyOutputsConfig, _ *config.Loader) {
	if !cfg.Enabled {
		return
	}

	visited := make(map[string]bool)
	for _, block := range blocks {
		if block.Type == "dependency" {
			checkDependencyOutputRefs(result, block, filePath, visited)
		}
	}
}

func checkDependencyOutputRefs(result *Result, block ast.BlockInfo, currentFilePath string, visited map[string]bool) {
	depName := block.Labels[0]
	depPath := getDependencyPath(block.Block.Body)
	if depPath == "" {
		return
	}

	currentDir := filepath.Dir(currentFilePath)
	depFullPath := filepath.Clean(filepath.Join(currentDir, depPath))

	absDep, _ := filepath.Abs(depFullPath)

	if visited[absDep] {
		result.Issues = append(result.Issues, Issue{
			Severity: SeverityWarning,
			Rule:     "dependency_outputs",
			Message:  fmt.Sprintf("circular dependency detected for %q", depName),
			Location: block.Block.TypeRange,
		})
		return
	}
	visited[absDep] = true

	outputs, mockOutputs := getDependencyOutputs(depFullPath)
	if outputs == nil && len(mockOutputs) == 0 {
		result.Issues = append(result.Issues, Issue{
			Severity: SeverityWarning,
			Rule:     "dependency_outputs",
			Message:  fmt.Sprintf("cannot validate dependency %q: outputs not found in %s", depName, depPath),
			Location: block.Block.TypeRange,
		})
		return
	}

	checkInputsOutputRefs(result, block.Block.Body, outputs, mockOutputs, depName, depPath)
}

func getDependencyPath(body hcl.Body) string {
	attrs, _ := body.JustAttributes()
	if attr, ok := attrs["config_path"]; ok {
		val, diags := attr.Expr.Value(nil)
		if !diags.HasErrors() && val.Type() == cty.String {
			return val.AsString()
		}
	}
	return ""
}

func getDependencyOutputs(depPath string) (map[string]outputDef, map[string]mockOutput) {
	outputs := make(map[string]outputDef)
	mockOuts := make(map[string]mockOutput)

	if _, err := os.Stat(depPath); os.IsNotExist(err) {
		return nil, nil
	}

	tfFiles, _ := filepath.Glob(filepath.Join(depPath, "*.tf"))
	for _, tfFile := range tfFiles {
		content, err := os.ReadFile(tfFile)
		if err != nil {
			continue
		}

		outputsFromTf := parseOutputsFromTf(string(content))
		for name, out := range outputsFromTf {
			outputs[name] = out
		}
	}

	mockPath := filepath.Join(depPath, ".mock-outputs.json")
	if mockContent, err := os.ReadFile(mockPath); err == nil {
		mockOuts = parseMockOutputs(mockContent)
		for name := range mockOuts {
			outputs[name] = outputDef{}
		}
	}

	if len(outputs) == 0 && len(mockOuts) == 0 {
		return nil, nil
	}

	return outputs, mockOuts
}

var outputBlockRe = regexp.MustCompile(`^output\s+"(\w+)"`)

func parseOutputsFromTf(content string) map[string]outputDef {
	outputs := make(map[string]outputDef)

	lines := strings.Split(content, "\n")
	var inOutput bool
	var currentName string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if match := outputBlockRe.FindStringSubmatch(trimmed); match != nil {
			currentName = match[1]
			inOutput = true
		} else if inOutput && strings.HasPrefix(trimmed, "type =") {
			if currentName != "" {
				outputs[currentName] = outputDef{}
			}
		} else if trimmed == "}" {
			inOutput = false
			currentName = ""
		}
	}

	return outputs
}

func parseMockOutputs(content []byte) map[string]mockOutput {
	var mock mockOutputs
	if err := json.Unmarshal(content, &mock); err != nil {
		return nil
	}
	return mock.Outputs
}

func checkInputsOutputRefs(result *Result, body hcl.Body, outputs map[string]outputDef, mockOutputs map[string]mockOutput, depName, depPath string) {
	attrs, _ := body.JustAttributes()
	for _, attr := range attrs {
		val, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			continue
		}

		checkObjectForOutputs(result, val, outputs, mockOutputs, depName, depPath, attr.Expr.Range())
	}
}

// checkObjectForOutputs is kept for future use with output validation
func checkObjectForOutputs(_ *Result, _ cty.Value, _ map[string]outputDef, _ map[string]mockOutput, _, _ string, _ hcl.Range) {
}
