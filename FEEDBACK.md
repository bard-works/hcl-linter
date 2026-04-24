# FEEDBACK.md - HCL Linter Code Quality Review

## Executive Summary

This document provides a comprehensive code quality review of the hcl-linter codebase, with findings ordered by criticality level (P0-P3). Each issue includes a problem description, suggested solution, and affected files.

**Review Scope**: ~40 Go source files across cmd, internal/engine, internal/rules, internal/config, internal/diag, internal/ast packages.

---

## P0 - CRITICAL (High Impact / Security / Correctness)

### Issue 1: Global Mutable Registry State

**Problem**: The `defaultRegistry` in `internal/rules/registry.go:9` is a package-level global mutable variable. This violates Go best practices and creates several problems:
- **Thread safety**: No synchronization for concurrent access
- **Test isolation**: Cannot inject mock registries for unit testing
- **Composability**: Cannot run multiple engines with different rule sets in the same process

```go
// internal/rules/registry.go:9
var defaultRegistry = &Registry{}

// internal/engine/engine.go:41
registry: rules.DefaultRegistry(),
```

**Solution**: Refactor to use dependency injection:

1. Accept registry as a constructor parameter:
   ```go
   func New(loader *config.Loader, registry *rules.Registry) *Engine {
       return &Engine{
           configLoader: loader,
           registry:     registry,
       }
   }
   ```

2. Provide a default for backward compatibility:
   ```go
   func NewWithDefaults(loader *config.Loader) *Engine {
       return New(loader, rules.DefaultRegistry())
   }
   ```

3. Use functional options for configuration:
   ```go
   type EngineOption func(*Engine)
   func WithRegistry(r *rules.Registry) EngineOption {
       return func(e *Engine) { e.registry = r }
   }
   ```

**Affected Files**:
- `internal/rules/registry.go` (line 9-12)
- `internal/engine/engine.go` (line 38-43)

---

### Issue 2: Unsafe Regex Compilation with Panic Risk

**Problem**: Using `regexp.MustCompile` at package init time creates panics if regex patterns are invalid. While currently at init time, this is unsafe for patterns that might come from configuration:

```go
// internal/rules/key_value.go:65-69
pattern = regexp.MustCompile(`^[a-z][a-zA-Z0-9]*$`)
```

```go
// internal/rules/terraform_block.go:112-113
tfVersionRe    = regexp.MustCompile(`^v?\d+\.\d+(\.\d+)?$`)
tfConstraintRe = regexp.MustCompile(`^(>=|<=|>|<|~>|!=|==)?\s*v?\d+\.\d+(\.\d+)?`)
```

```go
// internal/rules/dependency_outputs.go:152
var depOutputBlockRe = regexp.MustCompile(`^output\s+"(\w+)"`)
```

```go
// internal/rules/name_validation.go:91
regex := regexp.MustCompile(pattern)
```

**Solution**: Use `regexp.Compile` with explicit error handling:

```go
// Option 1: Compile at init with error check
var tfVersionRe *regexp.Regexp
func init() {
    var err error
    tfVersionRe, err = regexp.Compile(`^v?\d+\.\d+(\.\d+)?$`)
    if err != nil {
        panic(fmt.Sprintf("invalid regex pattern: %v", err))
    }
}

// Option 2: Validate at config loading time
func (c *Config) Validate() error {
    if c.NamePattern != "" {
        _, err := regexp.Compile(c.NamePattern)
        if err != nil {
            return fmt.Errorf("invalid name_pattern regex: %w", err)
        }
    }
    return nil
}
```

**Affected Files**:
- `internal/rules/key_value.go` (lines 65, 67, 69)
- `internal/rules/terraform_block.go` (lines 112, 113)
- `internal/rules/dependency_outputs.go` (line 152)
- `internal/rules/name_validation.go` (line 91)

---

## P1 - MAJOR (Code Quality / Maintainability)

### Issue 3: CLI Code Duplication

**Problem**: `runLint` (lines 115-228) and `runLintModeWithExitCode` (lines 127-228) in main.go share ~100 nearly-identical lines. Additionally, `run` function (lines 237-281) duplicates similar path resolution logic.

```go
// Lines 115-117
func runLint(cmd *cobra.Command, args []string) error {
    return run(cmd, args, false, false)
}

// Lines 127-228 - ~100 lines reused
func runLintModeWithExitCode(_ *cobra.Command, args []string) error { ... }

// Lines 237-281 - Similar duplication
func run(_ *cobra.Command, args []string, checkMode, fixMode bool) error { ... }
```

