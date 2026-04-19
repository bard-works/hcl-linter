package linter

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
)

func checkTerraformBlockImpl(result *Result, file *hcl.File, cfg *config.TerraformBlockConfig) {
	blocks := ast.GetTopLevelBlocks(file)

	for _, block := range blocks {
		if block.Type != "terraform" {
			continue
		}

		attrs := ast.GetBlockAttributes(block.Block.Body)

		if cfg.SourceRequired {
			checkTerraformSourceRequired(result, attrs, block.Block)
		}

		if cfg.VersionFormat {
			checkTerraformVersionFormat(result, attrs, block.Block)
		}

		if cfg.ExtraArgumentsValid {
			checkTerraformExtraArguments(result, block.Block.Body, block.Block)
		}

		if cfg.NoDeprecatedFields {
			checkTerraformDeprecatedFields(result, block.Block.Body, block.Block)
		}
	}
}

func checkTerraformSourceRequired(result *Result, attrs map[string]hcl.Expression, block *hclsyntax.Block) {
	if _, ok := attrs["source"]; !ok {
		result.Issues = append(result.Issues, Issue{
			Severity: SeverityError,
			Rule:     "terraform_source_required",
			Message:  "terraform block must have 'source' attribute",
			Location: block.TypeRange,
		})
	}
}

func checkTerraformVersionFormat(result *Result, attrs map[string]hcl.Expression, _ *hclsyntax.Block) {
	if version, ok := attrs["version"]; ok {
		versionStr := getStringValue(version)
		if versionStr != "" && !isValidTerraformVersion(versionStr) {
			result.Issues = append(result.Issues, Issue{
				Severity: SeverityWarning,
				Rule:     "terraform_version_format",
				Message:  fmt.Sprintf("terraform version %q may not match expected format (e.g., >= 1.0.0)", versionStr),
				Location: version.Range(),
			})
		}
	}

	if requiredVersion, ok := attrs["required_version"]; ok {
		versionStr := getStringValue(requiredVersion)
		if versionStr != "" && !isValidTerraformVersionConstraint(versionStr) {
			result.Issues = append(result.Issues, Issue{
				Severity: SeverityWarning,
				Rule:     "terraform_version_format",
				Message:  fmt.Sprintf("terraform required_version %q may not match expected format (e.g., >= 1.0.0, < 2.0.0)", versionStr),
				Location: requiredVersion.Range(),
			})
		}
	}
}

var (
	terraformVersionRegex    = regexp.MustCompile(`^v?\d+\.\d+(\.\d+)?$`)
	terraformConstraintRegex = regexp.MustCompile(`^(>=|<=|>|<|~>|!=|==)?\s*v?\d+\.\d+(\.\d+)?`)
)

func isValidTerraformVersion(version string) bool {
	return terraformVersionRegex.MatchString(version)
}

func isValidTerraformVersionConstraint(constraint string) bool {
	parts := strings.Split(constraint, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if !terraformConstraintRegex.MatchString(part) {
			return false
		}
	}
	return true
}

func checkTerraformExtraArguments(result *Result, body hcl.Body, _ *hclsyntax.Block) {
	var extraArgsBlocks []*hcl.Block

	schemaWithLabel := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "extra_arguments", LabelNames: []string{"name"}},
		},
	}
	content1, _, _ := body.PartialContent(schemaWithLabel)
	extraArgsBlocks = append(extraArgsBlocks, content1.Blocks...)

	schemaNoLabel := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "extra_arguments"},
		},
	}
	content2, _, _ := body.PartialContent(schemaNoLabel)
	extraArgsBlocks = append(extraArgsBlocks, content2.Blocks...)

	seen := make(map[string]bool)
	for _, extraBlock := range extraArgsBlocks {
		rangeKey := extraBlock.TypeRange.String()
		if seen[rangeKey] {
			continue
		}
		seen[rangeKey] = true

		attrs := ast.GetBlockAttributes(extraBlock.Body)

		hasName := len(extraBlock.Labels) > 0
		if nameAttr, ok := attrs["name"]; ok {
			nameStr := getStringValue(nameAttr)
			if nameStr != "" {
				hasName = true
			}
		}
		if !hasName {
			result.Issues = append(result.Issues, Issue{
				Severity: SeverityWarning,
				Rule:     "terraform_extra_arguments_valid",
				Message:  "extra_arguments block should have a non-empty 'name' attribute",
				Location: extraBlock.TypeRange,
			})
		}

		_, hasArguments := attrs["arguments"]
		var hasNestedBlocks bool
		if sibBody, ok := extraBlock.Body.(*hclsyntax.Body); ok {
			hasNestedBlocks = len(sibBody.Blocks) > 0
		}

		if !hasArguments && !hasNestedBlocks {
			result.Issues = append(result.Issues, Issue{
				Severity: SeverityWarning,
				Rule:     "terraform_extra_arguments_valid",
				Message:  "extra_arguments block should have 'arguments' or nested blocks",
				Location: extraBlock.TypeRange,
			})
		}
	}
}

var terraformDeprecatedFields = map[string]string{
	"terraform":   "Use 'source' instead",
	"before_hook": "Use 'before_hooks' (plural) instead",
	"after_hook":  "Use 'after_hooks' (plural) instead",
}

func checkTerraformDeprecatedFields(result *Result, body hcl.Body, _ *hclsyntax.Block) {
	attrs, _ := body.JustAttributes()
	for name := range attrs {
		if msg, ok := terraformDeprecatedFields[name]; ok {
			result.Issues = append(result.Issues, Issue{
				Severity: SeverityWarning,
				Rule:     "terraform_deprecated_fields",
				Message:  fmt.Sprintf("field %q is deprecated: %s", name, msg),
				Location: attrs[name].Expr.Range(),
			})
		}
	}

	nestedBlocks := ast.GetBlockNestedBlocks(body, "before_hook")
	for _, nestedBlock := range nestedBlocks {
		result.Issues = append(result.Issues, Issue{
			Severity: SeverityWarning,
			Rule:     "terraform_deprecated_fields",
			Message:  "block 'before_hook' is deprecated: use 'before_hooks' (plural) instead",
			Location: nestedBlock.TypeRange,
		})
	}

	nestedBlocks = ast.GetBlockNestedBlocks(body, "after_hook")
	for _, nestedBlock := range nestedBlocks {
		result.Issues = append(result.Issues, Issue{
			Severity: SeverityWarning,
			Rule:     "terraform_deprecated_fields",
			Message:  "block 'after_hook' is deprecated: use 'after_hooks' (plural) instead",
			Location: nestedBlock.TypeRange,
		})
	}

	if _, ok := attrs["terraform"]; ok {
		result.Issues = append(result.Issues, Issue{
			Severity: SeverityWarning,
			Rule:     "terraform_deprecated_fields",
			Message:  "field 'terraform' is deprecated: Use 'source' instead",
			Location: attrs["terraform"].Expr.Range(),
		})
	}

	nestedTerraformBlocks := ast.GetBlockNestedBlocks(body, "terraform")
	for _, nestedBlock := range nestedTerraformBlocks {
		result.Issues = append(result.Issues, Issue{
			Severity: SeverityWarning,
			Rule:     "terraform_deprecated_fields",
			Message:  "block 'terraform' is deprecated: Use 'source' instead",
			Location: nestedBlock.TypeRange,
		})
	}
}
