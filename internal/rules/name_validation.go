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

const defaultNamePattern = `^[a-z][a-z0-9_]*$`

type NameValidationRule struct{}

func (r NameValidationRule) Name() string  { return "name_validation" }
func (r NameValidationRule) Priority() int { return PrioritySemantic }

func init() { Register(NameValidationRule{}) }

func (r NameValidationRule) Doc() RuleDoc {
	return RuleDoc{
		Summary:     "Enforces naming conventions for block labels.",
		Severity:    "error",
		Fixable:     true,
		ConfigBlock: "name_validation",
		ConfigFields: []ConfigField{
			{Name: "enabled", Type: "bool", Required: true, Doc: "Activate the rule"},
			{Name: "pattern", Type: "string", Required: false, Default: `"^[a-z][a-z0-9_]*$"`, Doc: "Regex that block labels must match"},
			{Name: "blocks", Type: "[]string", Required: false, Default: "[]", Doc: "Block types to validate; empty = all block types"},
		},
		Example: Example{
			Violation: `dependency "my-vpc" { config_path = "../vpc" }`,
			Fixed:     `dependency "my_vpc" { config_path = "../vpc" }`,
		},
	}
}

func (r NameValidationRule) Enabled(cfg *config.Rules) bool {
	return cfg != nil && cfg.NameValidation != nil && cfg.NameValidation.Enabled
}

func (r NameValidationRule) Check(ctx *Context) []diag.Issue {
	cfg := ctx.Config.NameValidation
	var issues []diag.Issue

	var allowedBlocks map[string]bool
	if len(cfg.Blocks) > 0 {
		allowedBlocks = make(map[string]bool, len(cfg.Blocks))
		for _, b := range cfg.Blocks {
			allowedBlocks[b] = true
		}
	}

	var pattern *regexp.Regexp
	if cfg.Pattern != "" {
		var err error
		pattern, err = regexp.Compile(cfg.Pattern)
		if err != nil {
			issues = append(issues, diag.Issue{
				Severity: diag.SeverityError,
				Rule:     "name_validation",
				Message:  fmt.Sprintf("name_validation pattern %q is invalid: %v", cfg.Pattern, err),
				Location: hcl.Range{},
			})
			return issues
		}
	}

	nameValidationRecursive(&issues, ctx.Blocks, allowedBlocks, pattern)
	return issues
}

func (r NameValidationRule) Fix(ctx *Context) (int, error) {
	newContent, changed := FixNameValidation(string(ctx.Content), ctx.Blocks, ctx.Config.NameValidation)
	if !changed {
		return 0, nil
	}
	ctx.Content = []byte(newContent)
	return 1, nil
}

// FixNameValidation replaces hyphens and spaces with underscores in block labels
// and updates all references throughout the file.
func FixNameValidation(content string, blocks []ast.BlockInfo, cfg *config.NameValidationConfig) (string, bool) {
	pattern := cfg.Pattern
	if pattern == "" {
		pattern = defaultNamePattern
	}
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return content, false
	}

	blockSet := make(map[string]bool, len(cfg.Blocks))
	for _, b := range cfg.Blocks {
		blockSet[b] = true
	}

	hasChanges := false
	labelChanges := make(map[string]string) // oldLabel -> newLabel

	for _, block := range blocks {
		if len(blockSet) > 0 && !blockSet[block.Type] {
			continue
		}
		for _, label := range block.Labels {
			if !regex.MatchString(label) {
				newLabel := strings.ReplaceAll(label, "-", "_")
				newLabel = strings.Join(strings.Fields(newLabel), "")
				if newLabel != label {
					labelChanges[label] = newLabel
					hasChanges = true
				}
			}
		}
	}

	if !hasChanges {
		return content, false
	}

	// Apply all label changes and update references
	for oldLabel, newLabel := range labelChanges {
		for _, block := range blocks {
			if len(blockSet) > 0 && !blockSet[block.Type] {
				continue
			}
			// Fix the label definition (quoted)
			content = strings.ReplaceAll(content, fmt.Sprintf("%q", oldLabel), fmt.Sprintf("%q", newLabel))

			// Fix dot notation references: dependency.old_name -> dependency.new_name
			dotRef := block.Type + "." + oldLabel
			dotNew := block.Type + "." + newLabel
			content = strings.ReplaceAll(content, dotRef, dotNew)

			// Fix index notation: dependency["old_name"] -> dependency["new_name"]
			idxRef := block.Type + `["` + oldLabel + `"]`
			idxNew := block.Type + `["` + newLabel + `"]`
			content = strings.ReplaceAll(content, idxRef, idxNew)

			// Fix interpolation references: ${dependency.old_name.outputs} -> ${dependency.new_name.outputs}
			interpRef := `${` + block.Type + `.` + oldLabel
			interpNew := `${` + block.Type + `.` + newLabel
			content = strings.ReplaceAll(content, interpRef, interpNew)
		}
	}

	return content, true
}

func nameValidationRecursive(issues *[]diag.Issue, blocks []ast.BlockInfo, allowedBlocks map[string]bool, pattern *regexp.Regexp) {
	for _, block := range blocks {
		if len(block.Labels) > 0 && (allowedBlocks == nil || allowedBlocks[block.Type]) {
			name := block.Labels[0]
			var valid bool
			var msg string
			if pattern != nil {
				valid = pattern.MatchString(name)
				msg = fmt.Sprintf("invalid name %q: does not match required pattern %q", name, pattern.String())
			} else {
				valid = isValidIdentifier(name)
				msg = fmt.Sprintf("invalid name %q: must contain only lowercase letters, numbers, and underscores, and must start with a letter", name)
			}
			if !valid {
				*issues = append(*issues, diag.Issue{
					Severity: diag.SeverityError,
					Rule:     "name_validation",
					Message:  msg,
					Location: block.Block.TypeRange,
				})
			}
		}
		if len(block.Block.Body.Blocks) > 0 {
			nameValidationRecursive(issues, ast.GetBlockInfoFromBlocks(block.Block.Body.Blocks), allowedBlocks, pattern)
		}
	}
}

func isValidIdentifier(name string) bool {
	if name == "" || !isValidStartChar(rune(name[0])) {
		return false
	}
	for _, ch := range name {
		if !isValidIdentifierChar(ch) {
			return false
		}
	}
	return true
}

func isValidStartChar(ch rune) bool {
	return ch >= 'a' && ch <= 'z'
}

func isValidIdentifierChar(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_'
}
