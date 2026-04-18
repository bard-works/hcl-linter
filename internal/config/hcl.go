package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func loadHCLConfig(path string) (*Rules, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %s: %w", path, err)
	}

	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(data, path)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to parse HCL config %s: %w", path, diags)
	}

	syntaxBody, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return &Rules{}, nil
	}

	rules := &Rules{}

	for _, block := range syntaxBody.Blocks {
		if block.Type != "rules" {
			continue
		}

		parseHCLRulesBlock(block.Body, rules)
	}

	return rules, nil
}

func parseHCLRulesBlock(body *hclsyntax.Body, rules *Rules) {
	for _, block := range body.Blocks {
		switch block.Type {
		case "block_order":
			rules.BlockOrder = parseHCLBlockOrder(block.Body)
		case "array_format":
			rules.ArrayFormat = parseHCLArrayFormat(block.Body)
		case "blank_lines":
			rules.BlankLines = parseHCLBlankLines(block.Body)
		case "name_validation":
			rules.NameValidation = parseHCLNameValidation(block.Body)
		case "duplicates":
			rules.Duplicates = parseHCLDuplicates(block.Body)
		case "required_fields":
			rules.RequiredFields = parseHCLRequiredFields(block.Body)
		case "required_blocks":
			rules.RequiredBlocks = parseHCLRequiredBlocks(block.Body)
		case "terragrunt":
			rules.Terragrunt = parseHCLTerragrunt(block.Body)
		case "terragrunt_functions":
			rules.TerragruntFunctions = parseHCLTerragruntFunctions(block.Body)
		case "terraform_block":
			rules.TerraformBlock = parseHCLTerraformBlock(block.Body)
		case "key_value":
			rules.KeyValue = parseHCLKeyValue(block.Body)
		case "count_for_each":
			rules.CountForEach = parseHCLCountForEach(block.Body)
		case "dependency_outputs":
			rules.DependencyOutputs = parseHCLDependencyOutputs(block.Body)
		}
	}

	if attr, ok := body.Attributes["max_concurrency"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			f, _ := val.AsBigFloat().Float64()
			rules.MaxConcurrency = int(f)
		}
	}
}

func parseHCLBlockOrder(body *hclsyntax.Body) *BlockOrderConfig {
	cfg := &BlockOrderConfig{}
	if attr, ok := body.Attributes["enabled"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.Enabled = val.True()
		}
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
	if attr, ok := body.Attributes["enabled"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.Enabled = val.True()
		}
	}
	if attr, ok := body.Attributes["multiline_threshold"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			f, _ := val.AsBigFloat().Float64()
			cfg.MultilineThreshold = int(f)
		}
	}
	return cfg
}

func parseHCLBlankLines(body *hclsyntax.Body) *BlankLinesConfig {
	cfg := &BlankLinesConfig{}
	if attr, ok := body.Attributes["enabled"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.Enabled = val.True()
		}
	}
	if attr, ok := body.Attributes["within_blocks"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.WithinBlocks = val.True()
		}
	}
	return cfg
}

func parseHCLNameValidation(body *hclsyntax.Body) *NameValidationConfig {
	cfg := &NameValidationConfig{}
	if attr, ok := body.Attributes["enabled"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.Enabled = val.True()
		}
	}
	if attr, ok := body.Attributes["pattern"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.Pattern = val.AsString()
		}
	}
	if attr, ok := body.Attributes["blocks"]; ok {
		cfg.Blocks = hclExprToStringSlice(attr.Expr)
	}
	return cfg
}

func parseHCLDuplicates(body *hclsyntax.Body) *DuplicatesConfig {
	cfg := &DuplicatesConfig{}
	if attr, ok := body.Attributes["enabled"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.Enabled = val.True()
		}
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
			if attr, ok := block.Body.Attributes["expose"]; ok {
				if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
					cfg.Include.Expose = val.True()
				}
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
		if attr, ok := block.Body.Attributes["type"]; ok {
			if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
				spec.Type = val.AsString()
			}
		}
		if attr, ok := block.Body.Attributes["count"]; ok {
			if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
				spec.Count = val.AsString()
			}
		}
		if attr, ok := block.Body.Attributes["error"]; ok {
			if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
				spec.Error = val.AsString()
			}
		}
		cfg.Required = append(cfg.Required, spec)
	}
	return cfg
}

