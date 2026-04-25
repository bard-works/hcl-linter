# HCL Linter - Claude Context

## Project

Configurable HCL 2 linter and auto-fixer, written in Go. Core rules HCL-generic; Terragrunt- and Terraform-specific rule sets ship as built-ins.
Module: `github.com/bard-works/hcl-linter`

## Commands

```bash
make test            # run all tests
make build           # build to dist/hcl-linter
go test ./...        # tests with default output
go test ./internal/engine/... -v -run TestName   # single test
make lint            # run golangci-lint
make fmt && make vet # format and vet
```

Run `go test ./...` after any code change before reporting done.

## Package structure

```
cmd/hcl-linter/main.go        # CLI (cobra): lint, check, fix, version commands
internal/engine/              # Engine: entry point for all lint and fix operations
internal/rules/               # Rule implementations (Rule/Fixer interface)
internal/config/              # config loading - HCL only, file-based matching
internal/diag/                # types only: Issue, Result, Severity
internal/ast/                 # HCL parse helpers
.hcl-linter/                  # example configs shipped with the project
```

## Architecture

```
CLI ──► Engine.LintFile(path) / Engine.FixFile(path)
              │
              ├── Config Loader ──► *config.Rules
              ├── AST Parser    ──► *hcl.File, []BlockInfo, []AttrInfo
              │
              └── Registry.Enabled(cfg) ──► []Rule
                        │
                        ├── BlockOrderRule.Check(ctx)  / .Fix(ctx)
                        ├── ArrayFormatRule.Check(ctx) / .Fix(ctx)
                        ├── BlankLinesRule              .Fix(ctx)  (fix-only)
                        ├── NameValidationRule.Check   / .Fix
                        ├── RequiredFieldsRule.Check   / .Fix
                        ├── DuplicatesRule.Check
                        ├── RequiredBlocksRule.Check
                        ├── DependencyPathsRule.Check
                        ├── IncludePathsRule.Check
                        ├── RemoteStateRule.Check
                        ├── HCLFunctionsRule.Check
                        ├── TerraformBlockRule.Check
                        ├── KeyValueRule.Check
                        ├── CountForEachRule.Check
                        └── DependencyOutputsRule.Check

Rule interface:  Name() / Enabled(cfg) / Check(ctx) []Issue
Fixer interface: Rule + Fix(ctx) ([]byte, bool, error)
Context:         FilePath, Content, File, Blocks, Attrs, Config
```

## Conventions for new rules

When adding or modifying rule in `internal/rules/`:

- **Register it.** Implement `Rule` (`Name() / Priority() / Enabled(cfg) / Check(ctx) []Issue / Doc() RuleDoc`) and call `Register(FooRule{})` from `init()` in rule file. Unregistered rule = dead code. Implement `Fixer` only when issue is auto-fixable — most rules are `Check`-only.
- **Document it.** Every rule must implement `Doc() RuleDoc` with non-empty `Summary`, `Severity`, `ConfigBlock`. Include at least one `ConfigField` entry and `Example.Violation` where meaningful. `hcl-linter explain` surfaces this data directly.
- **Use `internal/ast` helpers, not `hcl/v2` directly.** Use `ast.GetTopLevelBlocks`, `ast.GetBlockAttributes`, `ast.GetBlockNestedBlocks`, `ast.GetBodyAttributes`. Rules must not walk `hclsyntax.Body` by hand except where existing helper can't express what's needed (`remote_state` nested-block walk in `remote_state.go` is rare exception).
- **Guard config access.** Both `Enabled()` and `Check()` nil-check:
  ```go
  func (r FooRule) Enabled(cfg *config.Rules) bool {
      return cfg != nil && cfg.Foo != nil && cfg.Foo.Enabled
  }
  ```
  `Check` runs only when `Enabled` returns true, but still read config fields defensively.
- **Severity constants.** Emit `diag.SeverityError` or `diag.SeverityWarning` — never raw strings.
- **Check is read-only; Fix mutates.** `Check(ctx) []diag.Issue` must not touch `ctx.Content`. `Fix(ctx) (int, error)` mutates `ctx.Content` in place, returns edit count (0 = no-op) and error.
- **Every rule has matching `_test.go`.** For every `internal/rules/<rule>.go`, must have `internal/rules/<rule>_test.go` exercising `Check` (and `Fix` if present). Test in another file doesn't count — co-locate so "is this rule tested?" is filesystem question. `/add-rule` command enforces this in step 5.

## Config format

HCL only — JSON support intentionally removed. Config files in `.hcl-linter/`, matched by filename (`terragrunt.hcl` → `.hcl-linter/terragrunt.hcl`, fallback `.hcl-linter/default.hcl`).

## Cross-platform paths

CI runs on Ubuntu, macOS, Windows (see `.github/workflows/ci.yml`). Tests assuming POSIX separators pass locally, fail on Windows.

- Never concatenate `/` into path. Use `filepath.Join(a, b)` — not `a + "/" + b`.
- Comparing paths from `filepath.Join`/`filepath.Dir` against string literals is bug: `filepath.Join("/a", "b")` = `\a\b` on Windows. In tests, build expected value same way or normalize with `filepath.ToSlash` on both sides.
- For POSIX literal in test input, wrap in `filepath.FromSlash("/a/b")` so it becomes `\a\b` on Windows.
- When embedding OS path into HCL string in tests, wrap in `filepath.ToSlash(p)`. HCL treats `\` as escape, so raw Windows paths (`C:\Users\…`) break parse. HCL, `filepath.IsAbs`, `os.Stat` on Windows all accept forward slashes — forward-slash form round-trips cleanly.

## Test conventions

Config files and lint-target files both named `terragrunt.hcl` in tests. Avoid overwriting one with other — write configs into subdirectory:

  ```go
  func newTestLoader(t *testing.T, tmpDir string) *config.Loader {
      t.Helper()
      return config.NewLoader(filepath.Join(tmpDir, ".linter-rules"))
  }

  func setupTestConfig(t *testing.T, tmpDir string, content string) {
      t.Helper()
      configDir := filepath.Join(tmpDir, ".linter-rules")
      if err := os.MkdirAll(configDir, 0o755); err != nil { t.Fatal(err) }
      if err := os.WriteFile(filepath.Join(configDir, "terragrunt.hcl"), []byte(content), 0o644); err != nil { t.Fatal(err) }
  }
  ```

HCL config content in tests:

  ```hcl
  rules {
    block_order {
      enabled = true
      order   = ["include", "locals", "terraform"]
    }
  }
  ```

## `fix --format` flag

`hcl-linter fix ./ --format` applies default formatting without config:
- Block order: `include → locals → terraform → dependency → inputs`
- Array normalization (2+ items → multiline)
- Blank line cleanup within blocks

Implemented via `Engine.FormatFixFile` / `Engine.FormatFixFiles` in `internal/engine/engine.go`.

## Do not

- Re-introduce JSON config support anywhere
- Add comments explaining *what* code does — only add when *why* is non-obvious
- Create `.md` documentation files unless explicitly asked
- Add error handling for scenarios that can't happen
- Suggest or implement features not explicitly requested