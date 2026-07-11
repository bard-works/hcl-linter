package config

import "github.com/hashicorp/hcl/v2/hclsyntax"

func parseHCLTerraformBlock(body *hclsyntax.Body) *TerraformBlockConfig {
	cfg := &TerraformBlockConfig{}
	if b, ok := attrBool(body, "enabled"); ok {
		cfg.Enabled = b
	}
	if b, ok := attrBool(body, "source_required"); ok {
		cfg.SourceRequired = b
	}
	if b, ok := attrBool(body, "version_format"); ok {
		cfg.VersionFormat = b
	}
	if b, ok := attrBool(body, "extra_arguments_valid"); ok {
		cfg.ExtraArgumentsValid = b
	}
	if b, ok := attrBool(body, "no_deprecated_fields"); ok {
		cfg.NoDeprecatedFields = b
	}
	return cfg
}

func parseHCLDependencyPaths(body *hclsyntax.Body) *DependencyPathsConfig {
	cfg := &DependencyPathsConfig{}
	if b, ok := attrBool(body, "enabled"); ok {
		cfg.Enabled = b
	}
	return cfg
}

func parseHCLIncludePaths(body *hclsyntax.Body) *IncludePathsConfig {
	cfg := &IncludePathsConfig{}
	if b, ok := attrBool(body, "enabled"); ok {
		cfg.Enabled = b
	}
	return cfg
}

func parseHCLRemoteState(body *hclsyntax.Body) *RemoteStateConfig {
	cfg := &RemoteStateConfig{}
	if b, ok := attrBool(body, "enabled"); ok {
		cfg.Enabled = b
	}
	if b, ok := attrBool(body, "require_backend"); ok {
		cfg.RequireBackend = b
	}
	return cfg
}

func parseHCLKeyValue(body *hclsyntax.Body) *KeyValueConfig {
	cfg := &KeyValueConfig{}
	if b, ok := attrBool(body, "enabled"); ok {
		cfg.Enabled = b
	}
	if s, ok := attrString(body, "key_case"); ok {
		cfg.KeyCase = s
	}
	if attr, ok := body.Attributes["disallowed"]; ok {
		cfg.Disallowed = hclExprToStringSlice(attr.Expr)
	}
	if attr, ok := body.Attributes["value_pattern"]; ok {
		cfg.ValuePattern = parseValuePatternAttribute(attr.Expr)
	}
	return cfg
}

func parseValuePatternAttribute(expr hclsyntax.Expression) map[string]string {
	val, diags := expr.Value(nil)
	if diags.HasErrors() {
		return make(map[string]string)
	}
	if m, ok := ctyStringMap(val); ok {
		return m
	}
	return make(map[string]string)
}

func parseHCLCountForEach(body *hclsyntax.Body) *CountForEachConfig {
	cfg := &CountForEachConfig{}
	if b, ok := attrBool(body, "enabled"); ok {
		cfg.Enabled = b
	}
	if b, ok := attrBool(body, "warn_on_count_zero"); ok {
		cfg.WarnOnCountZero = b
	}
	if b, ok := attrBool(body, "warn_on_empty_for_each"); ok {
		cfg.WarnOnEmptyForEach = b
	}
	if b, ok := attrBool(body, "warn_on_conflict"); ok {
		cfg.WarnOnConflict = b
	}
	return cfg
}

func parseHCLDependencyOutputs(body *hclsyntax.Body) *DependencyOutputsConfig {
	cfg := &DependencyOutputsConfig{}
	if b, ok := attrBool(body, "enabled"); ok {
		cfg.Enabled = b
	}
	return cfg
}
