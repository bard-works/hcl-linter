package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Loader struct {
	configDir string

	resolveMu    sync.RWMutex
	resolveCache map[string]string
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
	for _, entry := range []struct {
		source ConfigSource
		dir    string
	}{
		{ConfigSourceCLI, loader.configDir},
		{ConfigSourceEnv, os.Getenv("HCL_LINTER_CONFIG_DIR")},
	} {
		if resolved, ok := resolveConfigDir(entry.dir); ok {
			return &ConfigResult{
				Source:     entry.source,
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

// resolveConfigDirForFile walks upward from the file's directory looking for
// a `.hcl-linter/` that contains a usable config for this file (either
// `<basename>.hcl` or `default.hcl`). Returns the closest matching dir, or
// falls back to the loader's root configDir. Results are cached per
// start-directory so concurrent LintFiles runs don't repeat the walk.
func (l *Loader) resolveConfigDirForFile(filename string) string {
	if l == nil {
		return ""
	}

	absFile, err := filepath.Abs(filename)
	if err != nil {
		return l.configDir
	}
	start := filepath.Dir(absFile)
	baseName := configBaseName(filename)

	cacheKey := start + "\x00" + baseName
	l.resolveMu.RLock()
	if cached, ok := l.resolveCache[cacheKey]; ok {
		l.resolveMu.RUnlock()
		return cached
	}
	l.resolveMu.RUnlock()

	resolved := l.walkForConfigDir(start, baseName)

	l.resolveMu.Lock()
	if l.resolveCache == nil {
		l.resolveCache = map[string]string{}
	}
	l.resolveCache[cacheKey] = resolved
	l.resolveMu.Unlock()
	return resolved
}

// walkForConfigDir searches from start upward for the closest `.hcl-linter/`
// with a match for baseName or a default.hcl. The walk is bounded by the
// parent of the root configDir - files outside that subtree skip the walk
// entirely and use the root directly. This keeps behaviour predictable for
// callers that pass bare filenames and avoids leaking filesystem structure
// above the configured project root.
func (l *Loader) walkForConfigDir(start, baseName string) string {
	rootAbs, _ := filepath.Abs(l.configDir)
	if rootAbs == "" {
		return l.configDir
	}
	boundary := filepath.Dir(rootAbs)
	if !isWithinDir(start, boundary) {
		return l.configDir
	}

	dir := start
	for {
		candidate := filepath.Join(dir, ".hcl-linter")
		if candidate == rootAbs {
			return l.configDir
		}
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			if configDirHasMatch(candidate, baseName) {
				return candidate
			}
		}

		if dir == boundary {
			return l.configDir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return l.configDir
		}
		dir = parent
	}
}

func isWithinDir(path, dir string) bool {
	if dir == "" {
		return false
	}
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || !strings.HasPrefix(rel, "..")
}

func configBaseName(filename string) string {
	return strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
}

func configDirHasMatch(dir, baseName string) bool {
	if firstExistingFile(findConfigFiles(dir, baseName)) != "" {
		return true
	}
	return firstExistingFile(findConfigFiles(dir, "default")) != ""
}

func (l *Loader) LoadForFile(filename string) (*Rules, error) {
	dir := l.resolveConfigDirForFile(filename)
	baseName := configBaseName(filename)

	paths := findConfigFiles(dir, baseName)
	if file := firstExistingFile(paths); file != "" {
		rules, err := l.loadConfigFile(file)
		if err == nil {
			return rules, nil
		}
		if !strings.Contains(err.Error(), "unsupported config format") {
			return nil, err
		}
	}

	defaultPaths := findConfigFiles(dir, "default")
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
	if strings.HasSuffix(path, ".hcl") {
		return loadHCLConfig(path)
	}

	return nil, fmt.Errorf("unsupported config format: %s", path)
}

func (l *Loader) HasConfigForFile(filename string) bool {
	if l == nil || l.configDir == "" {
		return false
	}

	dir := l.resolveConfigDirForFile(filename)
	baseName := configBaseName(filename)

	if firstExistingFile(findConfigFiles(dir, baseName)) != "" {
		return true
	}
	return firstExistingFile(findConfigFiles(dir, "default")) != ""
}

func (l *Loader) HasSpecificConfigForFile(filename string) bool {
	if l == nil || l.configDir == "" {
		return false
	}

	dir := l.resolveConfigDirForFile(filename)
	baseName := configBaseName(filename)

	return firstExistingFile(findConfigFiles(dir, baseName)) != ""
}
