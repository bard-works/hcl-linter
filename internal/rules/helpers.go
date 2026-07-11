package rules

import (
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

// hclStringValue returns the string value of a literal HCL expression, or ""
// if the expression isn't a pure string literal (e.g. function calls,
// references, numbers, bools, collections).
func hclStringValue(expr hcl.Expression) string {
	val, diags := expr.Value(nil)
	if diags.HasErrors() || val.IsNull() || !val.IsKnown() || val.Type() != cty.String {
		return ""
	}
	return val.AsString()
}

// resolveRelativePath resolves inputPath relative to baseDir unless inputPath
// is already absolute.
func resolveRelativePath(baseDir, inputPath string) string {
	if filepath.IsAbs(inputPath) {
		return inputPath
	}
	return filepath.Join(baseDir, inputPath)
}
