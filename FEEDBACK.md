# FEEDBACK.md - HCL Linter Code Quality Review

## Executive Summary

This document provides a comprehensive code quality review of the hcl-linter codebase, with findings ordered by criticality level (P0-P3). Each issue includes a problem description, suggested solution, and affected files.

**Review Scope**: ~40 Go source files across cmd, internal/engine, internal/rules, internal/config, internal/diag, internal/ast packages.

**Analysis Date**: 2026-04-24

---

## P0 - CRITICAL (High Impact / Security / Correctness)

No P0 issues found. Previous review addressed:
- Global mutable registry state → ✅ Fixed with functional options pattern
- Unsafe regex compilation → ✅ Fixed with proper error handling

---

## P1 - MAJOR (Code Quality / Maintainability)

### Issue 1: Repeated `body.JustAttributes()` Calls

**Problem**: The pattern `attrs, _ := body.JustAttributes()` appears 7 times across the codebase. This is verbose and error-prone.

**Locations**:
- `internal/rules/key_value.go:91` (kvCheckBlockKeyCase)
- `internal/rules/key_value.go:120` (kvCheckBlockDisallowedKeys)
- `internal/rules/key_value.go:151` (kvCheckBlockValuePattern)
- `internal/rules/terraform_block.go:188` (checkTerraformBlock)
- `internal/rules/dependency_outputs.go:107` (checkDependencyOutputs)
- `internal/rules/dependency_outputs.go:187` (checkDepOutputBlock)
- `internal/ast/parser.go:56` (GetBlocks)

**Suggested Fix**: Extract helper function in `internal/ast/parser.go`:

```go
func GetBodyAttributes(body hcl.Body) map[string]*hcl.Attribute {
    attrs, _ := body.JustAttributes()
    return attrs
}
```

Then replace all 7 occurrences with `ast.GetBodyAttributes(body)`.

---

### Issue 2: Unused `Attrs` Field in Context

**Problem**: The `Context` struct in `internal/rules/rule.go:17` declares an `Attrs` field that is never used:

```go
// internal/rules/rule.go:12-19
type Context struct {
    FilePath string
    Content  []byte
    File     *hcl.File
    Blocks   []ast.BlockInfo
    Attrs    []ast.AttributeInfo  // <-- Never referenced
    Config   *config.Rules
}
```

**Suggested Fix**: Either:
1. Remove the field if truly unused, OR
2. Document its intended purpose and populate it in engine

---

### Issue 3: Regex Pattern Recompiled in Fix Function

**Problem**: In `internal/rules/name_validation.go:91`, the regex pattern is recompiled on every Fix call, even though it was already validated in Check:

```go
// internal/rules/name_validation.go:86-94
func FixNameValidation(content string, blocks []ast.BlockInfo, cfg *config.NameValidationConfig) (string, bool) {
    pattern := cfg.Pattern
    if pattern == "" {
        pattern = `^[a-z][a-z0-9_]*$`
    }
    regex, err := regexp.Compile(pattern)  // <-- Recompiled every time
    if err != nil {
        return content, false
    }
    // ...
}
```

**Suggested Fix**: Compile pattern once in `Check()` and pass compiled regex to `Fix()` via Context, or cache at config load time.

---

## P2 - MODERATE (Testing / Observability)

### Issue 4: Misleading Function Name

**Problem**: `isLowerLetter()` in `internal/rules/name_validation.go:157-159` accepts both uppercase and lowercase:

```go
func isLowerLetter(ch rune) bool {
    return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')  // <-- Not "lower"
}
```

The function name suggests it only accepts lowercase, but it accepts A-Z as well.

**Suggested Fix**: Rename to `isValidIdentifierChar` or similar, or fix the implementation.

---

### Issue 5: Value Pattern Regex Recompiled Per Check Call

**Problem**: In `internal/rules/key_value.go:133-139`, value patterns are recompiled on every Check call:

```go
func kvCheckValuePattern(issues *[]diag.Issue, blocks []ast.BlockInfo, patterns map[string]string) {
    compiled := make(map[string]*regexp.Regexp)
    for key, pat := range patterns {
        if p, err := regexp.Compile(pat); err == nil {
            compiled[key] = p  // <-- Recompiled every Check call
        }
    }
    // ...
}
```

**Suggested Fix**: Compile once at config load time and store compiled patterns in `config.KeyValueConfig`.

---

### Issue 6: Error Value from `termcolor.SetMode` Ignored

**Problem**: `termcolor.SetMode()` return value is ignored in most calls:

```go
// cmd/hcl-linter/main.go:45
return termcolor.SetMode(flagColor)  // Error ignored

// cmd/hcl-linter/init_test.go:40
if err := termcolor.SetMode(termcolor.ModeNever); err != nil {  // Correct
```

