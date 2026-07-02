package rules

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
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

	// Pass 1: resolve every dependency block's target module and collect its
	// declared (or mocked) outputs. Emits the circular-dependency and
	// "cannot validate" warnings.
	targets := make(map[string]*depTarget)
	visited := make(map[string]bool)
	for _, block := range ctx.Blocks {
		if block.Type != "dependency" || len(block.Labels) == 0 {
			continue
		}
		depResolveTarget(&issues, targets, block, ctx.FilePath, visited, ctx.Breaker)
	}

	// Pass 2: walk every expression in the file for
	// dependency.<name>.outputs.<attr> traversals and flag references to
	// outputs the resolved target does not declare. Unresolvable targets
	// (missing dir, open circuit breaker, no outputs found) were already
	// warned about above and produce no reference errors.
	for _, ref := range depCollectOutputRefs(ctx.File) {
		target, ok := targets[ref.depName]
		if !ok || !target.resolvable {
			continue
		}
		if _, ok := target.outputs[ref.attr]; ok {
			continue
		}
		if _, ok := target.mockOuts[ref.attr]; ok {
			continue
		}
		issues = append(issues, diag.Issue{
			Severity: diag.SeverityError,
			Rule:     "dependency_outputs",
			Message: fmt.Sprintf("dependency %q: reference to undeclared output %q (not found in %s)",
				ref.depName, ref.attr, target.path),
			Location: ref.rng,
		})
	}

	return issues
}

// depTarget is one dependency block's resolved module.
type depTarget struct {
	path       string // config_path as written
	outputs    map[string]depOutputDef
	mockOuts   map[string]depMockOutput
	resolvable bool // true when at least one output (real or mock) was found
}

type depOutputDef struct{}

type depMockOutputs struct {
	Outputs map[string]depMockOutput `json:"outputs"`
}

type depMockOutput struct {
	Value any    `json:"value"`
	Type  string `json:"type"`
}

// depResolveTarget resolves one dependency block into targets, warning about
// circular references and targets whose outputs cannot be found.
func depResolveTarget(
	issues *[]diag.Issue,
	targets map[string]*depTarget,
	block ast.BlockInfo,
	currentFilePath string,
	visited map[string]bool,
	cb *CircuitBreaker,
) {
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

	outputs, mockOuts := depGetOutputs(cb, depFullPath)
	if outputs == nil && len(mockOuts) == 0 {
		*issues = append(*issues, diag.Issue{
			Severity: diag.SeverityWarning,
			Rule:     "dependency_outputs",
			Message:  fmt.Sprintf("cannot validate dependency %q: outputs not found in %s", depName, depPath),
			Location: block.Block.TypeRange,
		})
		targets[depName] = &depTarget{path: depPath}
		return
	}

	targets[depName] = &depTarget{
		path:       depPath,
		outputs:    outputs,
		mockOuts:   mockOuts,
		resolvable: true,
	}
}

func depGetPath(body hcl.Body) string {
	attrs := ast.GetBodyAttributes(body)
	if attr, ok := attrs["config_path"]; ok {
		val, diags := attr.Expr.Value(nil)
		if !diags.HasErrors() && val.Type() == cty.String {
			return val.AsString()
		}
	}
	return ""
}

func depGetOutputs(cb *CircuitBreaker, depPath string) (map[string]depOutputDef, map[string]depMockOutput) {
	outputs := make(map[string]depOutputDef)
	mockOuts := make(map[string]depMockOutput)

	// Short-circuit before touching the filesystem (including the Glob below)
	// when the breaker is open.
	if !cb.Allow() {
		return nil, nil
	}

	if _, err := cb.Stat(depPath); os.IsNotExist(err) {
		return nil, nil
	}

	tfFiles, _ := filepath.Glob(filepath.Join(depPath, "*.tf"))
	for _, tfFile := range tfFiles {
		content, err := cb.ReadFile(tfFile)
		if err != nil {
			continue
		}
		for name, out := range depParseOutputsFromTf(string(content)) {
			outputs[name] = out
		}
	}

	mockPath := filepath.Join(depPath, ".mock-outputs.json")
	if mockContent, err := cb.ReadFile(mockPath); err == nil {
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

// depParseOutputsFromTf collects top-level `output "name"` block names.
// Registration happens on the header line itself: Terraform output blocks
// carry value/description/sensitive but no mandatory attribute we could
// anchor on (in particular no `type` - that belongs to variable blocks).
func depParseOutputsFromTf(content string) map[string]depOutputDef {
	outputs := make(map[string]depOutputDef)
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if match := depOutputBlockRe.FindStringSubmatch(trimmed); match != nil {
			outputs[match[1]] = depOutputDef{}
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

// depOutputRef is one dependency.<name>.outputs.<attr> reference found in the
// linted file.
type depOutputRef struct {
	depName string
	attr    string
	rng     hcl.Range
}

// depCollectOutputRefs walks every expression in the file (top-level
// attributes, nested blocks, function arguments) and returns all
// dependency.<name>.outputs.<attr> traversals.
func depCollectOutputRefs(file *hcl.File) []depOutputRef {
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil
	}
	var refs []depOutputRef
	_ = hclsyntax.VisitAll(body, func(node hclsyntax.Node) hcl.Diagnostics {
		expr, ok := node.(*hclsyntax.ScopeTraversalExpr)
		if !ok {
			return nil
		}
		if ref, ok := depParseOutputTraversal(expr.Traversal); ok {
			refs = append(refs, ref)
		}
		return nil
	})
	return refs
}

// depParseOutputTraversal matches traversals of the shape
// dependency.<name>.outputs.<attr>[...]. Other shapes (index steps in the
// first four positions, shorter traversals) are skipped rather than guessed
// at, so unusual expressions never produce false positives.
func depParseOutputTraversal(tr hcl.Traversal) (depOutputRef, bool) {
	if len(tr) < 4 {
		return depOutputRef{}, false
	}
	root, ok := tr[0].(hcl.TraverseRoot)
	if !ok || root.Name != "dependency" {
		return depOutputRef{}, false
	}
	nameStep, ok := tr[1].(hcl.TraverseAttr)
	if !ok {
		return depOutputRef{}, false
	}
	outputsStep, ok := tr[2].(hcl.TraverseAttr)
	if !ok || outputsStep.Name != "outputs" {
		return depOutputRef{}, false
	}
	attrStep, ok := tr[3].(hcl.TraverseAttr)
	if !ok {
		return depOutputRef{}, false
	}
	return depOutputRef{
		depName: nameStep.Name,
		attr:    attrStep.Name,
		rng:     hcl.RangeBetween(root.SourceRange(), attrStep.SourceRange()),
	}, true
}
