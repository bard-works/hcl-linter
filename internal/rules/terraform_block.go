package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/diag"
)

type TerraformBlockRule struct{}

func (r TerraformBlockRule) Name() string  { return "terraform_block" }
func (r TerraformBlockRule) Priority() int { return PrioritySemantic }

func init() { Register(TerraformBlockRule{}) }

func (r TerraformBlockRule) Doc() RuleDoc {
	return RuleDoc{
		Summary:     "Validates the terraform block for required source, version format, and deprecated fields.",
		Severity:    "error",
		Fixable:     false,
		ConfigBlock: "terraform_block",
		ConfigFields: []ConfigField{
			{Name: "enabled", Type: "bool", Required: true, Doc: "Activate the rule"},
			{
				Name:     "source_required",
				Type:     "bool",
				Required: false,
				Default:  "false",
				Doc:      "Require the source attribute",
			},
			{
				Name:     "version_format",
				Type:     "bool",
				Required: false,
				Default:  "false",
				Doc:      "Validate required_version constraint syntax",
			},
			{
				Name:     "extra_arguments_valid",
				Type:     "bool",
				Required: false,
				Default:  "false",
				Doc:      "Validate extra_arguments block structure",
			},
			{
				Name:     "no_deprecated_fields",
				Type:     "bool",
				Required: false,
				Default:  "false",
				Doc:      "Warn on deprecated attributes",
			},
		},
		Example: Example{
			Violation: `terraform {
  source = ""
}`,
		},
	}
}

func (r TerraformBlockRule) Enabled(cfg *config.Rules) bool {
	return cfg != nil && cfg.TerraformBlock != nil && cfg.TerraformBlock.Enabled
}

func (r TerraformBlockRule) Check(ctx *Context) []diag.Issue {
	cfg := ctx.Config.TerraformBlock
	var issues []diag.Issue

	for _, block := range ctx.Blocks {
		if block.Type != "terraform" {
			continue
		}

		attrs := ast.GetBlockAttributes(block.Block.Body)

		if cfg.SourceRequired {
			checkTFSourceRequired(&issues, attrs, block.Block)
		}
		if cfg.VersionFormat {
			checkTFVersionFormat(&issues, attrs)
		}
		if cfg.ExtraArgumentsValid {
			checkTFExtraArguments(&issues, block.Block.Body)
		}
		if cfg.NoDeprecatedFields {
			checkTFDeprecatedFields(&issues, block.Block.Body)
		}
	}

	return issues
}

func checkTFSourceRequired(issues *[]diag.Issue, attrs map[string]hcl.Expression, block *hclsyntax.Block) {
	if _, ok := attrs["source"]; !ok {
		*issues = append(*issues, diag.Issue{
			Severity: diag.SeverityError,
			Rule:     "terraform_source_required",
			Message:  "terraform block must have 'source' attribute",
			Location: block.TypeRange,
		})
	}
}

func checkTFVersionFormat(issues *[]diag.Issue, attrs map[string]hcl.Expression) {
	if version, ok := attrs["version"]; ok {
		if v := hclStringValue(version); v != "" && !tfVersionValid(v) {
			*issues = append(*issues, diag.Issue{
				Severity: diag.SeverityWarning,
				Rule:     "terraform_version_format",
				Message:  fmt.Sprintf("terraform version %q may not match expected format (e.g., >= 1.0.0)", v),
				Location: version.Range(),
			})
		}
	}

	if rv, ok := attrs["required_version"]; ok {
		if v := hclStringValue(rv); v != "" && !tfConstraintValid(v) {
			*issues = append(*issues, diag.Issue{
				Severity: diag.SeverityWarning,
				Rule:     "terraform_version_format",
				Message: fmt.Sprintf(
					"terraform required_version %q may not match expected format (e.g., >= 1.0.0, < 2.0.0)",
					v,
				),
				Location: rv.Range(),
			})
		}
	}
}

var (
	tfVersionRe    = regexp.MustCompile(`^v?\d+\.\d+(\.\d+)?$`)
	tfConstraintRe = regexp.MustCompile(`^(>=|<=|>|<|~>|!=|==)?\s*v?\d+\.\d+(\.\d+)?`)
)

