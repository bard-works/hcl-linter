---
status: active
type: implementation-contract
area: backend
---

# Phase 1: Config Parsing Panic-Proofing

**Execution Tier:** Advanced Reasoning | **Target Effort:** med
**Prerequisites:** none (first phase)

## Goal

`hcl-linter` must never panic on a syntactically valid config file containing wrong-typed attribute values (e.g. `enabled = "yes"`, `order = "not-a-list"`, `max_concurrency = true`). Every `cty.Value` conversion in `internal/config/` is type-guarded; a wrong-typed value is treated as unset (identical to today's behavior for expressions with eval errors). Reproduction case that currently panics with `panic: not bool` must instead lint normally. `internal/config/hcl.go` (447 lines) is split into files each under 200 lines.

## Decisions Needed (Gatekeeper Rules)

> **CRITICAL FOR AGENT:** If this section contains unresolved options, you MUST NOT write application code.
> 1. Stop execution immediately.
> 2. Present the concrete options to the user in the conversation.
> 3. Once resolved, you MUST first rewrite this file, changing the block below to `[RESOLVED: Option X]`, commit it, and only then start coding.

> **[RESOLVED: Option A]** Guarded-helper approach vs `gohcl` struct decoding.
> - **Option A (recommended): type-guarded helpers.** Add `internal/config/ctyutil.go` with `ctyBool/ctyString/ctyInt/ctyStringSlice/ctyStringMap/ctyStringSliceMap` and mechanically replace all ~30 unguarded call sites. Small deterministic diff, zero behavior change for valid configs, silent skip for wrong types (matching today's handling of eval-error expressions).
> - **Option B: rewrite parsing with `gohcl.DecodeBody`** into HCL-tagged structs. Deletes ~400 lines and yields "expected bool, got string" diagnostics with source ranges, but is a wholesale rewrite of `hcl.go` + `types.go` tags with a much larger blast radius and behavioral edge cases (strictness on unknown attributes) that need new decisions.
>
> Phase is written for Option A. If Option B is chosen, this contract must be redrafted first.

## Codebase Blueprint

### Schema & Structural Changes

None. No config format change; wrong-typed values become no-ops instead of panics. (Phase 10 adds *warnings* for them via the validator; out of scope here.)

### File Operations

- **Create:** `internal/config/ctyutil.go` (~70 lines) — guarded conversion helpers.
- **Create:** `internal/config/ctyutil_test.go` — table test: each helper against bool/string/number/list/map/null/unknown values.
- **Create:** `internal/config/hcl_rule_blocks.go` — receive the per-rule `parseHCL*` functions moved out of `hcl.go` so both files land < 200 lines. Pure move, no logic changes beyond the guard substitution.
- **Modify:** `internal/config/hcl.go` — keep `loadHCLConfig*`, `resolveExtendsPath`, `parseHCLRulesBlock`, `LoadConfigDir*`; replace every unguarded conversion.
- **Modify:** `internal/config/hcl_test.go` — add regression tests (see gates).

### Exact call sites to guard (verified 2026-07-11, `internal/config/hcl.go`)

`val.True()`: lines 123, 139, 150, 160, 165, 175, 193, 209, 248, 253, 258, 268, 273, 278, 283, 288, 298, 308, 318, 323, 333, 367, 372, 377, 382, 392.
`val.AsString()`: lines 47, 50 (extends), 180, 226, 231, 236, 338, 358, 406, 422.
`val.AsBigFloat()`: lines 113, 144.
`val.AsValueMap()`: lines 356, 417.
`val.AsValueSlice()`: lines 403, 419.

## Step-by-Step Implementation Flow

1. Create `internal/config/ctyutil.go`:

   ```go
   package config

   import "github.com/zclconf/go-cty/cty"

   // ctyBool returns the boolean value and true, or (false, false) when the
   // value is null, unknown, or not a bool. Callers treat "not ok" as unset.
   func ctyBool(v cty.Value) (bool, bool) {
       if v.IsNull() || !v.IsKnown() || v.Type() != cty.Bool {
           return false, false
       }
       return v.True(), true
   }

   func ctyString(v cty.Value) (string, bool) {
       if v.IsNull() || !v.IsKnown() || v.Type() != cty.String {
           return "", false
       }
       return v.AsString(), true
   }

   func ctyInt(v cty.Value) (int, bool) {
       if v.IsNull() || !v.IsKnown() || v.Type() != cty.Number {
           return 0, false
       }
       f, _ := v.AsBigFloat().Float64()
       return int(f), true
   }

   // ctyStringSlice returns string elements of a list/tuple/set value.
   // Non-string elements are skipped. Returns (nil, false) for non-collections.
   func ctyStringSlice(v cty.Value) ([]string, bool) {
       if v.IsNull() || !v.IsKnown() || !v.CanIterateElements() {
           return nil, false
       }
       var out []string
       for it := v.ElementIterator(); it.Next(); {
           _, ev := it.Element()
           if s, ok := ctyString(ev); ok {
               out = append(out, s)
           }
       }
       return out, true
   }

   // ctyStringMap returns string-valued entries of an object/map value.
   func ctyStringMap(v cty.Value) (map[string]string, bool) {
       if v.IsNull() || !v.IsKnown() || !(v.Type().IsObjectType() || v.Type().IsMapType()) {
           return nil, false
       }
       out := make(map[string]string)
       for k, ev := range v.AsValueMap() {
           if s, ok := ctyString(ev); ok {
               out[k] = s
           }
       }
       return out, true
   }

   // ctyStringSliceMap returns entries whose values are string collections
   // (used by block_order.nested_order).
   func ctyStringSliceMap(v cty.Value) (map[string][]string, bool) {
       if v.IsNull() || !v.IsKnown() || !(v.Type().IsObjectType() || v.Type().IsMapType()) {
           return nil, false
       }
       out := make(map[string][]string)
       for k, ev := range v.AsValueMap() {
           if ss, ok := ctyStringSlice(ev); ok {
           out[k] = ss
           }
       }
       return out, true
   }
   ```

2. Replace every listed call site. Substitution pattern (identical shape everywhere):

   ```go
   // BEFORE
   if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
       cfg.Enabled = val.True()
   }
   // AFTER
   if val, diags := attr.Expr.Value(nil); !diags.HasErrors() {
       if b, ok := ctyBool(val); ok {
           cfg.Enabled = b
       }
   }
   ```

   For the `extends` attribute (`hcl.go:45-54`): guard with `ctyString`; when not ok, skip the extends handling entirely (do not attempt to resolve a base path). For `hclExprToStringSlice`, `parseValuePatternAttribute`, `parseNestedOrderAttribute`: reimplement bodies on top of `ctyStringSlice` / `ctyStringMap` / `ctyStringSliceMap`, preserving the existing "return empty on eval error" behavior.

3. Move the fifteen `parseHCL<Rule>` functions plus `parseValuePatternAttribute`, `parseNestedOrderAttribute`, `hclExprToStringSlice` into new `internal/config/hcl_rule_blocks.go`. `hcl.go` retains loading, extends, dispatch, and `LoadConfigDir*`. Verify both files < 200 lines afterwards (`wc -l`).

4. Add regression tests in `internal/config/hcl_test.go` — write a temp config per case and assert `loadHCLConfig` returns without panicking and yields the zero/unset field:
   - `enabled = "yes"` in `block_order` (the reproduced crash),
   - `order = "not-a-list"`,
   - `pattern = 42` in `name_validation`,
   - `max_concurrency = true`,
   - `nested_order = ["not", "a", "map"]`,
   - `value_pattern = { a = 1 }` (non-string map value skipped, string entries kept),
   - `extends = 5` (extends ignored, local rules still parsed).

5. End-to-end confirmation: run the audit reproduction —
   `printf 'rules { block_order { enabled = "yes" } }' > /tmp/p1/.hcl-linter/default.hcl` plus a trivial `a.hcl`, then `go run ./cmd/hcl-linter lint /tmp/p1 -c /tmp/p1/.hcl-linter`; expect exit 0 and no panic.

## Quality Gates & Validation

An implementation is incomplete until ALL blocks below pass successfully.

### 1. Static Analysis Gates
- [ ] **File Length Check:** `ctyutil.go`, `hcl.go`, `hcl_rule_blocks.go` each strictly under 200 lines.
- [ ] **Complexity Check:** every new/modified function cyclomatic complexity < 10.
- [ ] **Linting:** `make lint` and `make vet` clean. If findings occur, fix code manually (no suppression comments).

### 2. Functional Testing
- [ ] `make build` passes with zero errors.
- [ ] `make test` passes, including the 7 new regression cases above.
- [ ] `grep -n 'AsString()\|\.True()\|AsValueSlice\|AsValueMap\|AsBigFloat' internal/config/*.go` returns hits **only** inside `ctyutil.go`.
- [ ] Step 5 end-to-end reproduction exits 0.

## Rollback Vector

Single commit; revert with `git revert <phase-01-sha>`. No state, no migrations. Reverting restores the panic behavior documented in the README audit evidence.
