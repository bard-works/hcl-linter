package config

import (
	"os"
	"path/filepath"
	"strings"
)

func resolveConfigDir(path string) (string, bool) {
	if path == "" {
		return "", false
	}

	dir := path
	if !strings.HasSuffix(dir, ".hcl-linter") && !strings.HasSuffix(dir, "/.hcl-linter") {
		dir = filepath.Join(path, ".hcl-linter")
	}
	if _, err := os.Stat(dir); err == nil {
		return dir, true
	}
	if _, err := os.Stat(path); err == nil {
		return path, true
	}
	return "", false
}

func findConfigFiles(dir, baseName string) []string {
	if baseName == "" {
		baseName = "default"
	}

	var files []string
	for _, ext := range []string{".json", ".hcl", ""} {
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
