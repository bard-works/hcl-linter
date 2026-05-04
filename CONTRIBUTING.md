# Contributing to hcl-linter

Thanks for your interest in contributing. This document covers local setup,
project conventions, and how to add a new lint rule.

## Development setup

Requirements:

- Go (see `.tool-versions` for the pinned version)
- `make`
- `golangci-lint` (for `make lint`)

Common commands:

```bash
make test            # run all tests
make build           # build to dist/hcl-linter
make lint            # run golangci-lint
make fmt && make vet # format and vet
go test ./...                                       # full test suite
go test ./internal/engine/... -v -run TestName      # single test
```

Always run `go test ./...` after any code change before opening a PR.

## Project layout

```
cmd/hcl-linter/               # CLI entrypoint (cobra): lint, check, fix, version
internal/engine/              # Engine: entry point for lint and fix operations
internal/rules/               # Rule implementations (Rule/Fixer interface)
internal/config/              # Config loading - HCL only, file-based matching
internal/diag/                # Types only: Issue, Result, Severity
internal/ast/                 # HCL parse helpers
internal/termcolor/           # ANSI colour helpers
.hcl-linter/                  # Example configs shipped with the project
docs/                         # User documentation (rules, CLI, config, architecture)
```

See [docs/architecture.md](docs/architecture.md) for the data-flow diagram
and a breakdown of the `Rule` / `Fixer` interfaces.

## Rule and Fixer interfaces

```go
// Every rule implements Rule.
type Rule interface {
    Name()    string
    Enabled(cfg *config.Rules) bool
    Check(ctx *Context) []diag.Issue
}

// Rules that can auto-correct also implement Fixer.
type Fixer interface {
    Rule
    Fix(ctx *Context) ([]byte, bool, error)
}
```

`Fix` runs in a fixed engine pipeline order (BlockOrder → NameValidation →
RequiredFields → ArrayFormat → BlankLines) because each fix re-parses the
file before the next step.

## Conventions for new rules

When adding or modifying a rule in `internal/rules/`:

- **Register it.** Implement `Rule` and add it to `engine.New()` via
  `reg.Register(...)`. A rule that isn't registered is dead code. Implement
  `Fixer` only when the issue is auto-fixable - most rules are `Check`-only.
- **Use `internal/ast` helpers, not `hcl/v2` directly.** Reach for
  `ast.GetTopLevelBlocks`, `ast.GetBlockAttributes`,
  `ast.GetBlockNestedBlocks`. Rules should not walk `hclsyntax.Body` by hand
  except where an existing helper can't express what's needed.
- **Guard config access.** Both `Enabled()` and `Check()` should nil-check:

  ```go
  func (r FooRule) Enabled(cfg *config.Rules) bool {
      return cfg != nil && cfg.Foo != nil && cfg.Foo.Enabled
  }
  ```

  `Check` runs only when `Enabled` returns true, but still read config fields
  defensively.

- **Severity constants.** Emit `diag.SeverityError` or `diag.SeverityWarning`
  - never raw strings.
- **Check is read-only; Fix mutates.** `Check(ctx) []diag.Issue` must not
  touch `ctx.Content`. `Fix(ctx) ([]byte, bool, error)` returns the new
  bytes, a `changed` bool (false = no-op, return original content), and an
  error.
- **Every rule has a co-located test.** For every
  `internal/rules/<rule>.go`, there must be an
  `internal/rules/<rule>_test.go` exercising `Check` (and `Fix`, if
  present). A test living in another file doesn't count - keep them
  co-located so "is this rule tested?" is a filesystem question.

## Config format

HCL only. JSON support was intentionally removed - do not reintroduce it.
Config files live in `.hcl-linter/` and are matched by filename
(`terragrunt.hcl` → `.hcl-linter/terragrunt.hcl`, fallback to
`.hcl-linter/default.hcl`).

## Cross-platform paths

CI runs on Ubuntu, macOS, and Windows (see `.github/workflows/ci.yml`).
Tests and code that assume POSIX separators will pass locally and fail on
Windows.

- Never concatenate `/` into a path. Use `filepath.Join(a, b)` - not
  `a + "/" + b`.
- Comparing paths from `filepath.Join`/`filepath.Dir` against string
  literals is a bug: `filepath.Join("/a", "b")` is `\a\b` on Windows. In
  tests either build the expected value the same way, or normalize with
  `filepath.ToSlash` on both sides before comparing.
- For a POSIX literal in test input, wrap it in `filepath.FromSlash("/a/b")`
  so it becomes `\a\b` on Windows.
- When embedding an OS path into HCL string content in tests, wrap it in
  `filepath.ToSlash(p)`. HCL strings treat `\` as an escape character, so
  raw Windows paths (`C:\Users\…`) break the parse. HCL, `filepath.IsAbs`,
  and `os.Stat` on Windows all accept forward slashes, so the forward-slash
  form round-trips cleanly.

## Test conventions

Config files and lint-target files are both named `terragrunt.hcl` in
tests. To avoid overwriting one with the other, write configs into a
subdirectory:

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

HCL config content in tests uses this shape:

```hcl
rules {
  block_order {
    enabled = true
    order   = ["include", "locals", "terraform"]
  }
}
```

## Pull requests

- One logical change per PR.
- Include tests for behavioural changes.
- Run `make test`, `make lint`, and `make fmt && make vet` before pushing.
- Update `CHANGELOG.md` under the `## [Unreleased]` section.
- If adding or renaming a rule, update [docs/rules.md](docs/rules.md).

## Release process

Maintainers only. See [docs/architecture.md](docs/architecture.md#build--release)
for the Makefile targets (`make tag VERSION=…`, `make release VERSION=…`).
