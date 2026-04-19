Add a new lint rule to the engine. The rule name is provided as $ARGUMENTS (e.g. `/add-rule MyNewRule`).

Steps to follow — work through them in order, running `go test ./...` after step 4:

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
    "github.com/bard-works/hcl-linter/internal/linter"
)

type MyNewRule struct{}

func (r MyNewRule) Name() string { return "my_new_rule" }

func (r MyNewRule) Enabled(cfg *config.Rules) bool {
    return cfg != nil && cfg.MyNewRule != nil && cfg.MyNewRule.Enabled
}

func (r MyNewRule) Check(ctx *Context) []linter.Issue {
    if ctx.Config.MyNewRule == nil {
        return nil
    }
    var issues []linter.Issue
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

## 5. Add tests in `internal/rules/my_new_rule_test.go`

Use the `buildContext` helper (defined in `block_order_test.go`):

```go
package rules_test

import (
    "testing"
    "github.com/bard-works/hcl-linter/internal/config"
)

func TestMyNewRule(t *testing.T) {
    cfg := &config.Rules{
        MyNewRule: &config.MyNewRuleConfig{Enabled: true},
    }
    ctx := buildContext(t, `
        // HCL content here
    `, cfg)

    rule := MyNewRule{}
    issues := rule.Check(ctx)
    if len(issues) != 1 {
        t.Errorf("expected 1 issue, got %d", len(issues))
    }
}
```

## 6. Run tests

```bash
go test ./...
```

All must be green before considering the rule done.
