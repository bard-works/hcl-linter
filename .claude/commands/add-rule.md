Add a new lint rule to the engine. The rule name is provided as $ARGUMENTS (e.g. `/add-rule MyNewRule`).

Steps to follow — work through them in order. Step 5 (tests) is **not optional**:
every `internal/rules/<rule>.go` file must have a matching `<rule>_test.go`.
Treat a rule without tests as incomplete.

## 1. Add config type in `internal/config/types.go`

Add a new `*MyNewRuleConfig` field to the `Rules` struct and define the config struct:

```go
// in Rules struct:
MyNewRule *MyNewRuleConfig `json:"my_new_rule,omitempty"`

// new type:
type MyNewRuleConfig struct {
    Enabled bool `json:"enabled"`
    // add rule-specific fields here
}
```

Also add `r.MyNewRule != nil` to the `IsEnabled()` method.

## 2. Create `internal/rules/my_new_rule.go`

Implement the `Rule` interface (and optionally `Fixer`):

```go
package rules

import (
    "github.com/bard-works/hcl-linter/internal/config"
    "github.com/bard-works/hcl-linter/internal/diag"
)

type MyNewRule struct{}

func (r MyNewRule) Name() string { return "my_new_rule" }

func (r MyNewRule) Enabled(cfg *config.Rules) bool {
    return cfg != nil && cfg.MyNewRule != nil && cfg.MyNewRule.Enabled
}

func (r MyNewRule) Check(ctx *Context) []diag.Issue {
    if ctx.Config.MyNewRule == nil {
        return nil
    }
    var issues []diag.Issue
    // implement check logic using ctx.Blocks, ctx.Attrs, ctx.File, ctx.FilePath
    return issues
}

// Only add Fix if the rule is auto-correctable:
// func (r MyNewRule) Fix(ctx *Context) ([]byte, bool, error) { ... }
```

Key context fields:
- `ctx.Blocks` — `[]ast.BlockInfo` top-level blocks
- `ctx.Attrs` — `[]ast.AttributeInfo` top-level attributes
- `ctx.File` — `*hcl.File` full parsed file
- `ctx.FilePath` — absolute file path
- `ctx.Config` — `*config.Rules`

## 3. Register in `internal/engine/engine.go`

Add to the `New()` function after the last `reg.Register(...)` call:

```go
reg.Register(rules.MyNewRule{})
```

## 4. If the rule has a Fix method, wire it into the fix pipeline

In `engine.go` `FixFile()`, add a fix step in the correct position (after the existing steps):

```go
if cfg.MyNewRule != nil && cfg.MyNewRule.Enabled {
    newContent, changed := rules.FixMyNewRule(contentStr, blocks, cfg.MyNewRule)
    if changed {
        contentStr = newContent
        result.Changes++
        parser = ast.NewParser()
        file, _ = parser.ParseContent([]byte(contentStr), path)
        blocks = ast.GetTopLevelBlocks(file)
    }
}
```

## 5. Add tests in `internal/rules/my_new_rule_test.go` (required)

The file **must** exist alongside the rule file — one `<rule>_test.go` per
`<rule>.go`. Use the `buildContext` helper (defined in `block_order_test.go`)
for in-memory content, or `buildContextFromFile` + `writeFile` (defined in
`shared_test.go`) when the rule needs a real file on disk.

```go
package rules_test

import (
    "testing"

    "github.com/bard-works/hcl-linter/internal/config"
    "github.com/bard-works/hcl-linter/internal/rules"
)

func TestMyNewRule(t *testing.T) {
    cfg := &config.Rules{
        MyNewRule: &config.MyNewRuleConfig{Enabled: true},
    }
    ctx := buildContext(t, `
        // HCL content here
    `, cfg)

    issues := rules.MyNewRule{}.Check(ctx)
    if len(issues) != 1 {
        t.Errorf("expected 1 issue, got %d", len(issues))
    }
}
```

Cover at minimum: a happy path (no issues), one violating input, and any
config toggle the rule exposes (e.g. a `require_X` sub-flag). If the rule
implements `Fix`, add a round-trip test asserting the fixed output.

## 6. Run tests and confirm coverage

```bash
go test ./...
```

All tests must be green, and the new rule file should contribute
meaningfully to coverage. Spot-check with:

```bash
go test -coverprofile=coverage.out -coverpkg=./... ./...
go tool cover -func=coverage.out | grep my_new_rule
```

The rule's `Check` (and `Fix`, if present) should be above ~80%. If it's
0%, the rule isn't being hit by any test — fix the test before calling the
rule done.