func tfVersionValid(version string) bool {
	return tfVersionRe.MatchString(version)
}

func tfConstraintValid(constraint string) bool {
	for _, part := range strings.Split(constraint, ",") {
		if !tfConstraintRe.MatchString(strings.TrimSpace(part)) {
			return false
		}
	}
	return true
}

func checkTFExtraArguments(issues *[]diag.Issue, body hcl.Body) {
	var extraArgsBlocks []*hcl.Block

	s1 := &hcl.BodySchema{Blocks: []hcl.BlockHeaderSchema{{Type: "extra_arguments", LabelNames: []string{"name"}}}}
	c1, _, _ := body.PartialContent(s1)
	extraArgsBlocks = append(extraArgsBlocks, c1.Blocks...)

	s2 := &hcl.BodySchema{Blocks: []hcl.BlockHeaderSchema{{Type: "extra_arguments"}}}
	c2, _, _ := body.PartialContent(s2)
	extraArgsBlocks = append(extraArgsBlocks, c2.Blocks...)

	seen := make(map[string]bool)
	for _, b := range extraArgsBlocks {
		key := b.TypeRange.String()
		if seen[key] {
			continue
		}
		seen[key] = true

		attrs := ast.GetBlockAttributes(b.Body)

		hasName := len(b.Labels) > 0
		if nameAttr, ok := attrs["name"]; ok {
			if hclStringValue(nameAttr) != "" {
				hasName = true
			}
		}
		if !hasName {
			*issues = append(*issues, diag.Issue{
				Severity: diag.SeverityWarning,
				Rule:     "terraform_extra_arguments_valid",
				Message:  "extra_arguments block should have a non-empty 'name' attribute",
				Location: b.TypeRange,
			})
		}

		_, hasArguments := attrs["arguments"]
		var hasNestedBlocks bool
		if sb, ok := b.Body.(*hclsyntax.Body); ok {
			hasNestedBlocks = len(sb.Blocks) > 0
		}
		if !hasArguments && !hasNestedBlocks {
			*issues = append(*issues, diag.Issue{
				Severity: diag.SeverityWarning,
				Rule:     "terraform_extra_arguments_valid",
				Message:  "extra_arguments block should have 'arguments' or nested blocks",
				Location: b.TypeRange,
			})
		}
	}
}

var tfDeprecatedFields = map[string]string{
	"terraform":   "Use 'source' instead",
	"before_hook": "Use 'before_hooks' (plural) instead",
	"after_hook":  "Use 'after_hooks' (plural) instead",
}

func checkTFDeprecatedFields(issues *[]diag.Issue, body hcl.Body) {
	attrs := ast.GetBodyAttributes(body)
	for name, attr := range attrs {
		if msg, ok := tfDeprecatedFields[name]; ok {
			*issues = append(*issues, diag.Issue{
				Severity: diag.SeverityWarning,
				Rule:     "terraform_deprecated_fields",
				Message:  fmt.Sprintf("field %q is deprecated: %s", name, msg),
				Location: attr.Expr.Range(),
			})
		}
	}

	for _, nb := range ast.GetBlockNestedBlocks(body, "before_hook") {
		*issues = append(*issues, diag.Issue{
			Severity: diag.SeverityWarning,
			Rule:     "terraform_deprecated_fields",
			Message:  "block 'before_hook' is deprecated: use 'before_hooks' (plural) instead",
			Location: nb.TypeRange,
		})
	}
	for _, nb := range ast.GetBlockNestedBlocks(body, "after_hook") {
		*issues = append(*issues, diag.Issue{
			Severity: diag.SeverityWarning,
			Rule:     "terraform_deprecated_fields",
			Message:  "block 'after_hook' is deprecated: use 'after_hooks' (plural) instead",
			Location: nb.TypeRange,
		})
	}

	for _, nb := range ast.GetBlockNestedBlocks(body, "terraform") {
		*issues = append(*issues, diag.Issue{
			Severity: diag.SeverityWarning,
			Rule:     "terraform_deprecated_fields",
			Message:  "block 'terraform' is deprecated: Use 'source' instead",
			Location: nb.TypeRange,
		})
	}
}
