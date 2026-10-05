package rules_test

import (
	"strings"
	"testing"

	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/rules"
)

func TestArrayFormatRuleCheck(t *testing.T) {
	cfg := &config.Rules{
		ArrayFormat: &config.ArrayFormatConfig{Enabled: true},
	}

	tests := []struct {
		name        string
		content     string
		expectIssue bool
	}{
		{
			name:        "multiline array - no issue",
			content:     "arr = [\n  \"a\",\n  \"b\",\n]\n",
			expectIssue: false,
		},
		{
			name:        "single-item inline - no issue",
			content:     "arr = [\"only\"]\n",
			expectIssue: false,
		},
		{
			name:        "two-item inline tuple inside block - issue",
			content:     "inputs {\n  arr = [\"a\", \"b\"]\n}\n",
			expectIssue: true,
		},
	}

	rule := rules.ArrayFormatRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := buildContext(t, tt.content, cfg)
			issues := rule.Check(ctx)
			hasIssue := false
			for _, issue := range issues {
				if issue.Rule == "array_format" {
					hasIssue = true
				}
			}
			if tt.expectIssue && !hasIssue {
				t.Error("expected array_format issue, got none")
			}
			if !tt.expectIssue && hasIssue {
				t.Errorf("unexpected array_format issue: %v", issues)
			}
		})
	}
}

func TestArrayFormatRuleFixSort(t *testing.T) {
	tests := []struct {
		name     string
		sort     bool
		input    string
		wantSort bool
	}{
		{
			name:     "sort disabled - preserves order",
			sort:     false,
			input:    `arr = ["charlie", "alpha", "bravo"]` + "\n",
			wantSort: false,
		},
		{
			name:     "sort enabled - alphabetical",
			sort:     true,
			input:    `arr = ["charlie", "alpha", "bravo"]` + "\n",
			wantSort: true,
		},
	}

	rule := rules.ArrayFormatRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Rules{
				ArrayFormat: &config.ArrayFormatConfig{Enabled: true, Sort: tt.sort},
			}
			ctx := buildContext(t, tt.input, cfg)
			n, err := rule.Fix(ctx)
			if err != nil {
				t.Fatalf("Fix error: %v", err)
			}
			if n == 0 {
				t.Fatal("expected Fix to report a change")
			}
			got := string(ctx.Content)
			alphaIdx := strings.Index(got, "alpha")
			charlieIdx := strings.Index(got, "charlie")
			if tt.wantSort && alphaIdx > charlieIdx {
				t.Errorf("expected sorted output (alpha before charlie):\n%s", got)
			}
			if !tt.wantSort && charlieIdx > alphaIdx {
				t.Errorf("expected unsorted output (charlie before alpha):\n%s", got)
			}
		})
	}
}

func TestFixArraysPreservesFileEnding(t *testing.T) {
	for _, in := range []string{
		"inputs = {\n  azs = [\"a\", \"b\"]\n}\n",
		"inputs = {\n  azs = [\"a\", \"b\"]\n}",
		"azs = [\"a\", \"b\"]\n",
	} {
		once, _ := rules.FixArrays(in, false)
		if strings.HasSuffix(in, "\n") != strings.HasSuffix(once, "\n") || strings.HasSuffix(once, "\n\n") {
			t.Errorf("file ending changed for %q: got %q", in, once)
		}
		twice, changes := rules.FixArrays(once, false)
		if changes != 0 || twice != once {
			t.Errorf("not idempotent for %q: %q -> %q", in, once, twice)
		}
	}
}

// The text-based fixer must leave alone every shape it cannot rewrite
// without changing meaning.
func TestFixArraysLeavesUnsafeShapesAlone(t *testing.T) {
	cases := map[string]string{
		"array of objects": `policy = jsonencode({
  Statement = [
    {
      Action = "s3:GetObject"
    },
  ]
})
`,
		"function call wrapping array": "tags = merge(local.tags, [\"a\", \"b\"])\n",
		"call inside array":            "x = [format(\"%s\", a), \"b\"]\n",
		"references":                   "ids = [dependency.a.outputs.id, dependency.b.outputs.id]\n",
		"interpolation":                "x = [\"${var.a}\", \"b\"]\n",
		"trailing comment":             "x = [\"a\", \"b\"] # keep me\n",
		"heredoc body": `user_data = <<EOT
names = ["a", "b"]
EOT
`,
		"block comment": "/*\nx = [\"a\", \"b\"]\n*/\n",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			got, changes := rules.FixArrays(in, false)
			if changes != 0 || got != in {
				t.Errorf("expected no change, got %d change(s):\n%s", changes, got)
			}
		})
	}
}

