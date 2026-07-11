package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

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

	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(data, absPath)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to parse HCL config %s: %w", absPath, diags)
	}

	syntaxBody, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return &Rules{}, nil
	}

	rules := &Rules{}

	if s, ok := attrString(syntaxBody, "extends"); ok {
		basePath := resolveExtendsPath(absPath, s)
		baseRules, baseErr := loadHCLConfigWithVisited(basePath, visited)
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
