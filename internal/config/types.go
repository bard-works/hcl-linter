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
	Rules Rules
}

// Rules contains all rule configurations.
type Rules struct {
	BlockOrder        *BlockOrderConfig
	ArrayFormat       *ArrayFormatConfig
	NameValidation    *NameValidationConfig
	Duplicates        *DuplicatesConfig
	RequiredFields    *RequiredFieldsConfig
	BlankLines        *BlankLinesConfig
	RequiredBlocks    *RequiredBlocksConfig
	DependencyPaths   *DependencyPathsConfig
	IncludePaths      *IncludePathsConfig
	RemoteState       *RemoteStateConfig
	HCLFunctions      *HCLFunctionsConfig
	TerraformBlock    *TerraformBlockConfig
	KeyValue          *KeyValueConfig
	CountForEach      *CountForEachConfig
	DependencyOutputs *DependencyOutputsConfig
	MaxConcurrency    int
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
	Enabled     bool
	Order       []string
	NestedOrder map[string][]string
}

// ArrayFormatConfig configures array formatting rules.
type ArrayFormatConfig struct {
	Enabled            bool
	MultilineThreshold int
	Sort               bool
}

// NameValidationConfig configures name validation rules.
type NameValidationConfig struct {
	Enabled bool
	Pattern string
	Blocks  []string
}

// DuplicatesConfig configures duplicate detection rules.
type DuplicatesConfig struct {
	Enabled bool
	Blocks  []string
}

// RequiredFieldsConfig configures required field rules.
type RequiredFieldsConfig struct {
	Include *IncludeRequired
}

// IncludeRequired specifies required fields for include blocks.
type IncludeRequired struct {
	Expose bool
}

// RequiredBlocksConfig configures required block rules.
type RequiredBlocksConfig struct {
	Required []RequiredBlockSpec
}

// RequiredBlockSpec specifies a required block.
type RequiredBlockSpec struct {
	Type  string
	Count string
	Error string
}

// DependencyPathsConfig configures validation of dependency-block config_path attributes.
type DependencyPathsConfig struct {
	Enabled bool
}

// IncludePathsConfig configures validation of include-block path attributes.
type IncludePathsConfig struct {
	Enabled bool
}

// RemoteStateConfig configures validation of remote_state blocks (nested inside the terraform block).
type RemoteStateConfig struct {
	Enabled        bool
	RequireBackend bool
}

// DependencyBlockSpec represents a dependency block label.
type DependencyBlockSpec struct {
	Label      string
	ConfigPath string
}

// IncludeBlockSpec represents an include block.
type IncludeBlockSpec struct {
	Path string
}

// RemoteStateBlockSpec represents a remote_state block.
type RemoteStateBlockSpec struct {
	Backend string
	Config  map[string]struct {
		Required bool
	}
}

// HCLFunctionsConfig configures validation of HCL function calls
// (currently: Terragrunt's find_in_parent_folders and get_env).
type HCLFunctionsConfig struct {
	Enabled                   bool
	FindInParentFoldersExists bool
	GetEnvHasDefault          bool
}

// TerraformBlockConfig configures Terraform block validation rules.
type TerraformBlockConfig struct {
	Enabled             bool
	SourceRequired      bool
	VersionFormat       bool
	ExtraArgumentsValid bool
	NoDeprecatedFields  bool
}

// KeyValueConfig configures key-value validation rules.
type KeyValueConfig struct {
	Enabled      bool
	KeyCase      string
	ValuePattern map[string]string
	Disallowed   []string
}

// CountForEachConfig configures count/for_each validation rules.
type CountForEachConfig struct {
	Enabled            bool
	WarnOnCountZero    bool
	WarnOnEmptyForEach bool
	WarnOnConflict     bool
}

// DependencyOutputsConfig configures dependency output validation rules.
type DependencyOutputsConfig struct {
	Enabled bool
}

// BlankLinesConfig configures blank line rules.
type BlankLinesConfig struct {
	Enabled      bool
	WithinBlocks bool
}
