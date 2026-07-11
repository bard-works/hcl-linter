package config

import "github.com/hashicorp/hcl/v2/hclsyntax"

func parseHCLBlockOrder(body *hclsyntax.Body) *BlockOrderConfig {
	cfg := &BlockOrderConfig{}
	if b, ok := attrBool(body, "enabled"); ok {
		cfg.Enabled = b
	}
	if attr, ok := body.Attributes["order"]; ok {
		cfg.Order = hclExprToStringSlice(attr.Expr)
	}
	if attr, ok := body.Attributes["nested_order"]; ok {
		cfg.NestedOrder = parseNestedOrderAttribute(attr.Expr)
	}
	return cfg
}

func parseHCLArrayFormat(body *hclsyntax.Body) *ArrayFormatConfig {
	cfg := &ArrayFormatConfig{}
	if b, ok := attrBool(body, "enabled"); ok {
		cfg.Enabled = b
	}
	if n, ok := attrInt(body, "multiline_threshold"); ok {
		cfg.MultilineThreshold = n
	}
	if b, ok := attrBool(body, "sort"); ok {
		cfg.Sort = b
	}
	return cfg
}

func parseHCLBlankLines(body *hclsyntax.Body) *BlankLinesConfig {
	cfg := &BlankLinesConfig{}
	if b, ok := attrBool(body, "enabled"); ok {
		cfg.Enabled = b
	}
	if b, ok := attrBool(body, "within_blocks"); ok {
		cfg.WithinBlocks = b
	}
	return cfg
}

func parseHCLNameValidation(body *hclsyntax.Body) *NameValidationConfig {
	cfg := &NameValidationConfig{}
	if b, ok := attrBool(body, "enabled"); ok {
		cfg.Enabled = b
	}
	if s, ok := attrString(body, "pattern"); ok {
		cfg.Pattern = s
	}
	if attr, ok := body.Attributes["blocks"]; ok {
		cfg.Blocks = hclExprToStringSlice(attr.Expr)
	}
	return cfg
}

func parseHCLDuplicates(body *hclsyntax.Body) *DuplicatesConfig {
	cfg := &DuplicatesConfig{}
	if b, ok := attrBool(body, "enabled"); ok {
		cfg.Enabled = b
	}
	if attr, ok := body.Attributes["blocks"]; ok {
		cfg.Blocks = hclExprToStringSlice(attr.Expr)
	}
	return cfg
}

func parseHCLRequiredFields(body *hclsyntax.Body) *RequiredFieldsConfig {
	cfg := &RequiredFieldsConfig{}
	for _, block := range body.Blocks {
		if block.Type == "include" {
			cfg.Include = &IncludeRequired{}
			if b, ok := attrBool(block.Body, "expose"); ok {
				cfg.Include.Expose = b
			}
		}
	}
	return cfg
}

func parseHCLRequiredBlocks(body *hclsyntax.Body) *RequiredBlocksConfig {
	cfg := &RequiredBlocksConfig{}
	for _, block := range body.Blocks {
		if block.Type != "required" {
			continue
		}
		spec := RequiredBlockSpec{}
		if s, ok := attrString(block.Body, "type"); ok {
			spec.Type = s
		}
		if s, ok := attrString(block.Body, "count"); ok {
			spec.Count = s
		}
		if s, ok := attrString(block.Body, "error"); ok {
			spec.Error = s
		}
		cfg.Required = append(cfg.Required, spec)
	}
	return cfg
}

func parseHCLFunctions(body *hclsyntax.Body) *HCLFunctionsConfig {
	cfg := &HCLFunctionsConfig{}
	if b, ok := attrBool(body, "enabled"); ok {
		cfg.Enabled = b
	}
	if b, ok := attrBool(body, "find_in_parent_folders_exists"); ok {
		cfg.FindInParentFoldersExists = b
	}
	if b, ok := attrBool(body, "get_env_has_default"); ok {
		cfg.GetEnvHasDefault = b
	}
	return cfg
}

func hclExprToStringSlice(expr hclsyntax.Expression) []string {
	val, diags := expr.Value(nil)
	if diags.HasErrors() {
		return nil
	}
	ss, _ := ctyStringSlice(val)
	return ss
}

func parseNestedOrderAttribute(expr hclsyntax.Expression) map[string][]string {
	val, diags := expr.Value(nil)
	if diags.HasErrors() {
		return make(map[string][]string)
	}
	if m, ok := ctyStringSliceMap(val); ok {
		return m
	}
	return make(map[string][]string)
}