func parseHCLTerragruntFunctions(body *hclsyntax.Body) *TerragruntFunctionsConfig {
	cfg := &TerragruntFunctionsConfig{}
	if attr, ok := body.Attributes["enabled"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.Enabled = val.True()
		}
	}
	if attr, ok := body.Attributes["find_in_parent_folders_exists"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.FindInParentFoldersExists = val.True()
		}
	}
	if attr, ok := body.Attributes["get_env_has_default"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.GetEnvHasDefault = val.True()
		}
	}
	return cfg
}

func parseHCLTerraformBlock(body *hclsyntax.Body) *TerraformBlockConfig {
	cfg := &TerraformBlockConfig{}
	if attr, ok := body.Attributes["enabled"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.Enabled = val.True()
		}
	}
	if attr, ok := body.Attributes["source_required"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.SourceRequired = val.True()
		}
	}
	if attr, ok := body.Attributes["version_format"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.VersionFormat = val.True()
		}
	}
	if attr, ok := body.Attributes["extra_arguments_valid"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.ExtraArgumentsValid = val.True()
		}
	}
	if attr, ok := body.Attributes["no_deprecated_fields"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.NoDeprecatedFields = val.True()
		}
	}
	return cfg
}

func parseHCLTerragrunt(body *hclsyntax.Body) *TerragruntConfig {
	cfg := &TerragruntConfig{}
	if attr, ok := body.Attributes["enabled"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.Enabled = val.True()
		}
	}
	if attr, ok := body.Attributes["dependency_path_exists"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.DependencyPathExists = val.True()
		}
	}
	if attr, ok := body.Attributes["include_path_exists"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.IncludePathExists = val.True()
		}
	}
	if attr, ok := body.Attributes["remote_state_config"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.RemoteStateConfig = val.True()
		}
	}
	return cfg
}

func parseHCLKeyValue(body *hclsyntax.Body) *KeyValueConfig {
	cfg := &KeyValueConfig{}
	if attr, ok := body.Attributes["enabled"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.Enabled = val.True()
		}
	}
	if attr, ok := body.Attributes["key_case"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.KeyCase = val.AsString()
		}
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
	result := make(map[string]string)
	val, diags := expr.Value(nil)
	if diags.HasErrors() {
		return result
	}
	obj := val.AsValueMap()
	for key, v := range obj {
		result[key] = v.AsString()
	}
	return result
}

func parseHCLCountForEach(body *hclsyntax.Body) *CountForEachConfig {
	cfg := &CountForEachConfig{}
	if attr, ok := body.Attributes["enabled"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.Enabled = val.True()
		}
	}
	if attr, ok := body.Attributes["warn_on_count_zero"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.WarnOnCountZero = val.True()
		}
	}
	if attr, ok := body.Attributes["warn_on_empty_for_each"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.WarnOnEmptyForEach = val.True()
		}
	}
	if attr, ok := body.Attributes["warn_on_conflict"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.WarnOnConflict = val.True()
		}
	}
	return cfg
}

func parseHCLDependencyOutputs(body *hclsyntax.Body) *DependencyOutputsConfig {
	cfg := &DependencyOutputsConfig{}
	if attr, ok := body.Attributes["enabled"]; ok {
		if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			cfg.Enabled = val.True()
		}
	}
	return cfg
}

func hclExprToStringSlice(expr hclsyntax.Expression) []string {
	val, diags := expr.Value(nil)
	if diags.HasErrors() {
		return nil
	}
	arr := val.AsValueSlice()
	result := make([]string, 0, len(arr))
	for _, v := range arr {
		result = append(result, v.AsString())
	}
	return result
}

func parseNestedOrderAttribute(expr hclsyntax.Expression) map[string][]string {
	result := make(map[string][]string)
	val, diags := expr.Value(nil)
	if diags.HasErrors() {
		return result
	}
	obj := val.AsValueMap()
	for key, v := range obj {
		arr := v.AsValueSlice()
		var order []string
		for _, item := range arr {
			s := item.AsString()
			order = append(order, s)
		}
		result[key] = order
	}
	return result
}

func LoadConfigDir(configDir string) (*Loader, error) {
	if configDir == "" {
		loader, result := NewLoaderWithDiscovery("")
		if result.Source == ConfigSourceNone {
			return nil, errors.New("config directory does not exist")
		}
		return loader, nil
	}

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("config directory does not exist: %s", configDir)
	}
	return NewLoader(configDir), nil
}

func LoadConfigDirWithResult(configDir string) (*Loader, *ConfigResult) {
	return NewLoaderWithDiscovery(configDir)
}
