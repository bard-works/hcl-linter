# HCL Linter — Claude Context

## Project

A configurable linter and auto-fixer for Terragrunt HCL files. Written in Go.
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
internal/linter/              # types only: Issue, Result, Severity
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
                        ├── TerragruntRule.Check
                        ├── TerragruntFunctionsRule.Check
                        ├── TerraformBlockRule.Check
                        ├── KeyValueRule.Check
                        ├── CountForEachRule.Check
                        └── DependencyOutputsRule.Check

Rule interface:  Name() / Enabled(cfg) / Check(ctx) []Issue
Fixer interface: Rule + Fix(ctx) ([]byte, bool, error)
Context:         FilePath, Content, File, Blocks, Attrs, Config
```

## Config format

HCL only — JSON support was intentionally removed. Config files live in
`.hcl-linter/` and are matched by filename (`terragrunt.hcl` → `.hcl-linter/terragrunt.hcl`,
fallback to `.hcl-linter/default.hcl`).

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
