package rules_test

import (
	"os"
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/rules"
)

func buildContextFromFile(t *testing.T, filePath string, cfg *config.Rules) *rules.Context {
	t.Helper()
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	p := hclparse.NewParser()
	file, diags := p.ParseHCL(content, filePath)
	if diags.HasErrors() {
		t.Fatalf("parse error: %s", diags.Error())
	}
	return &rules.Context{
		FilePath: filePath,
		Content:  content,
		File:     file,
		Blocks:   ast.GetTopLevelBlocks(file),
		Attrs:    ast.GetTopLevelAttributes(file),
		Config:   cfg,
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
