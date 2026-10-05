# Architecture

Internal documentation for `hcl-linter` maintainers and contributors. For
user-facing docs see [rules.md](rules.md), [cli.md](cli.md), and
[configuration.md](configuration.md).

## Contents

1. [Why this tool exists](#why-this-tool-exists)
2. [Design choices](#design-choices)
3. [Package layout](#package-layout)
4. [Data flow](#data-flow)
5. [Rule and Fixer interfaces](#rule-and-fixer-interfaces)
6. [Implementation notes](#implementation-notes)
7. [Build & release](#build--release)

## Why this tool exists

HCL is permissive. Two Terragrunt files that do the same thing can look
entirely different - different block order, different quoting, different
casing on attribute names, inline vs multiline arrays, hooks named with
hyphens or underscores, paths that point nowhere but parse cleanly. HCL
itself has no opinion on any of this, and `terraform fmt` / `terragrunt
hclfmt` only touch whitespace.

In a codebase with more than one contributor this drift compounds: diffs
get noisy, reviewers spend time on layout instead of behaviour, and a
`config_path = "../vpc"` pointing at a moved directory can sit
unnoticed until a plan fails. The usual fallback - "write it in the style
guide, enforce in review" - doesn't scale past a few contributors.

`hcl-linter` exists to move those rules out of human review and into a
deterministic, fixable check. Not as a replacement for `terraform
validate` or policy-as-code tools (OPA, Sentinel) - those check semantics
after HCL is parsed. This tool checks the file shape: ordering, naming,
required blocks and attributes, path references, and formatting. The
things that should never be a review comment.

### What it is not

- Not a formatter in the `terraform fmt` sense. It can auto-fix a fixed
  set of rules (block order, array layout, blank lines, naming), but
  most rules are `Check`-only by design - auto-"fixing" a missing
  `source` attribute or a non-existent `config_path` would hide bugs,
  not solve them.
- Not a policy engine. Rules are structural, not about which AWS regions
  are allowed or whether an IAM policy is over-broad. Use OPA / Sentinel
  / Checkov for those.
- Not a language server. No incremental parse, no LSP protocol - it
  expects to run on a whole file or a whole tree.

## Design choices

A few decisions worth calling out, because they shape how rules are
written and how the engine behaves.

- **Config is HCL, not YAML or JSON.** The files being linted are HCL;
  asking contributors to context-switch into a second config language to
  describe rules about the first was rejected. JSON support was
  prototyped and removed; do not reintroduce it.
- **Per-filename config matching, not glob patterns.** `terragrunt.hcl`
  is configured by `.hcl-linter/terragrunt.hcl`. Files of different
  roles in the same directory (`terragrunt.hcl`, `root.hcl`,
  `service.hcl`) get independent rule sets without any pattern
  language. `default.hcl` is the fallback.
- **Per-directory overrides use closest-wins, not merging.** Merging
  rule sets across levels produced rules no one had written and no one
  could debug. A nested `.hcl-linter/` replaces the parent wholesale;
  sharing happens via `extends` inside a single directory.
- **Rules self-register.** Each rule calls `Register(...)` from an
  `init()` in its own file. The engine holds no hand-maintained list -
  a rule that isn't registered is dead code, full stop. Priority is an
  integer on the `Rule` interface, so pipeline order is a property of
  the rule, not of the engine.
- **Check is read-only; Fix mutates in place.** Fix returns an edit
  count; the engine re-parses the file between rules when that count is
  non-zero, so later rules see a consistent AST. This rules out whole
  classes of "fix A broke fix B" bugs.
- **`check` reports what `fix` would change.** Every fixable rule's
  `Check` flags exactly the shapes its `Fix` rewrites, so a file that
  passes `check` is left alone by `fix`, and a fixed file passes `check`
  (enforced over `docs/demo` by `internal/engine/idempotency_test.go`).
  `fix --dry-run` additionally shows the byte-level diff.

## Package layout

```
cmd/hcl-linter/main.go        # CLI entrypoint (cobra: lint, check, fix, version)
internal/
├── engine/
│   └── engine.go             # Engine: single entry point for all lint and fix ops
│                             #   LintFile / LintFiles / FixFile / FixFiles
│                             #   FormatFixFile / FormatFixFiles
├── rules/
│   ├── rule.go               # Rule, Fixer interfaces; Context struct; priority constants
│   ├── registry.go           # Registry: Register / Enabled / All / Sorted; global DefaultRegistry
│   ├── block_order.go        # BlockOrderRule       - Check + Fix
│   ├── array_format.go       # ArrayFormatRule      - Check + Fix
│   ├── blank_lines.go        # BlankLinesRule       - Fix only
│   ├── name_validation.go    # NameValidationRule   - Check + Fix
│   ├── required_fields.go    # RequiredFieldsRule   - Check + Fix
│   ├── required_blocks.go    # RequiredBlocksRule   - Check
│   ├── duplicates.go         # DuplicatesRule       - Check
│   ├── dependency_paths.go   # DependencyPathsRule  - Check
│   ├── include_paths.go      # IncludePathsRule     - Check
│   ├── remote_state.go       # RemoteStateRule      - Check
│   ├── hcl_functions.go      # HCLFunctionsRule     - Check
│   ├── terraform_block.go    # TerraformBlockRule   - Check
│   ├── key_value.go          # KeyValueRule         - Check
│   ├── count_foreach.go      # CountForEachRule     - Check
│   ├── dependency_outputs.go # DependencyOutputsRule - Check
│   └── helpers.go            # Shared helpers
├── config/
│   ├── loader.go             # Config discovery and loading
│   ├── hcl.go                # HCL config parser + extends resolution
│   └── types.go              # Config structs (Rules, BlockOrderConfig, …)
├── diag/
│   └── result.go             # Types only: Issue, Result, Severity
├── ast/
│   └── parser.go             # HCL parse helpers (blocks, attributes, expressions)
└── termcolor/                # ANSI colour helpers
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

| Constant            | Value | Used by                                                          |
| ------------------- | ----- | ---------------------------------------------------------------- |
| `PriorityStructure` | 100   | `BlockOrderRule`                                                 |
| `PrioritySemantic`  | 200   | all check-only rules, `NameValidationRule`, `RequiredFieldsRule` |
| `PriorityFormat`    | 300   | `ArrayFormatRule`                                                |
| `PriorityFinal`     | 400   | `BlankLinesRule`                                                 |

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
make snapshot       # Build every release artifact locally (needs goreleaser)
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

Releases are versioned by git tag (`v0.1.0`). Tags with a pre-release
suffix (`v0.2.0-alpha.1`) are published as GitHub pre-releases and do not
update the Homebrew tap. The `VERSION` file only feeds `make build`.

Version info is injected at build time via ldflags:

- `main.Version` - tag without the `v`
- `main.BuildDate` - commit date
- `main.GitCommit` - full commit SHA

Builds without ldflags (`go install ...@v0.1.0`) fill these in from Go's
embedded build info, so `hcl-linter version` still reports the module
version and commit.

### Release workflow

Set `VERSION` to the release (e.g. `0.1.0`), commit it, then:

```bash
make tag VERSION=0.1.0
git push origin v0.1.0
```

`make tag` and the Release workflow both refuse a tag that differs from
`VERSION`, because the GitHub Action installs the release named there:
`uses: bard-works/hcl-linter@v0.1.0` runs hcl-linter 0.1.0.

Pushing the tag runs `.github/workflows/release.yml`, which runs the tests
and then [GoReleaser](https://goreleaser.com) (`.goreleaser.yaml`) to
publish:

- `hcl-linter_<version>_<os>_<arch>.tar.gz` (`.zip` on Windows) for
  linux/darwin/windows on amd64 and arm64, plus `checksums.txt` and a
  source archive
- a build-provenance attestation for every archive
  (`gh attestation verify <file> --repo bard-works/hcl-linter`)
- `ghcr.io/bard-works/hcl-linter:<version>` and `:latest` (linux/amd64,
  linux/arm64)
- the `hcl-linter` cask in `bard-works/homebrew-tap` (stable tags only;
  needs the `HOMEBREW_TAP_GITHUB_TOKEN` secret)

Run `make snapshot` to build the same artifacts locally without
publishing.
