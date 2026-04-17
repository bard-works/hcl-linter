package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type ConfigSource int

const (
	ConfigSourceNone ConfigSource = iota
	ConfigSourceProject
	ConfigSourceHome
	ConfigSourceCwd
	ConfigSourceEnv
	ConfigSourceCLI
)

func (s ConfigSource) String() string {
	switch s {
	case ConfigSourceProject:
		return "project"
	case ConfigSourceHome:
		return "~/.hcl-linter"
	case ConfigSourceCwd:
		return ".hcl-linter"
	case ConfigSourceEnv:
		return "HCL_LINTER_CONFIG_DIR"
	case ConfigSourceCLI:
		return "--config-source"
	default:
		return "none"
	}
}

type ConfigResult struct {
	Rules      *Rules
	Source     ConfigSource
	SourcePath string
	WarningMsg string
}

type Config struct {
	Rules Rules `json:"rules"`
}

type BlankLinesConfig struct {
	Enabled      bool `json:"enabled"`
	WithinBlocks bool `json:"within_blocks"`
}

type Rules struct {
	BlockOrder     *BlockOrderConfig     `json:"block_order,omitempty"`
	ArrayFormat    *ArrayFormatConfig    `json:"array_format,omitempty"`
	NameValidation *NameValidationConfig `json:"name_validation,omitempty"`
	Duplicates     *DuplicatesConfig     `json:"duplicates,omitempty"`
	RequiredFields *RequiredFieldsConfig `json:"required_fields,omitempty"`
	BlankLines     *BlankLinesConfig     `json:"blank_lines,omitempty"`
	RequiredBlocks *RequiredBlocksConfig `json:"required_blocks,omitempty"`
	MaxConcurrency int                   `json:"max_concurrency,omitempty"`
}

const EnvMaxConcurrency = "HCL_LINTER_MAX_CONCURRENCY"

func GetMaxConcurrency(cfg *Rules) int {
	if envVal := os.Getenv(EnvMaxConcurrency); envVal != "" {
		if val, err := strconv.Atoi(envVal); err == nil && val > 0 {
			return val
		}
	}
	if cfg != nil && cfg.MaxConcurrency > 0 {
		return cfg.MaxConcurrency
	}
	return runtime.NumCPU()
}

type BlockOrderConfig struct {
	Enabled bool     `json:"enabled"`
	Order   []string `json:"order"`
}

type ArrayFormatConfig struct {
	Enabled            bool `json:"enabled"`
	MultilineThreshold int  `json:"multiline_threshold"`
}

type NameValidationConfig struct {
	Enabled bool     `json:"enabled"`
	Pattern string   `json:"pattern"`
	Blocks  []string `json:"blocks"`
}

type DuplicatesConfig struct {
	Enabled bool     `json:"enabled"`
	Blocks  []string `json:"blocks"`
}

type RequiredFieldsConfig struct {
	Include *IncludeRequired `json:"include,omitempty"`
}

type IncludeRequired struct {
	Expose bool `json:"expose"`
}

type RequiredBlocksConfig struct {
	Required []RequiredBlockSpec `json:"required"`
}

type RequiredBlockSpec struct {
	Type  string `json:"type"`
	Count string `json:"count"`
	Error string `json:"error"`
}

func (r *Rules) IsEnabled() bool {
	return r.BlockOrder != nil || r.ArrayFormat != nil ||
		r.NameValidation != nil || r.Duplicates != nil || r.RequiredFields != nil ||
		r.BlankLines != nil || r.RequiredBlocks != nil
}

type Loader struct {
	configDir string
}

func NewLoader(configDir string) *Loader {
	return &Loader{
		configDir: configDir,
	}
}

func getProjectConfigDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exeDir := filepath.Dir(exe)
	return filepath.Join(exeDir, ".hcl-linter"), nil
}

func getHomeConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".hcl-linter"), nil
}

func getCwdConfigDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, ".hcl-linter"), nil
}

