---
status: active
type: implementation-contract
area: backend
---

# Phase 3: `name_validation` Fixer — Stop Corrupting Unrelated Content

**Execution Tier:** Advanced Reasoning | **Target Effort:** med
**Prerequisites:** Phase 02 Complete & Verified (panic containment protects fix runs while this fixer is reworked)

## Goal

`hcl-linter fix` with `name_validation` enabled must rename **only** (a) the target block's label token and (b) genuine references to that block (`type.label...`, `type["label"]`, `${type.label...}`), never string values, comments, other blocks' labels, or references sharing a prefix (`dependency.net` must not touch `dependency.network`). Current implementation (`internal/rules/name_validation.go:98-170`, `FixNameValidation`) does blind `strings.ReplaceAll` over the whole file — audit finding C2, silent data corruption.

## Decisions Needed (Gatekeeper Rules)

> **CRITICAL FOR AGENT:** If this section contains unresolved options, you MUST NOT write application code.
> 1. Stop execution immediately.
> 2. Present the concrete options to the user in the conversation.
> 3. Once resolved, you MUST first rewrite this file, changing the block below to `[RESOLVED: Option X]`, commit it, and only then start coding.

> **[RESOLVED: Option A]** rewrite strategy.
> - **Option A (recommended): `hclwrite`-based rename.** Parse `ctx.Content` with `hclwrite.ParseConfig`, rename via `Block.SetLabels`, rewrite traversal tokens (`hclwrite.Body` token walk for `TokenIdent` sequences `type` `.` `label`). Token-level correctness: string values and comments are structurally untouchable. Cost: token-walking code for traversals inside expressions (hclwrite exposes expression tokens as raw `Tokens`), roughly 100-150 new lines.
> - **Option B: boundary-safe regex replacement on text.** Keep the string pipeline but replace each of the four `ReplaceAll` calls with anchored regexes (label: `(?m)^(\s*` + type + `\s+)"` + QuoteMeta(old) + `"` ; dot ref: `\b` + QuoteMeta(type+"."+old) + `\b` with a lookahead-free boundary since Go RE2 lacks lookarounds — use a trailing capture group `([^\w-]|$)` and re-insert it). Smaller diff, but still text surgery: cannot distinguish a reference inside a *string literal* or comment from a real reference. Eliminates the prefix bug and unrelated-value bug for quoted labels, not the comment/string-literal cases.
>
> Phase is written for Option A. If Option B is locked, steps 2-4 are replaced by the regex table above and gate 3 (string-literal immunity) is waived — record that waiver here.

## Codebase Blueprint

### Current behavior (verified 2026-07-11)

- `Fix` (line 90): `FixNameValidation(string(ctx.Content), ctx.Blocks, ctx.Config.NameValidation)`; on change sets `ctx.Content`, returns 1.
- `FixNameValidation` (line 98): compiles `cfg.Pattern` (default `^[a-z][a-z0-9_]*$`), collects labels failing the pattern, computes `newLabel` = hyphens/spaces → underscores, then four `strings.ReplaceAll` passes over the entire content (lines 150-167) — the corruption source.
- Engine re-parses content after every fixer that reports changes (`engine.go:215` → `refreshContext`), so output must remain parseable HCL.

### File Operations

- **Modify:** `internal/rules/name_validation.go` — replace `FixNameValidation` internals; public signature may change to return `([]byte, bool)` since the only caller is `Fix` in the same file (verify with `grep -rn FixNameValidation --include='*.go'`; tests referencing it must be updated in the same commit).
- **Create:** `internal/rules/name_validation_fix.go` if the rewrite pushes `name_validation.go` past 200 lines (currently 221 — already over; do not grow it: move `Fix` + rename helpers into the new file and shrink `name_validation.go` below its current line count).
- **Modify:** `internal/rules/name_validation_test.go` — corruption regression tests.

## Step-by-Step Implementation Flow

1. Compute the rename set exactly as today (pattern-failing labels, hyphen/space → underscore), but key it by `(blockType, oldLabel)` — not by label alone — so `dependency "web"` and `include "web"` rename independently and the `cfg.Blocks` filter (blockSet) is honored per occurrence.
2. Parse with `hclwrite.ParseConfig(ctx.Content, ctx.FilePath, hcl.InitialPos)`; on diagnostics error, return unchanged (fix pipelines never receive unparseable content — engine parses first — so this is a defensive no-op path).
3. Label rename: for each top-level `*hclwrite.Block` whose `(Type(), Labels()[0])` is in the rename set, `SetLabels` with the new label. Recurse into nested blocks' bodies for nested renames only if the block type is in the rename set (matches current recursive check behavior in `nameValidationRecursive`).
4. Reference rewrite: walk every attribute's expression tokens (`Body().Attributes()` → `Expr().BuildTokens(nil)`); rewrite token runs matching:
   - `TokenIdent(type)` `TokenDot` `TokenIdent(oldLabel)` → replace the third token's bytes with the new label. Token adjacency guarantees no prefix collisions (`network` is a single ident token, never `net` + `work`).
   - `TokenIdent(type)` `TokenOBrack` `TokenQuotedLit("oldLabel")` (index form) → replace the literal.
   - Interpolation `${type.label...}` is already tokenized inside template expressions — the same ident-dot-ident sequence applies; no separate string handling.
   Apply via `Body().SetAttributeRaw(name, rewrittenTokens)` per changed attribute; recurse into nested block bodies.
5. Return `file.Bytes()` and whether any token changed. `Fix` sets `ctx.Content` and returns the number of renamed labels (not 1) so `FixResult.Changes` reflects real work — matches the `Fixer` doc contract ("number of logical edits", `rule.go:72-76`).
6. Regression tests (all must fail against the old implementation — verify by running them before the rewrite):
   - String-value immunity: `dependency "my-vpc" { config_path = "../my-vpc" }` with `inputs = { note = "my-vpc" }` → label and `dependency.my_vpc` refs renamed; both string values `"../my-vpc"` and `"my-vpc"` **unchanged** except the label position.
   - Comment immunity: `# uses my-vpc` line unchanged.
   - Prefix immunity: blocks `net` (invalid: `net-x`) and `network`; renaming `net-x` → `net_x` leaves `dependency.network.outputs.id` untouched.
   - Type scoping: with `blocks = ["dependency"]`, an `include "my-vpc"` block is untouched even though a `dependency "my-vpc"` is renamed.
   - Output re-parses: run result through `hclsyntax` parse, no diagnostics.

## Quality Gates & Validation

### 1. Static Analysis Gates
- [ ] **File Length Check:** any created file < 200 lines; `name_validation.go` net line count strictly ≤ 221 (its current size — must not grow).
- [ ] **Complexity Check:** all new/modified functions < 10 cyclomatic complexity (the token walker is the risk; split match/rewrite into separate functions).
- [ ] **Linting:** `make lint` and `make vet` clean.

### 2. Functional Testing
- [ ] `make build` zero errors.
- [ ] `make test` passes; the five regression tests above are present and were confirmed red against the pre-phase implementation (note this in the commit message).
- [ ] `hcl-linter fix --dry-run` over `docs/demo/name_validation/` produces a diff touching only labels and references (manual inspection, paste into completion note).

## Rollback Vector

Single commit; `git revert <phase-03-sha>`. Behavior-only change to fix output; no state. Reverting restores the corruption bug — do not revert without also reverting any downstream repos' auto-fixed files if they were produced in the interim.