**Solution**: Extract common operations into shared helpers:

```go
// 1. Shared path resolution
func resolvePaths(path string, filter []string) ([]string, error) {
    // logic from runLint ModeWithExitCode lines 146-160
}

// 2. Shared config loading
func loadConfigWithResult() (*config.Loader, *config.ConfigResult) {
    // logic from getLoader() and config loading
}

// 3. Shared file filtering
func filterFilesByConfig(files []string, loader *config.Loader) []string {
    // logic from lines 164-179
}
```

**Affected Files**:
- `cmd/hcl-linter/main.go` (lines 115-228, 237-281, 340-411)

---

### Issue 4: Missing SeverityInfo Constant

**Problem**: Only two severity levels exist in `internal/diag/result.go:20-23`, but the codebase may need additional severities (info, notice):

```go
const (
    SeverityError   Severity = "error"
    SeverityWarning Severity = "warning"
)
```

**Solution**: Add missing severity constants:

```go
const (
    SeverityError   Severity = "error"
    SeverityWarning Severity = "warning"
    SeverityInfo    Severity = "info"
    SeverityNotice  Severity = "notice"
)
```

**Affected Files**:
- `internal/diag/result.go` (lines 20-23)

---

### Issue 5: Inadequate Error Wrapping

**Problem**: Errors lack context when propagated, making debugging difficult:

```go
// internal/engine/engine.go:59
return nil, fmt.Errorf("parse error: %s", diags.Error())
// Should use %w to wrap the error

// internal/engine/engine.go:167
return fmt.Errorf("parse error after fix: %s", diags.Error())
```

**Solution**: Use proper error wrapping:

```go
return nil, fmt.Errorf("parse error: %w", err)
return fmt.Errorf("parse error after fix: %w", err)
```

Also add structured error types:

```go
type ParseError struct {
    File string
    Cause error
}
func (e *ParseError) Error() string {
    return fmt.Sprintf("parse error in %s: %v", e.File, e.Cause)
}
func (e *ParseError) Unwrap() error { return e.Cause }
```

**Affected Files**:
- `internal/engine/engine.go` (lines 59, 167)
- `internal/ast/parser.go` (check for similar patterns)

---

## P2 - MODERATE (Testing / Observability)

### Issue 6: Incomplete Test Coverage

**Problem**: Several rule files lack corresponding test files. Per project convention, every rule must have a co-located test file.

Rules WITH tests:
- block_order_test.go ✓
- array_format_test.go ✓
- hcl_functions_test.go ✓
- key_value_test.go ✓
- terraform_block_test.go ✓
- remote_state_test.go ✓
- dependency_paths_test.go
- duplicates_test.go
- required_fields_test.go
- name_validation_test.go
- include_paths_test.go
- required_blocks_test.go
- count_foreach_test.go
- dependency_outputs_test.go

Rules WITHOUT tests:
- blank_lines.go (fix-only rule - should still have tests)
- name_validation.go
- duplicates.go
- include_paths.go
- dependency_paths.go
- remote_state.go (has test but missing coverage for some functions)

**Solution**: Add test files for all rules following existing patterns:

```go
// internal/rules/blank_lines_test.go
func TestBlankLinesFix(t *testing.T) {
    // Test the Fix method for blank_lines.go
}
```

**Affected Files**:
- `internal/rules/blank_lines.go` → missing `blank_lines_test.go`
- `internal/rules/name_validation.go` → missing `name_validation_test.go`
- `internal/rules/duplicates.go` → missing `duplicates_test.go`
- `internal/rules/include_paths.go` → missing `include_paths_test.go`
- `internal/rules/dependency_paths.go` → missing `dependency_paths_test.go`

---

### Issue 6b: Regex Patterns in Tests Not Covered

**Problem**: Most test files don't cover regex patterns comprehensively. For example, `terraform_block_test.go` should test edge cases in version matching.

**Solution**: Add boundary condition tests:

```go
// Test edge cases
{"v0.0.0", true},
{"v0.12.30", true},
{">=1.0.0 <2.0.0", true},
{"invalid", false},
{"", false},
```

**Affected Files**:
- `internal/rules/terraform_block_test.go`
- `internal/rules/key_value_test.go`

