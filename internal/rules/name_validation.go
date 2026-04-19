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

type NameValidationRule struct{}

func (r NameValidationRule) Name() string { return "name_validation" }

func (r NameValidationRule) Enabled(cfg *config.Rules) bool {
	return cfg != nil && cfg.NameValidation != nil && cfg.NameValidation.Enabled
}

func (r NameValidationRule) Check(ctx *Context) []linter.Issue {
	cfg := ctx.Config.NameValidation
	var issues []linter.Issue

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
			issues = append(issues, linter.Issue{
				Severity: linter.SeverityError,
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

func (r NameValidationRule) Fix(ctx *Context) ([]byte, bool, error) {
	cfg := ctx.Config.NameValidation
	newContent, changed := FixNameValidation(string(ctx.Content), ctx.Blocks, cfg)
	if !changed {
		return ctx.Content, false, nil
	}
	return []byte(newContent), true, nil
}

// FixNameValidation replaces hyphens with underscores in block labels that
// violate the name pattern. Exported for use by the legacy fixer.
func FixNameValidation(content string, blocks []ast.BlockInfo, cfg *config.NameValidationConfig) (string, bool) {
	pattern := cfg.Pattern
	if pattern == "" {
		pattern = `^[a-z][a-z0-9_]*$`
	}
	regex := regexp.MustCompile(pattern)

	blockSet := make(map[string]bool, len(cfg.Blocks))
	for _, b := range cfg.Blocks {
		blockSet[b] = true
	}

	hasChanges := false
	for _, block := range blocks {
		if len(blockSet) > 0 && !blockSet[block.Type] {
			continue
		}
		for _, label := range block.Labels {
			if !regex.MatchString(label) && strings.Contains(label, "-") {
				newLabel := strings.ReplaceAll(label, "-", "_")
				content = strings.ReplaceAll(content, fmt.Sprintf("%q", label), fmt.Sprintf("%q", newLabel))
				hasChanges = true
			}
		}
	}
	return content, hasChanges
}

func nameValidationRecursive(issues *[]linter.Issue, blocks []ast.BlockInfo, allowedBlocks map[string]bool, pattern *regexp.Regexp) {
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
				*issues = append(*issues, linter.Issue{
					Severity: linter.SeverityError,
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
	if name == "" || !isLowerLetter(rune(name[0])) {
		return false
	}
	for _, ch := range name {
		if !isLowerLetter(ch) && !isDigit(ch) && ch != '_' {
			return false
		}
	}
	return true
}

func isLowerLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}
