# Architecture

Internal documentation for `hcl-linter` maintainers and contributors. For
user-facing docs see [rules.md](rules.md), [cli.md](cli.md), and
[configuration.md](configuration.md).

## Contents

1. [Package layout](#package-layout)
2. [Data flow](#data-flow)
3. [Rule and Fixer interfaces](#rule-and-fixer-interfaces)
4. [Implementation notes](#implementation-notes)
5. [Build & release](#build--release)

## Package layout

```
cmd/hcl-linter/main.go       # CLI entrypoint (cobra: lint, check, fix, version)
internal/
├── engine/
│   └── engine.go            # Engine: single entry point for all lint and fix ops
│                            #   LintFile / LintFiles / FixFile / FixFiles
│                            #   FormatFixFile / FormatFixFiles
├── rules/
│   ├── rule.go              # Rule, Fixer interfaces; Context struct; priority constants
│   ├── registry.go          # Registry: Register / Enabled / All / Sorted; global DefaultRegistry
│   ├── block_order.go       # BlockOrderRule       - Check + Fix
│   ├── array_format.go      # ArrayFormatRule      - Check + Fix
│   ├── blank_lines.go       # BlankLinesRule       - Fix only
│   ├── name_validation.go   # NameValidationRule   - Check + Fix
│   ├── required_fields.go   # RequiredFieldsRule   - Check + Fix
│   ├── required_blocks.go   # RequiredBlocksRule   - Check
│   ├── duplicates.go        # DuplicatesRule       - Check
│   ├── dependency_paths.go  # DependencyPathsRule  - Check
│   ├── include_paths.go     # IncludePathsRule     - Check
│   ├── remote_state.go      # RemoteStateRule      - Check
│   ├── hcl_functions.go     # HCLFunctionsRule     - Check
│   ├── terraform_block.go   # TerraformBlockRule   - Check
│   ├── key_value.go         # KeyValueRule         - Check
│   ├── count_foreach.go     # CountForEachRule     - Check
│   ├── dependency_outputs.go# DependencyOutputsRule - Check
│   └── helpers.go           # Shared helpers
├── config/
│   ├── loader.go            # Config discovery and loading
│   ├── hcl.go               # HCL config parser + extends resolution
│   └── types.go             # Config structs (Rules, BlockOrderConfig, …)
├── diag/
│   └── result.go            # Types only: Issue, Result, Severity
├── ast/
│   └── parser.go            # HCL parse helpers (blocks, attributes, expressions)
└── termcolor/               # ANSI colour helpers
```

## Data flow

```
      ┌─────────────────────────────────┐
      │          CLI (cobra)            │
      │   lint / check / fix / version  │
      └────────────────┬────────────────┘
                       │ path(s)
      ┌────────────────▼────────────────┐
      │             Engine              │
      │  buildContext(path)             │
      │    ├─ Config Loader             │
      │    │    LoadForFile → *Rules    │
      │    ├─ AST Parser                │
      │    │   ParseFile → *hcl.File    │
      │    │   GetTopLevelBlocks/Attrs  │
      │    └─ → Context{FilePath,       │
      │          Content, File,         │
      │          Blocks, Attrs, Config} │
      └────────────────┬────────────────┘
                       │ ctx
      ┌────────────────▼────────────────┐
      │    Registry.Sorted()            │
      │    rules ordered by Priority()  │
      │    filtered by Enabled(cfg)     │
      └──┬───────────┬──────┬──────┬────┘
         │           │      │      │
    ┌────▼──────┐ ┌──▼──┐ ┌─▼──┐ ┌─▼─────┐
    │BlockOrder │ │Name │ │ …  │ │Dep    │
    │Check / Fix│ │Valid│ │    │ │Outputs│
    └────┬──────┘ └──┬──┘ └─┬──┘ └─┬─────┘
         │           │      │      │
      ┌──▼───────────▼──────▼──────▼───┐
      │  []Issue (lint)                │
      │  ctx.Content mutated (fix)     │
      └────────────────────────────────┘
```

## Rule and Fixer interfaces

```go
// Every rule implements Rule.
type Rule interface {
    Name()     string
    Priority() int
    Enabled(cfg *config.Rules) bool
    Check(ctx *Context) []diag.Issue
}

// Rules that can auto-correct also implement Fixer.
type Fixer interface {
    Rule
    // Fix mutates ctx.Content in place and returns the number of logical
    // edits made (0 = no-op). The engine re-parses ctx after each non-zero
    // return so subsequent rules see an updated AST.
    Fix(ctx *Context) (int, error)
}
```

Priority constants (lower = runs first):

| Constant            | Value | Used by                                    |
|---------------------|-------|--------------------------------------------|
| `PriorityStructure` | 100   | `BlockOrderRule`                           |
| `PrioritySemantic`  | 200   | all check-only rules, `NameValidationRule`, `RequiredFieldsRule` |
| `PriorityFormat`    | 300   | `ArrayFormatRule`                          |
| `PriorityFinal`     | 400   | `BlankLinesRule`                           |

Rules register themselves into the global `DefaultRegistry()` via `init()`.
The engine's `runFixPipeline` calls `registry.Sorted()` to iterate rules in
priority order; after each `Fix` that returns `n > 0` the engine calls
`refreshContext` to re-parse `ctx.Content` so subsequent rules see a
consistent AST.

## Implementation notes

- Uses `github.com/hashicorp/hcl/v2` for parsing.
- Configuration format: HCL (same format as the files being linted).
- Coloured output lives in `internal/termcolor/`. Semantic helpers
  (`Error`, `Warning`, `Success`, `Path`, `Rule`, `Location`) wrap strings
  via `github.com/fatih/color`. `SetMode` is called once per invocation
  from cobra's `PersistentPreRunE`. One subtlety: `fatih/color.New()`
  bakes a per-instance `noColor` flag at construction time if `NO_COLOR`
  was set in env, so `SetMode` must resync each cached `*Color` via
  `EnableColor()` / `DisableColor()` - toggling the package-level
  `color.NoColor` alone is not sufficient. This is regression-tested in
  `termcolor_test.go`.

## Build & release

### Makefile targets

```bash
make build          # Build binary to dist/hcl-linter
make build-all      # Build for all platforms (darwin/linux/windows)
make test           # Run tests
make test-coverage  # Run tests with coverage report
make lint           # Run golangci-lint
make fmt            # Format code
make vet            # Run go vet
make tidy           # Tidy dependencies
make clean          # Clean build artifacts
make install        # Install to GOPATH/bin
```

### Versioning

Version is managed via the `VERSION` file:

```
0.1.0
```

Version info is injected at build time via ldflags:

- `main.Version` - from VERSION file
- `main.BuildDate` - build timestamp
- `main.GitCommit` - git SHA

### Release workflow

```bash
# Tag a release
make tag VERSION=0.1.0

# Build all platforms and generate checksums
make release VERSION=0.1.0
```

This creates:

- `dist/hcl-linter-darwin-amd64`
- `dist/hcl-linter-darwin-arm64`
- `dist/hcl-linter-linux-amd64`
- `dist/hcl-linter-linux-arm64`
- `dist/hcl-linter-windows-amd64.exe`
- `dist/*sha256` checksum files