---

### Issue 7: No Circuit Breaker for External Calls

**Problem**: Rules like `dependency_paths` and `dependency_outputs` make HTTP/localexec calls without circuit breaker protection. If a remote endpoint is slow or down, all lint operations hang:

```go
// internal/rules/dependency_paths.go:*
// No timeout or circuit breaker
resp, err := http.DefaultClient.Do(req)
```

**Solution**: Implement circuit breaker:

```go
type CircuitBreaker struct {
    failures int
    lastFail time.Time
    mu sync.Mutex
}

func (cb *CircuitBreaker) Allow() bool {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    if cb.failures >= 3 {
        if time.Since(cb.lastFail) < time.Minute {
            return false
        }
        cb.failures = 0
    }
    return true
}

func (cb *CircuitBreaker) RecordSuccess() {
    cb.mu.Lock()
    cb.failures = 0
    cb.mu.Unlock()
}

func (cb *CircuitBreaker) RecordFailure() {
    cb.mu.Lock()
    cb.failures++
    cb.lastFail = time.Now()
    cb.mu.Unlock()
}
```

**Affected Files**:
- `internal/rules/dependency_paths.go`
- `internal/rules/dependency_outputs.go`

---

### Issue 8: Inconsistent Logging

**Problem**: No structured logging exists. All output uses `fmt.Printf` scattered throughout:

```go
fmt.Printf("Checking %d file(s)...\n", len(filesToLint))
fmt.Printf("Linting %d files...\n", ...)
```

**Solution**: Add structured logging:

```go
import "github.com/rs/zerolog"

var log = zerolog.New(os.Stdout).With().Timestamp().Logger()

log.Info().Int("files", len(filesToLint)).Msg("linting files")
log.Error().Err(err).Msg("failed to lint file")
```

**Affected Files**:
- `cmd/hcl-linter/main.go`
- `internal/engine/engine.go`

---

## P3 - MINOR (Style / Polish)

### Issue 9: Inconsistent Severity Usage

**Problem**: While constants are defined, code may use string literals directly:
```go
// Should use diag.SeverityError instead of "error"
Severity: "error",
```

**Solution**: Audit all issue creation points:

```go
// Before
Issue{ Severity: "error", ... }

// After
Issue{ Severity: diag.SeverityError, ... }
```

**Affected Files**:
- All files creating `diag.Issue` structs (grep for `Severity:`)

---

### Issue 10: Helper Duplication Between Packages

**Problem**: `internal/rules/helpers.go` may duplicate functions in `internal/ast/helpers.go`.

**Solution**: Review and consolidate:

```go
// Check for duplicate implementations
func IsTerragruntFile(path string) bool { ... } // in ast or rules?
func NormalizePath(path string) string { ... }  // in ast or rules?
```

**Affected Files**:
- `internal/rules/helpers.go`
- `internal/ast/helpers.go`

---

### Issue 11: Flag Definitions Scattered

**Problem**: Flag definitions appear in `newRootCmd` without clear grouping.

**Solution**: Group flags by command:

```go
// Group 1: Global flags
rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "...")

// Group 2: Command-specific flags
fixCmd.Flags().BoolVar(&flagFormat, "format", false, "...")
fixCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "...")
```

**Affected Files**:
- `cmd/hcl-linter/main.go` (lines 52-106)

---

## Summary: Priority Action Items

| Priority | Issue | Effort | Impact |
|----------|-------|--------|--------|
| P0 | Global Registry Refactor | Medium | Testability, thread safety |
| P0 | Unsafe Regex | Low | Crash prevention |
| P1 | CLI Duplication | Medium | Maintainability |
| P1 | Error Wrapping | Low | Debugging |
| P2 | Test Coverage | High | Code confidence |
| P2 | Circuit Breaker | Medium | Reliability |
| P3 | Logging | Medium | Observability |

---

## Recommended Order of Work

1. **Immediate**: Add regex Compile with error handling (2 files, low effort)
2. **Soon**: Refactor registry to accept injection (moderate effort, high testability gain)
3. **This Sprint**: Add missing test files (high effort, required for feature confidence)
4. **Next Sprint**: CLI deduplication + error wrapping
5. **Backlog**: Structured logging, circuit breaker

---

*Generated: 2026-04-24*
*Review Coverage: ~40 source files analyzed*