func TestFixArraysInsideObjectKeepsComma(t *testing.T) {
	in := "inputs = {\n  azs = [\"a\", \"b\"],\n  name = \"x\"\n}\n"
	want := "inputs = {\n  azs = [\n    \"a\",\n    \"b\",\n  ],\n  name = \"x\"\n}\n"
	got, changes := rules.FixArrays(in, false)
	if changes != 1 || got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// check must report exactly what fix rewrites, so a fixed file passes check.
func TestArrayFormatCheckMatchesFix(t *testing.T) {
	cfg := &config.Rules{ArrayFormat: &config.ArrayFormatConfig{Enabled: true, MultilineThreshold: 2}}
	src := `inputs = {
  azs    = ["a", "b"]
  single = ["x"]
  refs   = [var.a, var.b]
  done = [
    "c",
    "d",
  ]
}
`
	rule := rules.ArrayFormatRule{}
	ctx := buildContext(t, src, cfg)
	issues := rule.Check(ctx)
	if len(issues) != 1 || issues[0].Location.Start.Line != 2 {
		t.Fatalf("expected one issue for azs on line 2, got %+v", issues)
	}

	if _, err := rule.Fix(ctx); err != nil {
		t.Fatal(err)
	}
	fixed := buildContext(t, string(ctx.Content), cfg)
	if issues := rule.Check(fixed); len(issues) != 0 {
		t.Errorf("fixed file still fails check: %+v\n%s", issues, ctx.Content)
	}
}

func TestArrayFormatThreshold(t *testing.T) {
	cfg := &config.Rules{ArrayFormat: &config.ArrayFormatConfig{Enabled: true, MultilineThreshold: 3}}
	src := "locals {\n  two   = [\"a\", \"b\"]\n  three = [\"a\", \"b\", \"c\"]\n}\n"
	ctx := buildContext(t, src, cfg)
	issues := rules.ArrayFormatRule{}.Check(ctx)
	if len(issues) != 1 || issues[0].Location.Start.Line != 3 {
		t.Fatalf("threshold 3 should only flag the 3-item array, got %+v", issues)
	}
	n, err := rules.ArrayFormatRule{}.Fix(ctx)
	if err != nil || n != 1 || !strings.Contains(string(ctx.Content), `two   = ["a", "b"]`) {
		t.Errorf("fix should honor the threshold (n=%d):\n%s", n, ctx.Content)
	}
}

// Check is derived from the fixer, so shapes the fixer leaves alone are not
// reported and every rewrite it would make is.
func TestArrayFormatCheckFollowsFixer(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		sort    bool
		message string // "" means no issue
		line    int
		column  int
	}{
		{name: "trailing comment", src: "a = [\"x\", \"y\"] # keep\n"},
		{name: "one-line object", src: "inputs = { azs = [\"a\", \"b\"] }\n"},
		{
			name:    "inline",
			src:     "locals {\n  azs = [\"a\", \"b\"]\n}\n",
			message: "array with 2 items should list one item per line",
			line:    2, column: 9,
		},
		{
			name:    "multiline missing comma",
			src:     "b = [\n  \"x\",\n  \"y\"\n]\n",
			message: "multiline array should list one item per line, each followed by a comma",
			line:    1, column: 5,
		},
		{
			name:    "multiline unsorted with sort",
			src:     "b = [\n  \"y\",\n  \"x\",\n]\n",
			sort:    true,
			message: "array items should be sorted",
			line:    1, column: 5,
		},
		{name: "multiline unsorted without sort", src: "b = [\n  \"y\",\n  \"x\",\n]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Rules{ArrayFormat: &config.ArrayFormatConfig{Enabled: true, Sort: tt.sort}}
			ctx := buildContext(t, tt.src, cfg)
			issues := rules.ArrayFormatRule{}.Check(ctx)
			n, err := rules.ArrayFormatRule{}.Fix(buildContext(t, tt.src, cfg))
			if err != nil {
				t.Fatal(err)
			}
			if len(issues) != n {
				t.Fatalf("check reports %d issue(s) but fix makes %d change(s): %+v", len(issues), n, issues)
			}
			if tt.message == "" {
				return
			}
			got := issues[0]
			if got.Message != tt.message || got.Location.Start.Line != tt.line ||
				got.Location.Start.Column != tt.column {
				t.Errorf("got %q at %d:%d, want %q at %d:%d", got.Message,
					got.Location.Start.Line, got.Location.Start.Column, tt.message, tt.line, tt.column)
			}
		})
	}
}
