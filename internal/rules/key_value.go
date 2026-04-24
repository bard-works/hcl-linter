package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/diag"
)

type KeyValueRule struct{}

func (r KeyValueRule) Name() string  { return "key_value" }
func (r KeyValueRule) Priority() int { return PrioritySemantic }

func init() { Register(KeyValueRule{}) }

func (r KeyValueRule) Doc() RuleDoc {
	return RuleDoc{
		Summary:     "Enforces key naming conventions, value patterns, and disallowed attributes.",
		Severity:    "error",
		Fixable:     false,
		ConfigBlock: "key_value",
		ConfigFields: []ConfigField{
			{Name: "enabled", Type: "bool", Required: true, Doc: "Activate the rule"},
			{Name: "key_case", Type: "string", Required: false, Default: `""`, Doc: `Enforce key casing: "snake_case", "camelCase", or "kebab-case"`},
			{Name: "value_pattern", Type: "map[string]string", Required: false, Default: "{}", Doc: "Regex patterns per attribute name that values must match"},
			{Name: "disallowed", Type: "[]string", Required: false, Default: "[]", Doc: "Attribute names that must not appear"},
		},
		Example: Example{
			Violation: `inputs = { myKey = "value" }  # violates snake_case`,
		},
	}
}

func (r KeyValueRule) Enabled(cfg *config.Rules) bool {
	return cfg != nil && cfg.KeyValue != nil && cfg.KeyValue.Enabled
}

func (r KeyValueRule) Check(ctx *Context) []diag.Issue {
	cfg := ctx.Config.KeyValue
	var issues []diag.Issue

	if cfg.KeyCase != "" {
		kvCheckKeyCase(&issues, ctx.Blocks, cfg.KeyCase)
	}
	if len(cfg.Disallowed) > 0 {
		kvCheckDisallowedKeys(&issues, ctx.Blocks, cfg.Disallowed)
	}
	if len(cfg.ValuePattern) > 0 {
		kvCheckValuePattern(&issues, ctx.Blocks, cfg.ValuePattern)
	}

	return issues
}

var keyCasePatterns = map[string]*regexp.Regexp{
	"camelcase":  regexp.MustCompile(`^[a-z][a-zA-Z0-9]*$`),
	"snake_case": regexp.MustCompile(`^[a-z][a-z0-9_]*$`),
	"kebab-case": regexp.MustCompile(`^[a-z][a-z0-9-]*$`),
}

func init() {
	for name, re := range keyCasePatterns {
		if re == nil {
			panic(fmt.Sprintf("invalid regex pattern for key_case %q", name))
		}
	}
}

func kvCheckKeyCase(issues *[]diag.Issue, blocks []ast.BlockInfo, caseType string) {
	pattern, ok := keyCasePatterns[strings.ToLower(caseType)]
	if !ok {
		return
	}

	for _, block := range blocks {
		kvCheckBlockKeyCase(issues, block.Block.Body, pattern, caseType)
		if len(block.Block.Body.Blocks) > 0 {
			nestedBlocks := ast.GetBlockInfoFromBlocks(block.Block.Body.Blocks)
			kvCheckKeyCase(issues, nestedBlocks, caseType)
		}
	}
}

func kvCheckBlockKeyCase(issues *[]diag.Issue, body hcl.Body, pattern *regexp.Regexp, caseType string) {
	attrs := ast.GetBodyAttributes(body)
	for name := range attrs {
		if !pattern.MatchString(name) {
			*issues = append(*issues, diag.Issue{
				Severity: diag.SeverityError,
				Rule:     "key_case",
				Message:  fmt.Sprintf("attribute %q should be %s", name, caseType),
				Location: attrs[name].Expr.Range(),
			})
		}
	}
}

func kvCheckDisallowedKeys(issues *[]diag.Issue, blocks []ast.BlockInfo, disallowed []string) {
	disallowedMap := make(map[string]bool)
	for _, k := range disallowed {
		disallowedMap[k] = true
	}

	for _, block := range blocks {
		kvCheckBlockDisallowedKeys(issues, block.Block.Body, disallowedMap)
		if len(block.Block.Body.Blocks) > 0 {
			nestedBlocks := ast.GetBlockInfoFromBlocks(block.Block.Body.Blocks)
			kvCheckDisallowedKeys(issues, nestedBlocks, disallowed)
		}
	}
}

func kvCheckBlockDisallowedKeys(issues *[]diag.Issue, body hcl.Body, disallowed map[string]bool) {
	attrs := ast.GetBodyAttributes(body)
	for name := range attrs {
		if disallowed[name] {
			*issues = append(*issues, diag.Issue{
				Severity: diag.SeverityError,
				Rule:     "disallowed_keys",
				Message:  fmt.Sprintf("attribute %q is not allowed", name),
				Location: attrs[name].Expr.Range(),
			})
		}
	}
}

func kvCheckValuePattern(issues *[]diag.Issue, blocks []ast.BlockInfo, patterns map[string]string) {
	compiled := make(map[string]*regexp.Regexp)
	for key, pat := range patterns {
		if p, err := regexp.Compile(pat); err == nil {
			compiled[key] = p
		}
	}

	for _, block := range blocks {
		kvCheckBlockValuePattern(issues, block.Block.Body, compiled)
		if len(block.Block.Body.Blocks) > 0 {
			nestedBlocks := ast.GetBlockInfoFromBlocks(block.Block.Body.Blocks)
			kvCheckValuePattern(issues, nestedBlocks, patterns)
		}
	}
}

func kvCheckBlockValuePattern(issues *[]diag.Issue, body hcl.Body, patterns map[string]*regexp.Regexp) {
	attrs := ast.GetBodyAttributes(body)
	for key, attr := range attrs {
		if pattern, ok := patterns[key]; ok {
			val, diags := attr.Expr.Value(nil)
			if diags.HasErrors() {
				continue
			}
			strVal := val.AsString()
			if !pattern.MatchString(strVal) {
				*issues = append(*issues, diag.Issue{
					Severity: diag.SeverityWarning,
					Rule:     "value_pattern",
					Message:  fmt.Sprintf("attribute %q value %q does not match pattern %q", key, strVal, pattern.String()),
					Location: attr.Expr.Range(),
				})
			}
		}
	}
}
