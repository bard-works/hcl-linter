package config

import (
	"os"
	"runtime"
	"strconv"
)

// ConfigSource represents where configuration is loaded from.
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

// ConfigResult contains the loaded config and metadata.
type ConfigResult struct {
	Rules      *Rules
	Source     ConfigSource
	SourcePath string
	WarningMsg string
}

// Config is the root config structure.
type Config struct {
	Rules Rules `json:"rules"`
}

// Rules contains all rule configurations.
type Rules struct {
	BlockOrder        *BlockOrderConfig        `json:"block_order,omitempty"`
	ArrayFormat       *ArrayFormatConfig       `json:"array_format,omitempty"`
	NameValidation    *NameValidationConfig    `json:"name_validation,omitempty"`
	Duplicates        *DuplicatesConfig        `json:"duplicates,omitempty"`
	RequiredFields    *RequiredFieldsConfig    `json:"required_fields,omitempty"`
	BlankLines        *BlankLinesConfig        `json:"blank_lines,omitempty"`
	RequiredBlocks    *RequiredBlocksConfig    `json:"required_blocks,omitempty"`
	DependencyPaths   *DependencyPathsConfig   `json:"dependency_paths,omitempty"`
	IncludePaths      *IncludePathsConfig      `json:"include_paths,omitempty"`
	RemoteState       *RemoteStateConfig       `json:"remote_state,omitempty"`
	HCLFunctions      *HCLFunctionsConfig      `json:"hcl_functions,omitempty"`
	TerraformBlock    *TerraformBlockConfig    `json:"terraform_block,omitempty"`
	KeyValue          *KeyValueConfig          `json:"key_value,omitempty"`
	CountForEach      *CountForEachConfig      `json:"count_for_each,omitempty"`
	DependencyOutputs *DependencyOutputsConfig `json:"dependency_outputs,omitempty"`
	MaxConcurrency    int                      `json:"max_concurrency,omitempty"`
}

const EnvMaxConcurrency = "HCL_LINTER_MAX_CONCURRENCY"

// GetMaxConcurrency returns the max concurrency setting from env, config, or defaults.
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

// IsEnabled returns true if any rule is configured.
func (r *Rules) IsEnabled() bool {
	return r.BlockOrder != nil || r.ArrayFormat != nil ||
		r.NameValidation != nil || r.Duplicates != nil || r.RequiredFields != nil ||
		r.BlankLines != nil || r.RequiredBlocks != nil ||
		r.DependencyPaths != nil || r.IncludePaths != nil || r.RemoteState != nil ||
		r.HCLFunctions != nil || r.TerraformBlock != nil || r.KeyValue != nil ||
		r.CountForEach != nil || r.DependencyOutputs != nil
}

// BlockOrderConfig configures block ordering rules.
type BlockOrderConfig struct {
	Enabled     bool                `json:"enabled"`
	Order       []string            `json:"order"`
	NestedOrder map[string][]string `json:"nested_order,omitempty"`
}

// ArrayFormatConfig configures array formatting rules.
type ArrayFormatConfig struct {
	Enabled            bool `json:"enabled"`
	MultilineThreshold int  `json:"multiline_threshold"`
	Sort               bool `json:"sort"`
}

// NameValidationConfig configures name validation rules.
type NameValidationConfig struct {
	Enabled bool     `json:"enabled"`
	Pattern string   `json:"pattern"`
	Blocks  []string `json:"blocks"`
}

// DuplicatesConfig configures duplicate detection rules.
type DuplicatesConfig struct {
	Enabled bool     `json:"enabled"`
	Blocks  []string `json:"blocks"`
}

// RequiredFieldsConfig configures required field rules.
type RequiredFieldsConfig struct {
	Include *IncludeRequired `json:"include,omitempty"`
}

// IncludeRequired specifies required fields for include blocks.
type IncludeRequired struct {
	Expose bool `json:"expose"`
}

// RequiredBlocksConfig configures required block rules.
type RequiredBlocksConfig struct {
	Required []RequiredBlockSpec `json:"required"`
}

// RequiredBlockSpec specifies a required block.
type RequiredBlockSpec struct {
	Type  string `json:"type"`
	Count string `json:"count"`
	Error string `json:"error"`
}

// DependencyPathsConfig configures validation of dependency-block config_path attributes.
type DependencyPathsConfig struct {
	Enabled bool `json:"enabled"`
}

// IncludePathsConfig configures validation of include-block path attributes.
type IncludePathsConfig struct {
	Enabled bool `json:"enabled"`
}

// RemoteStateConfig configures validation of remote_state blocks (nested inside the terraform block).
type RemoteStateConfig struct {
	Enabled        bool `json:"enabled"`
	RequireBackend bool `json:"require_backend"`
}

// DependencyBlockSpec represents a dependency block label.
type DependencyBlockSpec struct {
	Label      string `json:"label"`
	ConfigPath string `json:"config_path"`
}

// IncludeBlockSpec represents an include block.
type IncludeBlockSpec struct {
	Path string `json:"path"`
}

// RemoteStateBlockSpec represents a remote_state block.
type RemoteStateBlockSpec struct {
	Backend string `json:"backend"`
	Config  map[string]struct {
		Required bool `json:"required"`
	}
}

// HCLFunctionsConfig configures validation of HCL function calls
// (currently: Terragrunt's find_in_parent_folders and get_env).
type HCLFunctionsConfig struct {
	Enabled                   bool `json:"enabled"`
	FindInParentFoldersExists bool `json:"find_in_parent_folders_exists"`
	GetEnvHasDefault          bool `json:"get_env_has_default"`
}

// TerraformBlockConfig configures Terraform block validation rules.
type TerraformBlockConfig struct {
	Enabled             bool `json:"enabled"`
	SourceRequired      bool `json:"source_required"`
	VersionFormat       bool `json:"version_format"`
	ExtraArgumentsValid bool `json:"extra_arguments_valid"`
	NoDeprecatedFields  bool `json:"no_deprecated_fields"`
}

// KeyValueConfig configures key-value validation rules.
type KeyValueConfig struct {
	Enabled      bool              `json:"enabled"`
	KeyCase      string            `json:"key_case,omitempty"`
	ValuePattern map[string]string `json:"value_pattern,omitempty"`
	Disallowed   []string          `json:"disallowed,omitempty"`
}

// CountForEachConfig configures count/for_each validation rules.
type CountForEachConfig struct {
	Enabled            bool `json:"enabled"`
	WarnOnCountZero    bool `json:"warn_on_count_zero"`
	WarnOnEmptyForEach bool `json:"warn_on_empty_for_each"`
	WarnOnConflict     bool `json:"warn_on_conflict"`
}

// DependencyOutputsConfig configures dependency output validation rules.
type DependencyOutputsConfig struct {
	Enabled bool `json:"enabled"`
}

// BlankLinesConfig configures blank line rules.
type BlankLinesConfig struct {
	Enabled      bool `json:"enabled"`
	WithinBlocks bool `json:"within_blocks"`
}

