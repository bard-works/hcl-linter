package rules

import (
	"github.com/hashicorp/hcl/v2"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/bard-works/hcl-linter/internal/diag"
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

// Priority levels for deterministic fix pipeline ordering.
const (
	PriorityStructure = 100
	PrioritySemantic  = 200
	PriorityFormat    = 300
	PriorityFinal     = 400
)

// Rule is implemented by every lint rule.
type Rule interface {
	Name() string
	Priority() int
	Enabled(cfg *config.Rules) bool
	Check(ctx *Context) []diag.Issue
}

// Fixer is implemented by rules that can auto-correct violations.
// Fix mutates ctx.Content in place and returns the number of logical edits made (0 = no-op).
type Fixer interface {
	Rule
	Fix(ctx *Context) (int, error)
}
