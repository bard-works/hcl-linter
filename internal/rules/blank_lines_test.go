package rules_test

import (
	"strings"
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/rules"
)

func TestBlankLinesRuleEnabled(t *testing.T) {
	r := rules.BlankLinesRule{}

	if r.Enabled(nil) {
		t.Error("expected false for nil config")
	}
	if r.Enabled(&config.Rules{}) {
		t.Error("expected false for empty config")
	}
	if r.Enabled(&config.Rules{BlankLines: &config.BlankLinesConfig{Enabled: true, WithinBlocks: false}}) {
		t.Error("expected false when WithinBlocks=false")
	}
	if !r.Enabled(&config.Rules{BlankLines: &config.BlankLinesConfig{Enabled: true, WithinBlocks: true}}) {
		t.Error("expected true when Enabled=true and WithinBlocks=true")
	}
}

func TestBlankLinesRuleFix(t *testing.T) {
	cfg := &config.Rules{
		BlankLines: &config.BlankLinesConfig{Enabled: true, WithinBlocks: true},
	}

	tests := []struct {
		name    string
		content string
		changed bool
		want    string
	}{
		{
			name:    "removes blank line before closing brace in object attr",
			content: "inputs = {\n  key = \"value\"\n\n}\n",
			changed: true,
			want:    "inputs = {\n  key = \"value\"\n}\n",
		},
		{
			name:    "removes duplicate blank lines in object attr",
			content: "inputs = {\n  a = \"x\"\n\n\n  b = \"y\"\n}\n",
			changed: true,
			want:    "inputs = {\n  a = \"x\"\n\n  b = \"y\"\n}\n",
		},
		{
			name:    "no change when already clean",
			content: "inputs = {\n  a = \"x\"\n  b = \"y\"\n}\n",
			changed: false,
			want:    "inputs = {\n  a = \"x\"\n  b = \"y\"\n}\n",
		},
	}

	r := rules.BlankLinesRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := buildContext(t, tt.content, cfg)
			got, changed, err := r.Fix(ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if changed != tt.changed {
				t.Errorf("changed: got %v, want %v", changed, tt.changed)
			}
			if tt.want != "" && strings.TrimSpace(string(got)) != strings.TrimSpace(tt.want) {
				t.Errorf("content mismatch:\ngot:  %q\nwant: %q", string(got), tt.want)
			}
		})
	}
}
