package config

import (
	"os"
	"path/filepath"
)

func resolveConfigDir(path string) (string, bool) {
	if path == "" {
		return "", false
	}

	// If already pointing to .hcl-linter
	if filepath.Base(path) == ".hcl-linter" {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path, true
		}
		return "", false
	}

	// Try <path>/.hcl-linter
	configDir := filepath.Join(path, ".hcl-linter")
	if info, err := os.Stat(configDir); err == nil && info.IsDir() {
		return configDir, true
	}

	// Fallback: only if path itself is a dir (questionable design)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return path, true
	}

	return "", false
}

func findConfigFiles(dir, baseName string) []string {
	if baseName == "" {
		baseName = "default"
	}

	var files []string
	for _, ext := range []string{".hcl", ""} {
		var path string
		if ext == "" {
			path = filepath.Join(dir, baseName)
		} else {
			path = filepath.Join(dir, baseName+ext)
		}
		files = append(files, path)
	}
	return files
}

func firstExistingFile(paths []string) string {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
