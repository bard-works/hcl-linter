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

	// Breaker guards filesystem access for rules that read paths outside the
	// file under lint (dependency_paths, dependency_outputs). It is shared
	// across all files in a run so a broken mount trips once and suspends those
	// checks for the rest of the run. May be nil, which behaves as a no-op
	// always-closed breaker.
	Breaker *CircuitBreaker
}

// Priority levels for deterministic fix pipeline ordering.
const (
	PriorityStructure = 100
	PrioritySemantic  = 200
	PriorityFormat    = 300
	PriorityFinal     = 400
)

// RuleDoc holds human-readable documentation for a rule.
type RuleDoc struct {
	Summary      string        // one-line description for the table view
	Description  string        // optional longer explanation for the detail view
	Severity     string        // "error" or "warning"
	Fixable      bool          // true when the rule also implements Fixer
	ConfigBlock  string        // HCL block name inside rules { }
	ConfigFields []ConfigField // config keys the block accepts
	Example      Example       // optional violation+fix snippet
}

// ConfigField documents one key inside a rule's config block.
type ConfigField struct {
	Name     string // HCL attribute name
	Type     string // "bool", "string", "[]string", "map[string][]string", …
	Required bool
	Default  string // string repr of default; empty when Required=true
	Doc      string // one-line description
}

// Example holds illustrative HCL snippets for a rule.
type Example struct {
	Violation string // HCL that triggers a violation; empty = no example
	Fixed     string // HCL after auto-fix; empty when Fixable=false or no example
}

// Rule is implemented by every lint rule.
type Rule interface {
	Name() string
	Priority() int
	Enabled(cfg *config.Rules) bool
	Check(ctx *Context) []diag.Issue
	Doc() RuleDoc
}

// Fixer is implemented by rules that can auto-correct violations.
// Fix mutates ctx.Content in place and returns the number of logical edits made (0 = no-op).
type Fixer interface {
	Rule
	Fix(ctx *Context) (int, error)
}
