package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// ValidationIssue represents a problem found in a config file.
type ValidationIssue struct {
	File    string
	Message string
}

func (v ValidationIssue) String() string {
	return fmt.Sprintf("%s: %s", filepath.Base(v.File), v.Message)
}

var knownRuleBlocks = map[string]bool{
	"block_order": true, "array_format": true, "blank_lines": true,
	"name_validation": true, "duplicates": true, "required_fields": true,
	"required_blocks": true, "dependency_paths": true, "include_paths": true,
	"remote_state": true, "hcl_functions": true, "terraform_block": true,
	"key_value": true, "count_for_each": true, "dependency_outputs": true,
}

// ValidateConfigFile checks a single config file for unknown rule blocks and
// misconfigured (enabled but incomplete) rules. Returns one issue per problem.
func ValidateConfigFile(path string) []ValidationIssue {
	var issues []ValidationIssue
	issues = append(issues, detectUnknownBlocks(path)...)

	rules, err := loadHCLConfig(path)
	if err != nil {
		// Parse errors are surfaced at load time; skip required-field checks.
		return issues
	}
	issues = append(issues, detectMisconfiguredRules(path, rules)...)
	return issues
}

// ValidateDir validates every .hcl file in dir and returns all issues found.
func ValidateDir(dir string) []ValidationIssue {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []ValidationIssue{{File: dir, Message: fmt.Sprintf("cannot read config directory: %s", err)}}
	}

	var issues []ValidationIssue
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".hcl") {
			continue
		}
		issues = append(issues, ValidateConfigFile(filepath.Join(dir, entry.Name()))...)
	}
	return issues
}

func detectUnknownBlocks(path string) []ValidationIssue {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(data, path)
	if diags.HasErrors() {
		return nil
	}
	if file == nil {
		return nil
	}
	syntaxBody, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil
	}

	var issues []ValidationIssue
	for _, block := range syntaxBody.Blocks {
		if block.Type != "rules" {
			continue
		}
		for _, ruleBlock := range block.Body.Blocks {
			if !knownRuleBlocks[ruleBlock.Type] {
				issues = append(issues, ValidationIssue{
					File:    path,
					Message: fmt.Sprintf("unknown rule block %q (check for typos)", ruleBlock.Type),
				})
			}
		}
	}
	return issues
}

func detectMisconfiguredRules(path string, rules *Rules) []ValidationIssue {
	var issues []ValidationIssue
	issues = append(issues, detectBlockOrderIssues(path, rules)...)
	issues = append(issues, detectNameValidationIssues(path, rules)...)
	issues = append(issues, detectRequiredBlocksIssues(path, rules)...)
	issues = append(issues, detectKeyValueIssues(path, rules)...)
	return issues
}

func detectBlockOrderIssues(path string, rules *Rules) []ValidationIssue {
	if rules.BlockOrder != nil && rules.BlockOrder.Enabled && len(rules.BlockOrder.Order) == 0 {
		return []ValidationIssue{{File: path, Message: "block_order: enabled but 'order' list is empty"}}
	}
	return nil
}

func detectNameValidationIssues(path string, rules *Rules) []ValidationIssue {
	if rules.NameValidation == nil || !rules.NameValidation.Enabled {
		return nil
	}
	if rules.NameValidation.Pattern == "" {
		return []ValidationIssue{{File: path, Message: "name_validation: enabled but 'pattern' is not set"}}
	}
	if _, err := regexp.Compile(rules.NameValidation.Pattern); err != nil {
		return []ValidationIssue{{
			File:    path,
			Message: fmt.Sprintf("name_validation: pattern %q does not compile: %v", rules.NameValidation.Pattern, err),
		}}
	}
	return nil
}

func detectRequiredBlocksIssues(path string, rules *Rules) []ValidationIssue {
	if rules.RequiredBlocks != nil && len(rules.RequiredBlocks.Required) == 0 {
		return []ValidationIssue{{
			File:    path,
			Message: "required_blocks: configured but contains no 'required' entries",
		}}
	}
	return nil
}

func detectKeyValueIssues(path string, rules *Rules) []ValidationIssue {
	if rules.KeyValue == nil || !rules.KeyValue.Enabled {
		return nil
	}

	var issues []ValidationIssue
	if rules.KeyValue.KeyCase == "" && len(rules.KeyValue.ValuePattern) == 0 &&
		len(rules.KeyValue.Disallowed) == 0 {
		issues = append(issues, ValidationIssue{
			File:    path,
			Message: "key_value: enabled but none of 'key_case', 'value_pattern', or 'disallowed' are set",
		})
	}
	for key, pat := range rules.KeyValue.ValuePattern {
		if _, err := regexp.Compile(pat); err != nil {
			issues = append(issues, ValidationIssue{
				File:    path,
				Message: fmt.Sprintf("key_value.value_pattern[%q]: pattern %q does not compile: %v", key, pat, err),
			})
		}
	}
	return issues
}
