package ast

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func TestNewParserAndParseContent(t *testing.T) {
	p := NewParser()
	if p == nil || p.parser == nil {
		t.Fatal("NewParser returned nil")
	}

	file, diags := p.ParseContent([]byte(`locals { foo = 1 }`), "test.hcl")
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %s", diags.Error())
	}
	if file == nil {
		t.Fatal("expected file, got nil")
	}
}

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.hcl")
	if err := os.WriteFile(path, []byte(`locals { foo = 1 }`), 0o644); err != nil {
		t.Fatal(err)
	}

	p := NewParser()
	file, diags := p.ParseFile(path)
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %s", diags.Error())
	}
	if file == nil {
		t.Fatal("expected file, got nil")
	}
}

func TestGetTopLevelBlocks(t *testing.T) {
	p := NewParser()
	file, _ := p.ParseContent([]byte(`
terraform { source = "./m" }
locals { foo = 1 }
dependency "vpc" { config_path = "../vpc" }
`), "test.hcl")

	blocks := GetTopLevelBlocks(file)
	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(blocks))
	}

	types := map[string]bool{}
	for _, b := range blocks {
		types[b.Type] = true
	}
	for _, want := range []string{"terraform", "locals", "dependency"} {
		if !types[want] {
			t.Errorf("missing block type %q", want)
		}
	}

	for _, b := range blocks {
		if b.Type == "dependency" {
			if len(b.Labels) != 1 || b.Labels[0] != "vpc" {
				t.Errorf("got labels %v, want [vpc]", b.Labels)
			}
		}
		if b.Block == nil {
			t.Error("expected Block pointer to be set")
		}
		if b.EndLine < b.StartLine {
			t.Errorf("EndLine %d < StartLine %d", b.EndLine, b.StartLine)
		}
	}
}

func TestGetTopLevelBlocks_Empty(t *testing.T) {
	p := NewParser()
	file, _ := p.ParseContent([]byte(``), "test.hcl")
	blocks := GetTopLevelBlocks(file)
	if len(blocks) != 0 {
		t.Errorf("expected 0 blocks, got %d", len(blocks))
	}
}

func TestGetBlockAttributes(t *testing.T) {
	p := NewParser()
	file, _ := p.ParseContent([]byte(`
terraform {
  source  = "./m"
  version = "1.0.0"
}
`), "test.hcl")

	body := file.Body.(*hclsyntax.Body)
	attrs := GetBlockAttributes(body.Blocks[0].Body)
	if _, ok := attrs["source"]; !ok {
		t.Error("missing 'source' attribute")
	}
	if _, ok := attrs["version"]; !ok {
		t.Error("missing 'version' attribute")
	}
}

func TestGetBlockNestedBlocks(t *testing.T) {
	p := NewParser()
	file, _ := p.ParseContent([]byte(`
terraform {
  source = "./m"
  remote_state {
    backend = "s3"
  }
  before_hook "h1" { commands = ["apply"] }
  before_hook "h2" { commands = ["plan"] }
}
`), "test.hcl")

	body := file.Body.(*hclsyntax.Body)
	blockBody := body.Blocks[0].Body

	remote := GetBlockNestedBlocks(blockBody, "remote_state")
	if len(remote) != 1 {
		t.Errorf("expected 1 remote_state, got %d", len(remote))
	}

	hooks := GetBlockNestedBlocks(blockBody, "before_hook")
	if len(hooks) != 2 {
		t.Errorf("expected 2 before_hooks, got %d", len(hooks))
	}

	all := GetBlockNestedBlocks(blockBody, "*")
	// Wildcard should return all nested blocks: remote_state + 2 before_hooks = 3
	if len(all) != 3 {
		t.Errorf("wildcard GetBlockNestedBlocks: expected 3 blocks, got %d", len(all))
	}
}

func TestGetTopLevelAttributes(t *testing.T) {
	p := NewParser()
	file, _ := p.ParseContent([]byte(`
inputs = { name = "test" }
extends = "base"
`), "test.hcl")

	attrs := GetTopLevelAttributes(file)
	names := map[string]bool{}
	for _, a := range attrs {
		names[a.Name] = true
		if a.Expr == nil {
			t.Errorf("attr %s has nil Expr", a.Name)
		}
		if a.EndLine < a.StartLine {
			t.Errorf("attr %s EndLine %d < StartLine %d", a.Name, a.EndLine, a.StartLine)
		}
	}
	if !names["inputs"] || !names["extends"] {
		t.Errorf("got attrs %v, want inputs and extends", names)
	}
}

func TestIsObjectAttribute(t *testing.T) {
	p := NewParser()
	file, _ := p.ParseContent([]byte(`
inputs = { name = "test" }
scalar = "value"
`), "test.hcl")

	attrs := GetTopLevelAttributes(file)
	for _, a := range attrs {
		got := IsObjectAttribute(a.Expr)
		switch a.Name {
		case "inputs":
			if !got {
				t.Error("expected inputs to be object")
			}
		case "scalar":
			if got {
				t.Error("expected scalar not to be object")
			}
		}
	}
}

func TestGetBlockInfoFromBlocks(t *testing.T) {
	p := NewParser()
	file, _ := p.ParseContent([]byte(`
dependency "a" { config_path = "x" }
dependency "b" { config_path = "y" }
`), "test.hcl")

	body := file.Body.(*hclsyntax.Body)
	infos := GetBlockInfoFromBlocks(body.Blocks)
	if len(infos) != 2 {
		t.Fatalf("expected 2 infos, got %d", len(infos))
	}
	if infos[0].Type != "dependency" || infos[0].Labels[0] != "a" {
		t.Errorf("infos[0]: got type=%s label=%v", infos[0].Type, infos[0].Labels)
	}
}

func TestGetAttributeRange(t *testing.T) {
	p := NewParser()
	file, _ := p.ParseContent([]byte(`
inputs = {
  name = "test"
}
scalar = "value"
`), "test.hcl")

	attrs := GetTopLevelAttributes(file)
	for _, a := range attrs {
		start, end := GetAttributeRange(a.Expr)
		if end < start {
			t.Errorf("attr %s: end %d < start %d", a.Name, end, start)
		}
		if a.Name == "inputs" && end == start {
			t.Errorf("expected multiline inputs range, got start=end=%d", start)
		}
	}
}

func TestGetBodyAttributes(t *testing.T) {
	p := NewParser()
	file, _ := p.ParseContent([]byte(`
terraform {
  source  = "./m"
  version = "1.0.0"
}
`), "test.hcl")

	body := file.Body.(*hclsyntax.Body)
	attrs := GetBodyAttributes(body.Blocks[0].Body)
	if attrs == nil {
		t.Fatal("expected non-nil map")
	}
	if _, ok := attrs["source"]; !ok {
		t.Error("missing 'source' attribute")
	}
	if _, ok := attrs["version"]; !ok {
		t.Error("missing 'version' attribute")
	}
	if _, ok := attrs["nonexistent"]; ok {
		t.Error("unexpected 'nonexistent' attribute")
	}
}
