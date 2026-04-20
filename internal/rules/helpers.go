package rules

import (
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
)

// hclStringValue returns the string value of a literal HCL expression, or ""
// if the expression isn't a pure literal (e.g. function calls, references).
func hclStringValue(expr hcl.Expression) string {
	val, diags := expr.Value(nil)
	if diags.HasErrors() {
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
