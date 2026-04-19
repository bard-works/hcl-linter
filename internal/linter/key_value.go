package linter

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
)

func checkKeyValueImpl(result *Result, blocks []ast.BlockInfo, cfg *config.KeyValueConfig) {
	if cfg.KeyCase != "" {
		checkKeyCase(result, blocks, cfg.KeyCase)
	}
	if len(cfg.Disallowed) > 0 {
		checkDisallowedKeys(result, blocks, cfg.Disallowed)
	}
	if len(cfg.ValuePattern) > 0 {
		checkValuePattern(result, blocks, cfg.ValuePattern)
	}
}

func checkKeyCase(result *Result, blocks []ast.BlockInfo, caseType string) {
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
		checkBlockKeyCase(result, block.Block.Body, pattern, caseType)
		if len(block.Block.Body.Blocks) > 0 {
			nestedBlocks := ast.GetBlockInfoFromBlocks(block.Block.Body.Blocks)
			checkKeyCase(result, nestedBlocks, caseType)
		}
	}
}

func checkBlockKeyCase(result *Result, body hcl.Body, pattern *regexp.Regexp, caseType string) {
	attrs, _ := body.JustAttributes()
	for name := range attrs {
		if !pattern.MatchString(name) {
			result.Issues = append(result.Issues, Issue{
				Severity: SeverityError,
				Rule:     "key_case",
				Message:  fmt.Sprintf("attribute %q should be %s", name, caseType),
				Location: attrs[name].Expr.Range(),
			})
		}
	}
}

func checkDisallowedKeys(result *Result, blocks []ast.BlockInfo, disallowed []string) {
	disallowedMap := make(map[string]bool)
	for _, k := range disallowed {
		disallowedMap[k] = true
	}

	for _, block := range blocks {
		checkBlockDisallowedKeys(result, block.Block.Body, disallowedMap)
		if len(block.Block.Body.Blocks) > 0 {
			nestedBlocks := ast.GetBlockInfoFromBlocks(block.Block.Body.Blocks)
			checkDisallowedKeys(result, nestedBlocks, disallowed)
		}
	}
}

func checkBlockDisallowedKeys(result *Result, body hcl.Body, disallowed map[string]bool) {
	attrs, _ := body.JustAttributes()
	for name := range attrs {
		if disallowed[name] {
			result.Issues = append(result.Issues, Issue{
				Severity: SeverityError,
				Rule:     "disallowed_keys",
				Message:  fmt.Sprintf("attribute %q is not allowed", name),
				Location: attrs[name].Expr.Range(),
			})
		}
	}
}

func checkValuePattern(result *Result, blocks []ast.BlockInfo, patterns map[string]string) {
	compilePatterns := make(map[string]*regexp.Regexp)
	for key, pat := range patterns {
		if p, err := regexp.Compile(pat); err == nil {
			compilePatterns[key] = p
		}
	}

	for _, block := range blocks {
		checkBlockValuePattern(result, block.Block.Body, compilePatterns)
		if len(block.Block.Body.Blocks) > 0 {
			nestedBlocks := ast.GetBlockInfoFromBlocks(block.Block.Body.Blocks)
			checkValuePattern(result, nestedBlocks, patterns)
		}
	}
}

func checkBlockValuePattern(result *Result, body hcl.Body, patterns map[string]*regexp.Regexp) {
	attrs, _ := body.JustAttributes()
	for key, attr := range attrs {
		if pattern, ok := patterns[key]; ok {
			val, diags := attr.Expr.Value(nil)
			if diags.HasErrors() {
				continue
			}
			strVal := val.AsString()
			if !pattern.MatchString(strVal) {
				result.Issues = append(result.Issues, Issue{
					Severity: SeverityWarning,
					Rule:     "value_pattern",
					Message:  fmt.Sprintf("attribute %q value %q does not match pattern %q", key, strVal, pattern.String()),
					Location: attr.Expr.Range(),
				})
			}
		}
	}
}
