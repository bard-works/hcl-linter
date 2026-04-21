# HCL Linter — Claude Context

## Project

A configurable HCL 2 linter and auto-fixer, written in Go. The core rules are
HCL-generic; Terragrunt- and Terraform-specific rule sets ship as built-ins.
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

Always run `go test ./...` after any code change before reporting done.

## Package structure

```
cmd/hcl-linter/main.go        # CLI (cobra): lint, check, fix, version commands
internal/engine/              # Engine: entry point for all lint and fix operations
internal/rules/               # Rule implementations (Rule/Fixer interface)
internal/config/              # config loading — HCL only, file-based matching
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

When adding or modifying a rule in `internal/rules/`:

- **Register it.** Implement `Rule` (`Name() / Enabled(cfg) / Check(ctx) []Issue`)
  and add it to `engine.New()` via `reg.Register(...)`. A rule that isn't
  registered is dead code. Implement `Fixer` only when the issue is
  auto-fixable — most rules are `Check`-only.
- **Use `internal/ast` helpers, not `hcl/v2` directly.** Reach for
  `ast.GetTopLevelBlocks`, `ast.GetBlockAttributes`,
  `ast.GetBlockNestedBlocks`. Rules should not walk `hclsyntax.Body` by hand
  except where an existing helper can't express what's needed (the
  `remote_state` nested-block walk in `terragrunt.go` is the rare exception).
- **Guard config access.** Both `Enabled()` and `Check()` should nil-check the
  way other rules do:
  ```go
  func (r FooRule) Enabled(cfg *config.Rules) bool {
      return cfg != nil && cfg.Foo != nil && cfg.Foo.Enabled
  }
  ```
  `Check` runs only when `Enabled` returns true, but still read config fields
  defensively.
- **Severity constants.** Emit `diag.SeverityError` or `diag.SeverityWarning`
  — never raw strings.
- **Check is read-only; Fix mutates.** `Check(ctx) []diag.Issue` must not
  touch `ctx.Content`. `Fix(ctx) ([]byte, bool, error)` returns the new bytes,
  a `changed` bool (false = no-op, return original content), and an error.
- **Every rule has a matching `_test.go`.** For every
  `internal/rules/<rule>.go`, there must be an `internal/rules/<rule>_test.go`
  that exercises `Check` (and `Fix`, if present). A test living in another
  file doesn't count — keep them co-located so "is this rule tested?" is a
  filesystem question. The `/add-rule` command enforces this in step 5.

## Config format

HCL only — JSON support was intentionally removed. Config files live in
`.hcl-linter/` and are matched by filename (`terragrunt.hcl` → `.hcl-linter/terragrunt.hcl`,
fallback to `.hcl-linter/default.hcl`).

## Cross-platform paths

CI runs on Ubuntu, macOS, and Windows (see `.github/workflows/ci.yml`). Tests and
code that assume POSIX separators will pass locally and fail on Windows.

- Never concatenate `/` into a path. Use `filepath.Join(a, b)` — not `a + "/" + b`.
- Comparing paths from `filepath.Join`/`filepath.Dir` against string literals is
  a bug: `filepath.Join("/a", "b")` is `\a\b` on Windows. In tests either build
  the expected value the same way, or normalize with `filepath.ToSlash` on both
  sides before comparing.
- For a POSIX literal in test input, wrap it in `filepath.FromSlash("/a/b")` so
  it becomes `\a\b` on Windows.
- When embedding an OS path into HCL string content in tests, wrap it in
  `filepath.ToSlash(p)`. HCL strings treat `\` as an escape character, so raw
  Windows paths (`C:\Users\…`) break the parse. HCL, `filepath.IsAbs`, and
  `os.Stat` on Windows all accept forward slashes, so the forward-slash form
  round-trips cleanly.

## Test conventions

- Config files and lint-target files are both named `terragrunt.hcl` in tests.
  To avoid overwriting one with the other, write configs into a subdirectory:

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

- HCL config content in tests uses this shape:

  ```hcl
  rules {
    block_order {
      enabled = true
      order   = ["include", "locals", "terraform"]
    }
  }
  ```

## `fix --format` flag

`hcl-linter fix ./ --format` applies default formatting without requiring any config:
- Block order: `include → locals → terraform → dependency → inputs`
- Array normalization (2+ items → multiline)
- Blank line cleanup within blocks

Implemented via `Engine.FormatFixFile` / `Engine.FormatFixFiles` in `internal/engine/engine.go`.

## Do not

- Re-introduce JSON config support anywhere
- Add comments explaining *what* code does — only add a comment when the *why* is non-obvious
- Create `.md` documentation files unless explicitly asked
- Add error handling for scenarios that can't happen
- Suggest or implement features not explicitly requested
