package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestResolveConfigDir(t *testing.T) {
	t.Run("empty path returns false", func(t *testing.T) {
		if _, ok := resolveConfigDir(""); ok {
			t.Error("expected false for empty path")
		}
	})

	t.Run("non-existent path returns false", func(t *testing.T) {
		if _, ok := resolveConfigDir("/nonexistent/path"); ok {
			t.Error("expected false for non-existent path")
		}
	})

	t.Run("existing directory with .hcl-linter suffix returns true", func(t *testing.T) {
		dir := t.TempDir()
		configDir := filepath.Join(dir, ".hcl-linter")
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			t.Fatal(err)
		}

		got, ok := resolveConfigDir(dir)
		if !ok {
			t.Error("expected true for existing .hcl-linter directory")
		}
		if got != configDir {
			t.Errorf("got %s, want %s", got, configDir)
		}
	})

	t.Run("existing directory without suffix appends .hcl-linter", func(t *testing.T) {
		dir := t.TempDir()
		configDir := filepath.Join(dir, ".hcl-linter")
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			t.Fatal(err)
		}

		got, ok := resolveConfigDir(dir)
		if !ok {
			t.Error("expected true")
		}
		if got != configDir {
			t.Errorf("got %s, want %s", got, configDir)
		}
	})

	t.Run("path already ending with .hcl-linter returns as-is", func(t *testing.T) {
		dir := t.TempDir()
		configDir := filepath.Join(dir, ".hcl-linter")
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			t.Fatal(err)
		}

		got, ok := resolveConfigDir(configDir)
		if !ok {
			t.Error("expected true")
		}
		if got != configDir {
			t.Errorf("got %s, want %s", got, configDir)
		}
	})

	t.Run("path ending with /.hcl-linter returns as-is", func(t *testing.T) {
		dir := t.TempDir()
		configDir := filepath.Join(dir, ".hcl-linter")
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			t.Fatal(err)
		}

		got, ok := resolveConfigDir(dir + "/.hcl-linter")
		if !ok {
			t.Error("expected true")
		}
		if got != configDir {
			t.Errorf("got %s, want %s", got, configDir)
		}
	})
}

func TestFindConfigFiles(t *testing.T) {
	t.Run("empty baseName defaults to default", func(t *testing.T) {
		files := findConfigFiles("/config", "")
		want := []string{
			"/config/default.hcl",
			"/config/default",
		}
		if len(files) != len(want) {
			t.Fatalf("got %d files, want %d", len(files), len(want))
		}
		for i, w := range want {
			if files[i] != w {
				t.Errorf("files[%d]: got %s, want %s", i, files[i], w)
			}
		}
	})

	t.Run("generates hcl and no extension files in correct order", func(t *testing.T) {
		dir := t.TempDir()

		hcl := filepath.Join(dir, "myfile.hcl")
		raw := filepath.Join(dir, "myfile")

		if err := os.WriteFile(hcl, []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(raw, []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}

		files := findConfigFiles(dir, "myfile")

		want := []string{hcl, raw}

		if !reflect.DeepEqual(files, want) {
			t.Fatalf("got %v, want %v", files, want)
		}
	})

	t.Run("generates hcl and no extension files", func(t *testing.T) {
		dir := t.TempDir()

		// create files
		hcl := filepath.Join(dir, "myfile.hcl")
		raw := filepath.Join(dir, "myfile")

		if err := os.WriteFile(hcl, []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(raw, []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}

		files := findConfigFiles(dir, "myfile")

		want := []string{hcl, raw}

		if len(files) != len(want) {
			t.Fatalf("got %d files, want %d (%v)", len(files), len(want), files)
		}

		gotSet := make(map[string]struct{}, len(files))
		for _, f := range files {
			gotSet[f] = struct{}{}
		}

		for _, w := range want {
			if _, ok := gotSet[w]; !ok {
				t.Errorf("missing expected file: %s (got %v)", w, files)
			}
		}
	})
}

func TestResolveExtendsPath(t *testing.T) {
	tests := []struct {
		name        string
		currentPath string
		ref         string
		want        string
	}{
		{
			name:        "name without extension gets .hcl appended",
			currentPath: "/config/dir/child.hcl",
			ref:         "base",
			want:        "/config/dir/base.hcl",
		},
		{
			name:        "name with .hcl extension unchanged",
			currentPath: "/config/dir/child.hcl",
			ref:         "base.hcl",
			want:        "/config/dir/base.hcl",
		},
		{
			name:        "resolves relative to current file directory",
			currentPath: "/a/b/c.hcl",
			ref:         "default",
			want:        "/a/b/default.hcl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveExtendsPath(tt.currentPath, tt.ref)
			if got != tt.want {
				t.Errorf("resolveExtendsPath(%q, %q) = %q, want %q", tt.currentPath, tt.ref, got, tt.want)
			}
		})
	}
}

func TestFirstExistingFile(t *testing.T) {
	t.Run("empty slice returns empty string", func(t *testing.T) {
		if got := firstExistingFile([]string{}); got != "" {
			t.Error("expected empty string for empty slice")
		}
	})

	t.Run("no existing files returns empty string", func(t *testing.T) {
		paths := []string{
			"/nonexistent/file1.hcl",
			"/nonexistent/file2.hcl",
		}
		if got := firstExistingFile(paths); got != "" {
			t.Errorf("got %s, want empty string", got)
		}
	})

	t.Run("returns first existing file", func(t *testing.T) {
		dir := t.TempDir()

		file1 := filepath.Join(dir, "file1.hcl")
		file2 := filepath.Join(dir, "file2.hcl")
		file3 := filepath.Join(dir, "file3")

		if err := os.WriteFile(file1, []byte("rules {}"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file2, []byte("rules {}"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file3, []byte("rules {}"), 0o644); err != nil {
			t.Fatal(err)
		}

		paths := []string{
			"/nonexistent/a.hcl",
			file1,
			file2,
			file3,
		}

		got := firstExistingFile(paths)
		if got != file1 {
			t.Errorf("got %s, want %s", got, file1)
		}
	})

	t.Run("skips non-existing in middle", func(t *testing.T) {
		dir := t.TempDir()

		file2 := filepath.Join(dir, "file2.hcl")
		if err := os.WriteFile(file2, []byte("rules {}"), 0o644); err != nil {
			t.Fatal(err)
		}

		paths := []string{
			"/nonexistent/a.hcl",
			"/nonexistent/b.hcl",
			file2,
		}

		got := firstExistingFile(paths)
		if got != file2 {
			t.Errorf("got %s, want %s", got, file2)
		}
	})
}
