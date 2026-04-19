# Architectural Refactoring Goal

Unify linter and fixer under a shared rule system to eliminate duplication,
introduce a scalable rule execution model, and improve separation of concerns.

---

## HOW TO CONTINUE THIS REFACTORING (read first each session)

### Current state (as of session 2, 2026-04-19)

Rules 1–6 are fully migrated into `internal/rules/`. Rules 7–13 still live in
`internal/linter/` and `internal/fix/` and must be migrated next.

**What exists now:**
- `internal/rules/` — rule.go, registry.go, block_order.go, array_format.go,
  name_validation.go, duplicates.go, required_fields.go, required_blocks.go
- `internal/engine/engine.go` — Engine struct; calls registered rules then old linter
- `cmd/hcl-linter/main.go` — uses Engine, no direct linter/fixer imports
- `internal/linter/linter.go` — still dispatches Terragrunt, TerraformBlock, KeyValue,
  CountForEach, DependencyOutputs (the unmigrated rules)
- `internal/fix/fixer.go` — still owns the fix pipeline; calls `rules.Fix*` for migrated
  rules and `fixBlankLinesWithinBlocks` directly for BlankLines (unmigrated)
- `internal/linter/validation.go` — DELETED
- `internal/linter/block_order.go` — DELETED
- `internal/linter/array_format.go` — DELETED

### Migration pattern (repeat this for each remaining rule)

For each rule (e.g. `BlankLinesRule`):

1. **Create** `internal/rules/<name>.go` with a struct implementing:
   - `Name() string`
   - `Enabled(cfg *config.Rules) bool`
   - `Check(ctx *Context) []linter.Issue` (if it has a linter counterpart)
   - `Fix(ctx *Context) ([]byte, bool, error)` (if it has a fixer counterpart)
   - Move logic verbatim from `internal/linter/<name>.go` and/or `internal/fix/<name>.go`
   - Any exported helper (`FixBlankLines`, `FixArrays`, etc.) stays exported for
     the transition period while `fixer.go` still calls it

2. **Register** the rule in `internal/engine/engine.go`:
   ```go
   reg.Register(rules.BlankLinesRule{})
   ```

3. **Remove** the dispatch call from `internal/linter/linter.go` (for Check rules)
   and/or replace the fix block in `internal/fix/fixer.go` with `rules.Fix<Name>(...)` 

4. **Delete** the source file(s) from `internal/linter/` and `internal/fix/`

5. **Move tests** — linter_test.go tests that call `linter.NewLinter` directly for
   the migrated rule will fail; move them to `internal/rules/<name>_test.go` using
   the `buildContext` helper (defined in `block_order_test.go`, package `rules_test`)

6. **Run** `go test ./...` — must be all green before moving to next rule

### Key architectural constraints to keep in mind

- `internal/rules` imports `internal/linter` (for Issue/Result types) — never reverse
- `internal/fix` may import `internal/rules` (fix → rules is fine)
- `internal/linter` must NOT import `internal/rules` (would be circular)
- Engine calls registered rules THEN delegates remaining to old linter — no double-counting
  because each rule is removed from linter.go dispatch when it is migrated
- BlankLinesRule has NO linter counterpart — only Fix side; `Enabled` returns false so
  Check is never called; fixer.go calls `fixBlankLinesWithinBlocks` which must move to rules
- Rules that need `*hcl.File` (Terragrunt, TerraformBlock) can get it from `ctx.File`
- `ctx.FilePath` is available for rules that need the file path (dependency path checks)

### BlankLinesRule (next up — Step 3.7)

- Source: `internal/fix/blank_lines.go` (no linter counterpart)
- It is a Fixer-only rule: implement `Fix` but NOT `Check` (or have Check return nil always)
- Implement `Enabled` as: `cfg != nil && cfg.BlankLines != nil && cfg.BlankLines.Enabled && cfg.BlankLines.WithinBlocks`
- In `fixer.go` FixFile, replace the `fixBlankLinesWithinBlocks(contentStr, blocks, attrs)`
  call with `rules.FixBlankLines(contentStr, blocks, attrs)` (export the function)
- In `FormatFixFile` in fixer.go, same substitution
- Delete `internal/fix/blank_lines.go`
- No linter_test.go tests to move (no linter counterpart exists)
- Add a test in `internal/rules/blank_lines_test.go`

### After all rules migrated (Steps 4–5)

Once rules 7–13 are done:
1. `internal/linter/linter.go` dispatch will be empty — delete all check dispatch,
   keep only the file parse + result creation as a thin stub, or delete linter entirely
   and have engine own parsing
2. `internal/fix/fixer.go` fix pipeline will be empty — engine takes it over
3. Delete `LintResult` struct (in linter.go, unused)
4. Delete `PreviewFix` method (in fixer.go, unused — check with `grep PreviewFix`)
5. Delete `SeverityInfo` constant (in result.go, unused)
6. Step 5: sort `LintFiles`/`FixFiles` output by file path (both in engine.go)
7. Step 5: fix config source non-determinism in `internal/config/`

---

## Status Legend
- [ ] TODO
- [x] DONE
- [~] IN PROGRESS

---

## Prerequisites (completed before this refactoring)

