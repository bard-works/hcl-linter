package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func loadHCLConfig(path string) (*Rules, error) {
	return loadHCLConfigWithVisited(path, map[string]bool{})
}

func loadHCLConfigWithVisited(path string, visited map[string]bool) (*Rules, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path %s: %w", path, err)
	}
	if visited[absPath] {
		return nil, fmt.Errorf("circular extends reference detected: %s", absPath)
	}
	visited[absPath] = true

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %s: %w", absPath, err)
	}
	return parseHCLConfig(data, absPath, visited)
}

// parseHCLConfig parses config source. name is the file path for on-disk
// configs (relative extends resolve against its directory) or "builtin:<x>"
// for presets.
func parseHCLConfig(data []byte, name string, visited map[string]bool) (*Rules, error) {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(data, name)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to parse HCL config %s: %w", name, diags)
	}

	syntaxBody, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return &Rules{}, nil
	}

	rules := &Rules{}

	if s, ok := attrString(syntaxBody, "extends"); ok {
		baseRules, baseErr := loadExtends(name, s, visited)
		if baseErr != nil {
			return nil, fmt.Errorf("failed to load extended config %q: %w", s, baseErr)
		}
		*rules = *baseRules
	}

	for _, block := range syntaxBody.Blocks {
		if block.Type != "rules" {
			continue
		}

		parseHCLRulesBlock(block.Body, rules)
	}

	return rules, nil
}

// loadExtends resolves an extends reference: a built-in preset
// ("builtin:terragrunt") or a config file relative to the current one.
func loadExtends(current, ref string, visited map[string]bool) (*Rules, error) {
	if preset, ok := strings.CutPrefix(ref, builtinPrefix); ok {
		key := builtinPrefix + preset
		if visited[key] {
			return nil, fmt.Errorf("circular extends reference detected: %s", key)
		}
		visited[key] = true
		data, err := presetSource(preset)
		if err != nil {
			return nil, err
		}
		return parseHCLConfig(data, key, visited)
	}
	if strings.HasPrefix(current, builtinPrefix) {
		return nil, fmt.Errorf("preset %s may only extend other builtin: presets", current)
	}
	return loadHCLConfigWithVisited(resolveExtendsPath(current, ref), visited)
}

func resolveExtendsPath(currentPath, ref string) string {
	dir := filepath.Dir(currentPath)
	if filepath.Ext(ref) == "" {
		ref += ".hcl"
	}
	return filepath.Join(dir, ref)
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
		case "dependency_paths":
			rules.DependencyPaths = parseHCLDependencyPaths(block.Body)
		case "include_paths":
			rules.IncludePaths = parseHCLIncludePaths(block.Body)
		case "remote_state":
			rules.RemoteState = parseHCLRemoteState(block.Body)
		case "hcl_functions":
			rules.HCLFunctions = parseHCLFunctions(block.Body)
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

	if n, ok := attrInt(body, "max_concurrency"); ok {
		rules.MaxConcurrency = n
	}
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
