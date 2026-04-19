package rules

import (
	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/linter"
	"github.com/hashicorp/hcl/v2"
)

// Context holds all parsed state passed to every rule.
type Context struct {
	FilePath string
	Content  []byte
	File     *hcl.File
	Blocks   []ast.BlockInfo
	Attrs    []ast.AttributeInfo
	Config   *config.Rules
}

// Rule is implemented by every lint rule.
type Rule interface {
	Name() string
	Enabled(cfg *config.Rules) bool
	Check(ctx *Context) []linter.Issue
}

// Fixer is implemented by rules that can auto-correct violations.
// Fix returns the (possibly modified) file content, whether any change was made, and any error.
type Fixer interface {
	Rule
	Fix(ctx *Context) ([]byte, bool, error)
}
