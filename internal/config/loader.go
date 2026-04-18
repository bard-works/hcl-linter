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
	for source, dir := range map[ConfigSource]string{
		ConfigSourceCLI: loader.configDir,
		ConfigSourceEnv: os.Getenv("HCL_LINTER_CONFIG_DIR"),
	} {
		if resolved, ok := resolveConfigDir(dir); ok {
			return &ConfigResult{
				Source:     source,
				SourcePath: resolved,
			}
		}
	}

	if cwd, _ := getCwdConfigDir(); cwd != "" {
		if _, err := os.Stat(cwd); err == nil {
			return &ConfigResult{Source: ConfigSourceCwd, SourcePath: cwd}
		}
	}
	if home, _ := getHomeConfigDir(); home != "" {
		if _, err := os.Stat(home); err == nil {
			return &ConfigResult{Source: ConfigSourceHome, SourcePath: home}
		}
	}
	if proj, _ := getProjectConfigDir(); proj != "" {
		if _, err := os.Stat(proj); err == nil {
			return &ConfigResult{
				Source:     ConfigSourceProject,
				SourcePath: proj,
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
	baseName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))

	paths := findConfigFiles(l.configDir, baseName)
	if file := firstExistingFile(paths); file != "" {
		rules, err := l.loadConfigFile(file)
		if err == nil {
			return rules, nil
		}
		if !strings.Contains(err.Error(), "unsupported config format") {
			return nil, err
		}
	}

	defaultPaths := findConfigFiles(l.configDir, "default")
	if file := firstExistingFile(defaultPaths); file != "" {
		rules, err := l.loadConfigFile(file)
		if err == nil {
			return rules, nil
		}
		if !strings.Contains(err.Error(), "unsupported config format") {
			return nil, err
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

	baseName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))

	paths := findConfigFiles(l.configDir, baseName)
	if firstExistingFile(paths) != "" {
		return true
	}

	return firstExistingFile(findConfigFiles(l.configDir, "default")) != ""
}

func (l *Loader) HasSpecificConfigForFile(filename string) bool {
	if l == nil || l.configDir == "" {
		return false
	}

	baseName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))

	return firstExistingFile(findConfigFiles(l.configDir, baseName)) != ""
}