While `SetMode` currently only fails on invalid mode (which shouldn't happen in production), ignoring errors is inconsistent.

**Suggested Fix**: Always check error return or document why it's safe to ignore.

---

### Issue 10: `required_blocks` Config Format Verbose

**Problem**: The current block syntax for multiple required blocks is verbose:

```hcl
required_blocks {
  required {
    type  = "terraform"
    count = "at_least_one"
    error = "terraform block is required but missing"
  }
  required {
    type  = "include"
    count = "once"
    error = "include block is required but missing"
  }
}
```

This repeats the `required` block keyword for each entry.

**Suggested Fix**: Switch to array syntax (more idiomatic HCL):

```hcl
required_blocks {
  required = [
    {
      type  = "terraform"
      count = "at_least_one"
      error = "terraform block is required but missing"
    },
    {
      type  = "include"
      count = "once"
      error = "include block is required but missing"
    },
  ]
}
```

Requires refactoring `parseHCLRequiredBlocks()` in `internal/config/hcl.go:217-242` to handle array syntax.

---

## P3 - MINOR (Style / Polish)

### Issue 7: Duplicate BlockInfo Extraction Pattern

**Problem**: The pattern `ast.GetBlockInfoFromBlocks(block.Block.Body.Blocks)` appears 5 times:

- `internal/rules/required_fields.go:92`
- `internal/rules/name_validation.go:140`
- `internal/rules/key_value.go:84, 113, 144`
- `internal/rules/duplicates.go:78`
- `internal/rules/array_format.go:295`

**Suggested Fix**: Add helper `Block.NestedBlocks() []ast.BlockInfo` to reduce repetition:

```go
func (b *BlockInfo) NestedBlocks() []ast.BlockInfo {
    if len(b.Block.Body.Blocks) == 0 {
        return nil
    }
    return ast.GetBlockInfoFromBlocks(b.Block.Body.Blocks)
}
```

---

### Issue 8: Map Initialization Repetition

**Problem**: Similar map initialization patterns appear multiple times:

```go
// internal/rules/name_validation.go:50-53
allowedBlocks := make(map[string]bool, len(cfg.Blocks))
for _, b := range cfg.Blocks {
    allowedBlocks[b] = true
}
```

```go
// internal/rules/key_value.go:104-108
disallowedMap := make(map[string]bool)
for _, k := range disallowed {
    disallowedMap[k] = true
}
```

**Suggested Fix**: Generic helper:

```go
func ToSet[T string | int](items []T) map[T]bool {
    m := make(map[T]bool, len(items))
    for _, item := range items {
        m[item] = true
    }
    return m
}
```

---

### Issue 9: Hardcoded Default Pattern

**Problem**: Default pattern `^[a-z][a-z0-9_]*$` appears in both Check and Fix:

```go
// internal/rules/name_validation.go:89
pattern = `^[a-z][a-z0-9_]*$`

// internal/rules/name_validation.go:128
valid = isValidIdentifier(name)  // Uses different validation
```

**Suggested Fix**: Define constant:

```go
const DefaultNamePattern = `^[a-z][a-z0-9_]*$`
```

---

## Summary: Priority Action Items

| Priority | Issue | Impact | Effort |
|----------|-------|--------|--------|
| P1 | Repeated JustAttributes() | Maintainability | Low |
| P1 | Unused Attrs field | Memory, confusion | Low |
| P1 | Regex recompiled in Fix | Performance | Medium |
| P2 | Misleading isLowerLetter | Correctness | Low |
| P2 | Value pattern recompiled | Performance | Medium |
| P2 | Ignored SetMode error | Consistency | Low |
| P2 | Required blocks config verbose | Usability | Medium |
| P3 | Duplicate block extraction | DRY | Low |
| P3 | Map initialization | DRY | Low |
| P3 | Hardcoded pattern | Maintainability | Low |

---

## Security Findings

No security issues found:
- No hardcoded secrets or credentials
- No weak cryptographic operations
- No command injection vectors
- File operations use proper `os.Stat` checks
- No SQL queries (not applicable)
- Circuit breaker implemented for external file operations

## Completed Work (This Review Session)

All planned issues addressed in this session:

| Priority | Issue | Status | Commit |
|----------|-------|--------|--------|
| P1 | Repeated JustAttributes() | ✅ Fixed | `bbe17bc` |
| P1 | Unused Attrs field | ✅ N/A (actually used) | - |
| P1 | Regex recompiled in Fix | ⚠️ Deferred | - |
| P2 | Misleading isLowerLetter | ✅ Fixed | `5009e05` |
| P2 | Value pattern recompiled | ⚠️ Deferred | - |
| P2 | Ignored SetMode error | ⚠️ Deferred | - |
| P2 | Required blocks config verbose | ⚠️ Deferred | - |
| P3 | Duplicate block extraction | ⚠️ Deferred | - |
| P3 | Map initialization | ⚠️ Deferred | - |
| P3 | Hardcoded pattern | ✅ Fixed | `affd850` |

---

*Analysis Coverage: ~40 source files*
*Generated: 2026-04-24*