func findConfigDir(loader *Loader) *ConfigResult {
	if loader.configDir != "" {
		configDir := loader.configDir
		if !strings.HasSuffix(configDir, ".hcl-linter") && !strings.HasSuffix(configDir, "/.hcl-linter") {
			configDir = filepath.Join(configDir, ".hcl-linter")
		}
		if _, err := os.Stat(configDir); err == nil {
			return &ConfigResult{
				Source:     ConfigSourceCLI,
				SourcePath: configDir,
			}
		}
		if _, err := os.Stat(loader.configDir); err == nil {
			return &ConfigResult{
				Source:     ConfigSourceCLI,
				SourcePath: loader.configDir,
			}
		}
	}

	envDir := os.Getenv("HCL_LINTER_CONFIG_DIR")
	if envDir != "" {
		configDir := envDir
		if !strings.HasSuffix(configDir, ".hcl-linter") && !strings.HasSuffix(configDir, "/.hcl-linter") {
			configDir = filepath.Join(envDir, ".hcl-linter")
		}
		if _, err := os.Stat(configDir); err == nil {
			return &ConfigResult{
				Source:     ConfigSourceEnv,
				SourcePath: configDir,
			}
		}
		if _, err := os.Stat(envDir); err == nil {
			return &ConfigResult{
				Source:     ConfigSourceEnv,
				SourcePath: envDir,
			}
		}
	}

	cwdDir, _ := getCwdConfigDir()
	if cwdDir != "" {
		if _, err := os.Stat(cwdDir); err == nil {
			return &ConfigResult{
				Source:     ConfigSourceCwd,
				SourcePath: cwdDir,
			}
		}
	}

	homeDir, _ := getHomeConfigDir()
	if homeDir != "" {
		if _, err := os.Stat(homeDir); err == nil {
			return &ConfigResult{
				Source:     ConfigSourceHome,
				SourcePath: homeDir,
			}
		}
	}

	projectDir, _ := getProjectConfigDir()
	if projectDir != "" {
		if _, err := os.Stat(projectDir); err == nil {
			return &ConfigResult{
				Source:     ConfigSourceProject,
				SourcePath: projectDir,
				WarningMsg: "No user config found, using project defaults",
			}
		}
	}

	return &ConfigResult{
		Source:     ConfigSourceNone,
		WarningMsg: "No config directory found, using empty rules",
	}
}

func NewLoaderWithDiscovery(explicitDir string) (*Loader, *ConfigResult) {
	loader := &Loader{
		configDir: explicitDir,
	}

	result := findConfigDir(loader)
	if result.Source != ConfigSourceNone {
		loader.configDir = result.SourcePath
	}

	return loader, result
}

func (l *Loader) LoadForFile(filename string) (*Rules, error) {
	base := filepath.Base(filename)

	baseName := strings.TrimSuffix(base, filepath.Ext(base))
	for _, ext := range []string{".json", ".hcl", ""} {
		var configFile string
		if ext == "" {
			configFile = filepath.Join(l.configDir, baseName)
		} else {
			configFile = filepath.Join(l.configDir, baseName+ext)
		}
		if _, err := os.Stat(configFile); err == nil {
			rules, err := l.loadConfigFile(configFile)
			if err == nil {
				return rules, nil
			}
			if !strings.Contains(err.Error(), "unsupported config format") {
				return nil, err
			}
		}
	}

	for _, ext := range []string{".json", ".hcl", ""} {
		var defaultFile string
		if ext == "" {
			defaultFile = filepath.Join(l.configDir, "default")
		} else {
			defaultFile = filepath.Join(l.configDir, "default"+ext)
		}
		if _, err := os.Stat(defaultFile); err == nil {
			rules, err := l.loadConfigFile(defaultFile)
			if err == nil {
				return rules, nil
			}
			if !strings.Contains(err.Error(), "unsupported config format") {
				return nil, err
			}
		}
	}

	return nil, fmt.Errorf("no config found for %s and no default config", filename)
}

func (l *Loader) HasConfigForFile(filename string) bool {
	if l == nil || l.configDir == "" {
		return false
	}

	base := filepath.Base(filename)
	baseName := strings.TrimSuffix(base, filepath.Ext(base))

	for _, ext := range []string{".json", ".hcl", ""} {
		var configFile string
		if ext == "" {
			configFile = filepath.Join(l.configDir, baseName)
		} else {
			configFile = filepath.Join(l.configDir, baseName+ext)
		}
		if _, err := os.Stat(configFile); err == nil {
			return true
		}
	}

	defaultFile := filepath.Join(l.configDir, "default.json")
	if _, err := os.Stat(defaultFile); err == nil {
		return true
	}

	return false
}

func (l *Loader) HasSpecificConfigForFile(filename string) bool {
	if l == nil || l.configDir == "" {
		return false
	}

	base := filepath.Base(filename)
	baseName := strings.TrimSuffix(base, filepath.Ext(base))

	for _, ext := range []string{".json", ".hcl", ""} {
		var configFile string
		if ext == "" {
			configFile = filepath.Join(l.configDir, baseName)
		} else {
			configFile = filepath.Join(l.configDir, baseName+ext)
		}
		if _, err := os.Stat(configFile); err == nil {
			return true
		}
	}

	return false
}

func (l *Loader) loadConfigFile(path string) (*Rules, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %s: %w", path, err)
	}

	if strings.HasSuffix(path, ".json") {
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config %s: %w", path, err)
		}
		return &cfg.Rules, nil
	}

	if strings.HasSuffix(path, ".hcl") {
		return l.loadHCLConfig(path)
	}

	return nil, fmt.Errorf("unsupported config format: %s", path)
}

func (l *Loader) loadHCLConfig(path string) (*Rules, error) {
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
