package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/linter"
)

type KeyValueRule struct{}

func (r KeyValueRule) Name() string { return "key_value" }

func (r KeyValueRule) Enabled(cfg *config.Rules) bool {
	return cfg != nil && cfg.KeyValue != nil && cfg.KeyValue.Enabled
}

func (r KeyValueRule) Check(ctx *Context) []linter.Issue {
	cfg := ctx.Config.KeyValue
	var issues []linter.Issue

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

func kvCheckKeyCase(issues *[]linter.Issue, blocks []ast.BlockInfo, caseType string) {
	var pattern *regexp.Regexp
	switch strings.ToLower(caseType) {
	case "camelcase":
		pattern = regexp.MustCompile(`^[a-z][a-zA-Z0-9]*$`)
	case "snake_case":
		pattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	case "kebab-case":
		pattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	default:
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

func kvCheckBlockKeyCase(issues *[]linter.Issue, body hcl.Body, pattern *regexp.Regexp, caseType string) {
	attrs, _ := body.JustAttributes()
	for name := range attrs {
		if !pattern.MatchString(name) {
			*issues = append(*issues, linter.Issue{
				Severity: linter.SeverityError,
				Rule:     "key_case",
				Message:  fmt.Sprintf("attribute %q should be %s", name, caseType),
				Location: attrs[name].Expr.Range(),
			})
		}
	}
}

func kvCheckDisallowedKeys(issues *[]linter.Issue, blocks []ast.BlockInfo, disallowed []string) {
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

func kvCheckBlockDisallowedKeys(issues *[]linter.Issue, body hcl.Body, disallowed map[string]bool) {
	attrs, _ := body.JustAttributes()
	for name := range attrs {
		if disallowed[name] {
			*issues = append(*issues, linter.Issue{
				Severity: linter.SeverityError,
				Rule:     "disallowed_keys",
				Message:  fmt.Sprintf("attribute %q is not allowed", name),
				Location: attrs[name].Expr.Range(),
			})
		}
	}
}

func kvCheckValuePattern(issues *[]linter.Issue, blocks []ast.BlockInfo, patterns map[string]string) {
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

func kvCheckBlockValuePattern(issues *[]linter.Issue, body hcl.Body, patterns map[string]*regexp.Regexp) {
	attrs, _ := body.JustAttributes()
	for key, attr := range attrs {
		if pattern, ok := patterns[key]; ok {
			val, diags := attr.Expr.Value(nil)
			if diags.HasErrors() {
				continue
			}
			strVal := val.AsString()
			if !pattern.MatchString(strVal) {
				*issues = append(*issues, linter.Issue{
					Severity: linter.SeverityWarning,
					Rule:     "value_pattern",
					Message:  fmt.Sprintf("attribute %q value %q does not match pattern %q", key, strVal, pattern.String()),
					Location: attr.Expr.Range(),
				})
			}
		}
	}
}