- [x] Bug: `duplicates.blocks` filter was ignored — now threads `cfg` through `checkDuplicatesImpl`
- [x] Bug: `name_validation.blocks` + `pattern` were ignored — now filters by blocks and compiles pattern
- [x] Bug: `array_format.sort` was ignored — `fixArrays` now accepts `sortItems bool`
- [x] Tests for all three bugs added to `linter_test.go` and `fixer_test.go`

---

## Step 1 — Define Rule/Fixer/Context interfaces (additive only)

**Goal:** Create the type vocabulary. No existing code changes. No behavior change.

**Files to create:**
- `internal/rules/rule.go` — `Context`, `Rule` interface, `Fixer` interface
- `internal/rules/registry.go` — `Registry` struct with `Register` + `All` methods

```go
// rule.go
package rules

type Context struct {
    FilePath string
    Content  []byte
    File     *hcl.File
    Blocks   []ast.BlockInfo
    Config   *config.Rules
}

type Rule interface {
    Name() string
    Enabled(cfg *config.Rules) bool
    Check(ctx *Context) []linter.Issue
}

type Fixer interface {
    Rule
    Fix(ctx *Context) ([]byte, bool, error) // returns new content, changed bool, error
}
```

- [x] Create `internal/rules/rule.go`
- [x] Create `internal/rules/registry.go`
- [x] `go test ./...` passes (no behavior change)

---

## Step 2 — Create Engine, wire CLI through it

**Goal:** Introduce `internal/engine/engine.go`. CLI calls engine; engine delegates to existing
linter/fixer packages. No rule migration yet — pure passthrough wrapper.

**Files to create:**
- `internal/engine/engine.go` — `Engine` struct with `LintFile`, `FixFile`, `FormatFixFile`, `LintFiles`, `FixFiles`

**Files to modify:**
- `cmd/hcl-linter/main.go` — construct `Engine` instead of `Linter`/`Fixer` directly

- [x] Create `internal/engine/engine.go`
- [x] Wire `cmd/hcl-linter/main.go` to use `Engine`
- [x] `go test ./...` passes

---

## Step 3 — Migrate rules one at a time into `internal/rules/`

Each rule gets a file in `internal/rules/`, implementing both `Rule` (Check) and `Fixer` (Fix)
using logic moved verbatim from linter/fix packages.

**Migration order:**
1. `BlockOrderRule` — from `linter/block_order.go` + `fix/block_order.go`
2. `ArrayFormatRule` — from `linter/array_format.go` + `fix/array_format.go`
3. `NameValidationRule` — from `linter/validation.go` (name section)
4. `DuplicatesRule` — from `linter/validation.go` (duplicates section)
5. `RequiredFieldsRule` — from `linter/required_fields.go`
6. `RequiredBlocksRule` — from `linter/required_blocks.go`
7. `BlankLinesRule` — from `fix/blank_lines.go` (fixer-only, no linter counterpart yet)
8. `TerragruntRule` — from `linter/terragrunt.go`
9. `TerragruntFunctionsRule` — from `linter/terragrunt_functions.go`
10. `TerraformBlockRule` — from `linter/terraform_block.go`
11. `KeyValueRule` — from `linter/key_value.go`
12. `CountForEachRule` — from `linter/count_foreach.go`
13. `DependencyOutputsRule` — from `linter/dependency_outputs.go`

For each rule:
- [x] 1. BlockOrderRule
- [x] 2. ArrayFormatRule
- [x] 3. NameValidationRule
- [x] 4. DuplicatesRule
- [x] 5. RequiredFieldsRule
- [x] 6. RequiredBlocksRule
- [ ] 7. BlankLinesRule
- [ ] 8. TerragruntRule
- [ ] 9. TerragruntFunctionsRule
- [ ] 10. TerraformBlockRule
- [ ] 11. KeyValueRule
- [ ] 12. CountForEachRule
- [ ] 13. DependencyOutputsRule

After each migration: engine calls the new rule struct; old function is deleted from linter/fix.

---

## Step 4 — Remove dead code

After all rules migrated, delete:

- `internal/linter/` — reduce to types only (`result.go`, `issue.go`) or move types to `internal/rules/`
- `internal/fix/fixer.go` — replaced by engine
- `LintResult` type (unused wrapper struct in linter.go)
- `PreviewFix` method (unused)
- `SeverityInfo` constant (unused)
- Any `GetBlockBodyAttributes`, `ParseJSON`, `checkObjectForOutputs` leftovers

- [ ] Remove `internal/linter/` rule implementations (keep types)
- [ ] Remove `internal/fix/fixer.go`
- [ ] Remove dead code items above
- [ ] `go test ./...` passes

---

## Step 5 — Fix non-determinism

- [ ] Config source selection: map iteration → explicit ordered slice in `internal/config/`
- [ ] `LintFiles` / `FixFiles` output: sort results by file path before returning

---

## Invariants to maintain throughout

- `fix` (config-driven) respects `array_format.sort` from config
- `fix --format` always sorts arrays (hardcoded `true`)
- No JSON config support reintroduced
- All existing tests pass after every step
- New rule struct tests added alongside each migration

---

## Current session progress

*(Update this section each session — what was done, what's next)*

**Session 2 (2026-04-19):** Wrote plan. Completed Steps 1–2. Migrated Rules 1–6 (BlockOrder, ArrayFormat, NameValidation, Duplicates, RequiredFields, RequiredBlocks). `validation.go` deleted entirely. `addAttributeToBlockStr` moved to `rules.addAttributeToBlock`. All tests green. Next: BlankLinesRule (Step 3.7).
