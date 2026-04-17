package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func (r *Rules) IsEnabled() bool {
	return r.BlockOrder != nil || r.ArrayFormat != nil ||
		r.NameValidation != nil || r.Duplicates != nil || r.RequiredFields != nil ||
		r.BlankLines != nil
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

	return nil, fmt.Errorf("unsupported config format: %s", path)
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
