package rules

import (
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
)

func TestHclStringValue(t *testing.T) {
	src := `
num     = 123
boolean = true
list    = [1]
mapping = { a = 1 }
str     = "ok"
`
	p := hclparse.NewParser()
	file, diags := p.ParseHCL([]byte(src), "test.hcl")
	if diags.HasErrors() {
		t.Fatalf("parse error: %s", diags.Error())
	}
	attrs, diags := file.Body.JustAttributes()
	if diags.HasErrors() {
		t.Fatalf("attrs error: %s", diags.Error())
	}

	tests := []struct {
		key  string
		want string
	}{
		{"num", ""},
		{"boolean", ""},
		{"list", ""},
		{"mapping", ""},
		{"str", "ok"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			attr, ok := attrs[tt.key]
			if !ok {
				t.Fatalf("attribute %q not found", tt.key)
			}
			got := hclStringValue(attr.Expr)
			if got != tt.want {
				t.Errorf("hclStringValue(%s) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}
