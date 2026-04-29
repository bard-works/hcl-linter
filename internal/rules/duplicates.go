package rules

import (
	"fmt"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/diag"
)

type DuplicatesRule struct{}

func (r DuplicatesRule) Name() string  { return "duplicates" }
func (r DuplicatesRule) Priority() int { return PrioritySemantic }

func init() { Register(DuplicatesRule{}) }

func (r DuplicatesRule) Doc() RuleDoc {
	return RuleDoc{
		Summary:     "Detects duplicate block definitions with the same type and label.",
		Severity:    "error",
		Fixable:     false,
		ConfigBlock: "duplicates",
		ConfigFields: []ConfigField{
			{Name: "enabled", Type: "bool", Required: true, Doc: "Activate the rule"},
			{Name: "blocks", Type: "[]string", Required: false, Default: "[]", Doc: "Block types to check; empty = all block types"},
		},
		Example: Example{
			Violation: `dependency "vpc" { config_path = "../vpc" }
dependency "vpc" { config_path = "../other-vpc" }`,
		},
	}
}

func (r DuplicatesRule) Enabled(cfg *config.Rules) bool {
	return cfg != nil && cfg.Duplicates != nil && cfg.Duplicates.Enabled
}

func (r DuplicatesRule) Check(ctx *Context) []diag.Issue {
	var issues []diag.Issue
	checkDuplicates(&issues, ctx.Blocks, ctx.Config.Duplicates)
	return issues
}

func checkDuplicates(issues *[]diag.Issue, blocks []ast.BlockInfo, cfg *config.DuplicatesConfig) {
	var allowedTypes map[string]bool
	if cfg != nil && len(cfg.Blocks) > 0 {
		allowedTypes = make(map[string]bool, len(cfg.Blocks))
		for _, t := range cfg.Blocks {
			allowedTypes[t] = true
		}
	}

	seen := make(map[string]map[string]bool)

	for _, block := range blocks {
		if allowedTypes == nil || allowedTypes[block.Type] {
			if seen[block.Type] == nil {
				seen[block.Type] = make(map[string]bool)
			}
			identifier := ""
			if len(block.Labels) > 0 {
				identifier = block.Labels[0]
			}
			if seen[block.Type][identifier] {
				*issues = append(*issues, diag.Issue{
					Severity: diag.SeverityError,
					Rule:     "duplicates",
					Message:  fmt.Sprintf("duplicate %s block with name/label %q", block.Type, identifier),
					Location: block.Block.TypeRange,
				})
			} else {
				seen[block.Type][identifier] = true
			}
		}

		if len(block.Block.Body.Blocks) > 0 {
			checkDuplicates(issues, ast.GetBlockInfoFromBlocks(block.Block.Body.Blocks), cfg)
		}
	}
}
