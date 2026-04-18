package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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

func (l *Loader) loadConfigFile(path string) (*Rules, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %s: %w", path, err)
	}

	if strings.HasSuffix(path, ".json") {
		return loadJSONConfig(data, path)
	}

	if strings.HasSuffix(path, ".hcl") {
		return loadHCLConfig(path)
	}

	return nil, fmt.Errorf("unsupported config format: %s", path)
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
