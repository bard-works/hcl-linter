package linter

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type Linter struct {
	configLoader *config.Loader
}

func NewLinter(configLoader *config.Loader) *Linter {
	return &Linter{
		configLoader: configLoader,
	}
}

func (l *Linter) LintFile(path string) (*Result, error) {
	cfg, err := l.configLoader.LoadForFile(path)
	if err != nil {
		return nil, err
	}

	parser := ast.NewParser()
	file, diags := parser.ParseFile(path)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse error: %s", diags.Error())
	}

	result := &Result{
		File:   path,
		Issues: []Issue{},
	}

	blocks := ast.GetTopLevelBlocks(file)

	if cfg.BlockOrder != nil && cfg.BlockOrder.Enabled {
		l.checkBlockOrder(result, blocks, cfg.BlockOrder)
	}

	if cfg.ArrayFormat != nil && cfg.ArrayFormat.Enabled {
		l.checkArrayFormat(result, path, file)
	}

	if cfg.NameValidation != nil && cfg.NameValidation.Enabled {
		l.checkNameValidation(result, blocks, cfg.NameValidation)
	}

	if cfg.Duplicates != nil && cfg.Duplicates.Enabled {
		l.checkDuplicates(result, blocks, cfg.Duplicates)
	}

	if cfg.RequiredFields != nil {
		l.checkRequiredFields(result, blocks, cfg.RequiredFields)
	}

	if cfg.RequiredBlocks != nil && len(cfg.RequiredBlocks.Required) > 0 {
		l.checkRequiredBlocks(result, blocks, cfg.RequiredBlocks)
	}

	if cfg.Terragrunt != nil && cfg.Terragrunt.Enabled {
		l.checkTerragrunt(result, path, blocks, cfg.Terragrunt)
	}

	if cfg.TerragruntFunctions != nil && cfg.TerragruntFunctions.Enabled {
		l.checkTerragruntFunctions(result, path, file, cfg.TerragruntFunctions)
	}

	if cfg.TerraformBlock != nil && cfg.TerraformBlock.Enabled {
		l.checkTerraformBlock(result, path, blocks, cfg.TerraformBlock)
	}

	return result, nil
}

func (l *Linter) checkBlockOrder(result *Result, blocks []ast.BlockInfo, cfg *config.BlockOrderConfig) {
	orderMap := make(map[string]int)
	for i, name := range cfg.Order {
		orderMap[name] = i
	}

	var ordered []string

	for _, block := range blocks {
		if idx, ok := orderMap[block.Type]; ok {
			ordered = append(ordered, fmt.Sprintf("%s[%d]", block.Type, idx))
		}
	}

	expectedOrder := make([]string, 0, len(cfg.Order))
	expectedOrder = append(expectedOrder, cfg.Order...)

	seen := make(map[string]int)
	for _, block := range blocks {
		pos := len(cfg.Order)
		if idx, ok := orderMap[block.Type]; ok {
			pos = idx
		}

		if lastIdx, seenBefore := seen[block.Type]; seenBefore {
			if pos != lastIdx {
				result.Issues = append(result.Issues, Issue{
					Severity: SeverityError,
					Rule:     "block_order",
					Message:  fmt.Sprintf("block type %q appears out of order. Expected: %v", block.Type, expectedOrder),
					Location: block.Block.TypeRange,
				})
			}
		}
		seen[block.Type] = pos
	}

	if len(ordered) > 1 {
		reportedPairs := make(map[string]bool)
		for i := 1; i < len(ordered); i++ {
			curr := ordered[i]
			prev := ordered[i-1]

			currName := strings.Split(curr, "[")[0]
			prevName := strings.Split(prev, "[")[0]

			if orderMap[currName] < orderMap[prevName] {
				pairKey := currName + ":" + prevName
				if !reportedPairs[pairKey] {
					result.Issues = append(result.Issues, Issue{
						Severity: SeverityError,
						Rule:     "block_order",
						Message:  fmt.Sprintf("block %q should come before %q", currName, prevName),
					})
					reportedPairs[pairKey] = true
				}
			}
		}
	}

	if len(cfg.NestedOrder) > 0 {
		l.checkNestedBlockOrder(result, blocks, cfg.NestedOrder)
	}
}

func (l *Linter) checkNestedBlockOrder(result *Result, blocks []ast.BlockInfo, nestedOrder map[string][]string) {
	for _, block := range blocks {
		if order, ok := nestedOrder[block.Type]; ok {
			nestedBlocks := block.Block.Body.Blocks
			if len(nestedBlocks) < 2 {
				continue
			}

			orderMap := make(map[string]int)
			for i, name := range order {
				orderMap[name] = i
			}

			var orderedBlocks []*hclsyntax.Block
			for _, nb := range nestedBlocks {
				if _, ok := orderMap[nb.Type]; ok {
					orderedBlocks = append(orderedBlocks, nb)
				}
			}

			if len(orderedBlocks) > 1 {
				reportedPairs := make(map[string]bool)
				for i := 1; i < len(orderedBlocks); i++ {
					currType := orderedBlocks[i].Type
					prevType := orderedBlocks[i-1].Type

					currIdx := orderMap[currType]
					prevIdx := orderMap[prevType]

					if currIdx < prevIdx {
						pairKey := currType + ":" + prevType
						if !reportedPairs[pairKey] {
							result.Issues = append(result.Issues, Issue{
								Severity: SeverityError,
								Rule:     "block_order",
								Message:  fmt.Sprintf("nested block %q should come before %q inside %q block", currType, prevType, block.Type),
								Location: orderedBlocks[i].TypeRange,
							})
							reportedPairs[pairKey] = true
						}
					}
				}
			}
		}
	}
}

func (l *Linter) checkArrayFormat(result *Result, _ string, file *hcl.File) {
	attrs, diags := file.Body.JustAttributes()
	if diags.HasErrors() {
		return
	}

	for _, attr := range attrs {
		if expr := attr.Expr; expr != nil {
			l.checkExpressionArrayFormat(result, attr.Name, expr)
		}
	}

	for _, block := range ast.GetTopLevelBlocks(file) {
		blockBody := block.Block.Body
		blockAttrs, _ := blockBody.JustAttributes()
		for name, attr := range blockAttrs {
			l.checkExpressionArrayFormat(result, fmt.Sprintf("%s.%s", block.Type, name), attr.Expr)
		}
	}
}

func (l *Linter) checkExpressionArrayFormat(result *Result, name string, expr hcl.Expression) {
	switch e := expr.(type) {
	case *hclsyntax.TupleConsExpr:
		l.checkTupleConsExpr(result, name, e)
	case *hclsyntax.ObjectConsExpr:
		for _, item := range e.Items {
			l.checkExpressionArrayFormat(result, name, item.ValueExpr)
		}
	}
}

func (l *Linter) checkTupleConsExpr(result *Result, name string, tuple *hclsyntax.TupleConsExpr) {
	for _, item := range tuple.Exprs {
		if !isStringLiteral(item) {
			return
		}
	}

	if len(tuple.Exprs) >= 2 {
		result.Issues = append(result.Issues, Issue{
			Severity:     SeverityError,
			Rule:         "array_format",
			Message:      fmt.Sprintf("array %q should be multiline (2+ items)", name),
			Location:     tuple.Range(),
			SuggestedFix: "multiline array format",
		})
	}
}

func isStringLiteral(expr hcl.Expression) bool {
	switch e := expr.(type) {
	case *hclsyntax.LiteralValueExpr:
		return true
	case *hclsyntax.TemplateExpr:
		return true
	case *hclsyntax.TemplateWrapExpr:
		return isStringLiteral(e.Wrapped)
	}
	return false
}

func (l *Linter) checkNameValidation(result *Result, blocks []ast.BlockInfo, cfg *config.NameValidationConfig) {
	pattern := cfg.Pattern
	if pattern == "" {
		pattern = `^[a-z][a-z0-9_]*$`
	}

	regex := regexp.MustCompile(pattern)

	blockSet := make(map[string]bool)
	for _, b := range cfg.Blocks {
		blockSet[b] = true
	}

	for _, block := range blocks {
		if !blockSet[block.Type] {
			continue
		}

		for _, label := range block.Labels {
			if !regex.MatchString(label) {
				result.Issues = append(result.Issues, Issue{
					Severity: SeverityError,
					Rule:     "name_validation",
					Message:  fmt.Sprintf("%s label %q does not match pattern %q", block.Type, label, pattern),
					Location: block.Block.TypeRange,
				})
			}
		}
	}
}

func (l *Linter) checkDuplicates(result *Result, blocks []ast.BlockInfo, cfg *config.DuplicatesConfig) {
	blockSet := make(map[string]bool)
	for _, b := range cfg.Blocks {
		blockSet[b] = true
	}

	for _, blockType := range cfg.Blocks {
		if !blockSet[blockType] {
			continue
		}

		seen := make(map[string]int)
		for i, block := range blocks {
			if block.Type != blockType {
				continue
			}

			if len(block.Labels) == 0 {
				continue
			}

			label := block.Labels[0]
			if _, exists := seen[label]; exists {
				result.Issues = append(result.Issues, Issue{
					Severity: SeverityError,
					Rule:     "duplicates",
					Message:  fmt.Sprintf("duplicate %s block with label %q", blockType, label),
					Location: blocks[i].Block.TypeRange,
				})
			} else {
				seen[label] = i
			}
		}
	}
}

func (l *Linter) checkRequiredFields(result *Result, blocks []ast.BlockInfo, cfg *config.RequiredFieldsConfig) {
	for _, block := range blocks {
		if block.Type == "include" {
			if cfg.Include != nil && cfg.Include.Expose {
				attrs := ast.GetBlockAttributes(block.Block.Body)
				if _, ok := attrs["expose"]; !ok {
					result.Issues = append(result.Issues, Issue{
						Severity:     SeverityError,
						Rule:         "required_fields",
						Message:      fmt.Sprintf("include block %q missing required field 'expose'", block.Labels),
						Location:     block.Block.TypeRange,
						SuggestedFix: "expose = true",
					})
				}
			}
		}
	}
}

func (l *Linter) checkRequiredBlocks(result *Result, blocks []ast.BlockInfo, cfg *config.RequiredBlocksConfig) {
	blockCounts := make(map[string]int)
	for _, block := range blocks {
		blockCounts[block.Type]++
	}

	for _, req := range cfg.Required {
		count := blockCounts[req.Type]

		if req.Count == "once" && count != 1 {
			result.Issues = append(result.Issues, Issue{
				Severity: SeverityError,
				Rule:     "required_blocks",
				Message:  req.Error,
				Location: hcl.Range{
					Filename: result.File,
					Start:    hcl.Pos{Line: 1, Column: 1},
					End:      hcl.Pos{Line: 1, Column: 1},
				},
			})
		}
	}
}

type LintResult struct {
	Result *Result
	Error  error
}

func (l *Linter) LintFiles(paths []string, maxConcurrency int) []*Result {
	if maxConcurrency <= 0 {
		maxConcurrency = runtime.NumCPU()
	}

	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	results := make(chan *Result, len(paths))

	for _, path := range paths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := l.LintFile(p)
			if err != nil {
				result = &Result{
					File: p,
					Issues: []Issue{{
						Severity: SeverityError,
						Rule:     "linter_error",
						Message:  err.Error(),
					}},
				}
			}
			results <- result
		}(path)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var allResults []*Result
	for r := range results {
		allResults = append(allResults, r)
	}
	return allResults
}

func (l *Linter) checkTerragrunt(result *Result, filePath string, blocks []ast.BlockInfo, cfg *config.TerragruntConfig) {
	if cfg.DependencyPathExists {
		l.checkDependencyPaths(result, filePath, blocks)
	}
	if cfg.IncludePathExists {
		l.checkIncludePaths(result, filePath, blocks)
	}
	if cfg.RemoteStateConfig {
		l.checkRemoteStateConfig(result, blocks)
	}
}

func (l *Linter) checkDependencyPaths(result *Result, filePath string, blocks []ast.BlockInfo) {
	fileDir := filepath.Dir(filePath)

	for _, block := range blocks {
		if block.Type != "dependency" {
			continue
		}

		attrs := ast.GetBlockAttributes(block.Block.Body)
		if configPath, ok := attrs["config_path"]; ok {
			if pathStr := getStringValue(configPath); pathStr != "" {
				resolvedPath := resolvePath(fileDir, pathStr)
				if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
					result.Issues = append(result.Issues, Issue{
						Severity: SeverityError,
						Rule:     "dependency_path_exists",
						Message:  fmt.Sprintf("dependency %q: config_path %q does not exist", block.Labels[0], pathStr),
						Location: configPath.Range(),
					})
				}
			}
		}
	}
}

func (l *Linter) checkIncludePaths(result *Result, filePath string, blocks []ast.BlockInfo) {
	fileDir := filepath.Dir(filePath)

	for _, block := range blocks {
		if block.Type != "include" {
			continue
		}

		attrs := ast.GetBlockAttributes(block.Block.Body)
		if path, ok := attrs["path"]; ok {
			if pathStr := getStringValue(path); pathStr != "" {
				resolvedPath := resolvePath(fileDir, pathStr)
				if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
					result.Issues = append(result.Issues, Issue{
						Severity: SeverityError,
						Rule:     "include_path_exists",
						Message:  fmt.Sprintf("include path %q does not exist", pathStr),
						Location: path.Range(),
					})
				}
			}
		}
	}
}

func (l *Linter) checkRemoteStateConfig(result *Result, blocks []ast.BlockInfo) {
	for _, block := range blocks {
		if block.Type != "terraform" {
			continue
		}

		nestedBlocks := block.Block.Body.Blocks
		var hasRemoteState bool
		var remoteStateBlock *hclsyntax.Block

		for _, nested := range nestedBlocks {
			if nested.Type == "remote_state" {
				hasRemoteState = true
				remoteStateBlock = nested
				break
			}
		}

		if !hasRemoteState {
			continue
		}

		attrs := ast.GetBlockAttributes(remoteStateBlock.Body)
		if backend, ok := attrs["backend"]; ok {
			backendStr := getStringValue(backend)
			if backendStr == "" {
				result.Issues = append(result.Issues, Issue{
					Severity: SeverityError,
					Rule:     "remote_state_config",
					Message:  "remote_state block missing required 'backend' attribute",
					Location: remoteStateBlock.TypeRange,
				})
			}
		} else {
			result.Issues = append(result.Issues, Issue{
				Severity: SeverityError,
				Rule:     "remote_state_config",
				Message:  "remote_state block missing required 'backend' attribute",
				Location: remoteStateBlock.TypeRange,
			})
		}
	}
}

func getStringValue(expr hcl.Expression) string {
	val, diags := expr.Value(nil)
	if diags.HasErrors() {
		return ""
	}
	return val.AsString()
}

func resolvePath(baseDir, inputPath string) string {
	if filepath.IsAbs(inputPath) {
		return inputPath
	}
	return filepath.Join(baseDir, inputPath)
}

func (l *Linter) checkTerragruntFunctions(result *Result, filePath string, file *hcl.File, cfg *config.TerragruntFunctionsConfig) {
	fileDir := filepath.Dir(filePath)

	if cfg.FindInParentFoldersExists || cfg.GetEnvHasDefault {
		l.walkAndCheckFunctions(result, fileDir, file, cfg)
	}
}

func (l *Linter) walkAndCheckFunctions(result *Result, fileDir string, file *hcl.File, cfg *config.TerragruntFunctionsConfig) {
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return
	}

	_ = hclsyntax.Walk(body, &functionCheckWalker{
		result:  result,
		fileDir: fileDir,
		cfg:     cfg,
	})
}

type functionCheckWalker struct {
	result  *Result
	fileDir string
	cfg     *config.TerragruntFunctionsConfig
}

func (w *functionCheckWalker) Enter(node hclsyntax.Node) hcl.Diagnostics {
	funcCall, ok := node.(*hclsyntax.FunctionCallExpr)
	if !ok {
		return nil
	}

	switch funcCall.Name {
	case "find_in_parent_folders":
		if w.cfg.FindInParentFoldersExists {
			checkFindInParentFolders(w.result, w.fileDir, funcCall)
		}
	case "get_env":
		if w.cfg.GetEnvHasDefault {
			checkGetEnvHasDefault(w.result, funcCall)
		}
	}

	return nil
}

func (w *functionCheckWalker) Exit(_ hclsyntax.Node) hcl.Diagnostics {
	return nil
}

func checkFindInParentFolders(result *Result, fileDir string, funcCall *hclsyntax.FunctionCallExpr) {
	if len(funcCall.Args) == 0 {
		defaultFile := "terragrunt.hcl"
		path := findInParent(fileDir, defaultFile)
		if path == "" {
			result.Issues = append(result.Issues, Issue{
				Severity: SeverityError,
				Rule:     "find_in_parent_folders_exists",
				Message:  "find_in_parent_folders() could not find terragrunt.hcl in parent directories",
				Location: funcCall.Range(),
			})
		}
		return
	}

	firstArg := funcCall.Args[0]
	filename := getStringValue(firstArg)
	if filename == "" {
		return
	}

	path := findInParent(fileDir, filename)
	if path == "" {
		result.Issues = append(result.Issues, Issue{
			Severity: SeverityError,
			Rule:     "find_in_parent_folders_exists",
			Message:  fmt.Sprintf("find_in_parent_folders(%q) could not find file in parent directories", filename),
			Location: funcCall.Range(),
		})
	}
}

func checkGetEnvHasDefault(result *Result, funcCall *hclsyntax.FunctionCallExpr) {
	if len(funcCall.Args) < 2 {
		result.Issues = append(result.Issues, Issue{
			Severity: SeverityWarning,
			Rule:     "get_env_has_default",
			Message:  "get_env() should have a default value as second argument",
			Location: funcCall.Range(),
		})
	}
}

func findInParent(dir, filename string) string {
	current := dir
	for {
		testPath := filepath.Join(current, filename)
		if _, err := os.Stat(testPath); err == nil {
			return testPath
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return ""
}

func (l *Linter) checkTerraformBlock(result *Result, _ string, blocks []ast.BlockInfo, cfg *config.TerraformBlockConfig) {
	for _, block := range blocks {
		if block.Type != "terraform" {
			continue
		}

		attrs := ast.GetBlockAttributes(block.Block.Body)

		if cfg.SourceRequired {
			l.checkTerraformSourceRequired(result, attrs, block.Block)
		}

		if cfg.VersionFormat {
			l.checkTerraformVersionFormat(result, attrs, block.Block)
		}

		if cfg.ExtraArgumentsValid {
			l.checkTerraformExtraArguments(result, block.Block.Body, block.Block)
		}

		if cfg.NoDeprecatedFields {
			l.checkTerraformDeprecatedFields(result, block.Block.Body, block.Block)
		}
	}
}

func (l *Linter) checkTerraformSourceRequired(result *Result, attrs map[string]hcl.Expression, block *hclsyntax.Block) {
	if _, ok := attrs["source"]; !ok {
		result.Issues = append(result.Issues, Issue{
			Severity: SeverityError,
			Rule:     "terraform_source_required",
			Message:  "terraform block must have 'source' attribute",
			Location: block.TypeRange,
		})
	}
}

func (l *Linter) checkTerraformVersionFormat(result *Result, attrs map[string]hcl.Expression, _ *hclsyntax.Block) {
	if version, ok := attrs["version"]; ok {
		versionStr := getStringValue(version)
		if versionStr != "" && !isValidTerraformVersion(versionStr) {
			result.Issues = append(result.Issues, Issue{
				Severity: SeverityWarning,
				Rule:     "terraform_version_format",
				Message:  fmt.Sprintf("terraform version %q may not match expected format (e.g., >= 1.0.0)", versionStr),
				Location: version.Range(),
			})
		}
	}

	if requiredVersion, ok := attrs["required_version"]; ok {
		versionStr := getStringValue(requiredVersion)
		if versionStr != "" && !isValidTerraformVersionConstraint(versionStr) {
			result.Issues = append(result.Issues, Issue{
				Severity: SeverityWarning,
				Rule:     "terraform_version_format",
				Message:  fmt.Sprintf("terraform required_version %q may not match expected format (e.g., >= 1.0.0, < 2.0.0)", versionStr),
				Location: requiredVersion.Range(),
			})
		}
	}
}

var (
	terraformVersionRegex    = regexp.MustCompile(`^v?\d+\.\d+(\.\d+)?$`)
	terraformConstraintRegex = regexp.MustCompile(`^(>=|<=|>|<|~>|!=|==)?\s*v?\d+\.\d+(\.\d+)?`)
)

func isValidTerraformVersion(version string) bool {
	return terraformVersionRegex.MatchString(version)
}

func isValidTerraformVersionConstraint(constraint string) bool {
	parts := strings.Split(constraint, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if !terraformConstraintRegex.MatchString(part) {
			return false
		}
	}
	return true
}

func (l *Linter) checkTerraformExtraArguments(result *Result, body hcl.Body, _ *hclsyntax.Block) {
	// Try both with and without label schema
	var extraArgsBlocks []*hcl.Block

	schemaWithLabel := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "extra_arguments", LabelNames: []string{"name"}},
		},
	}
	content1, _, _ := body.PartialContent(schemaWithLabel)
	extraArgsBlocks = append(extraArgsBlocks, content1.Blocks...)

	schemaNoLabel := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "extra_arguments"},
		},
	}
	content2, _, _ := body.PartialContent(schemaNoLabel)
	extraArgsBlocks = append(extraArgsBlocks, content2.Blocks...)

	// Deduplicate by block type range to avoid processing the same block twice
	seen := make(map[string]bool)
	for _, extraBlock := range extraArgsBlocks {
		rangeKey := extraBlock.TypeRange.String()
		if seen[rangeKey] {
			continue
		}
		seen[rangeKey] = true

		attrs := ast.GetBlockAttributes(extraBlock.Body)

		// Name can be either the block label or an attribute called "name"
		hasName := len(extraBlock.Labels) > 0
		if nameAttr, ok := attrs["name"]; ok {
			nameStr := getStringValue(nameAttr)
			if nameStr != "" {
				hasName = true
			}
		}
		if !hasName {
			result.Issues = append(result.Issues, Issue{
				Severity: SeverityWarning,
				Rule:     "terraform_extra_arguments_valid",
				Message:  "extra_arguments block should have a non-empty 'name' attribute",
				Location: extraBlock.TypeRange,
			})
		}

		_, hasArguments := attrs["arguments"]
		var hasNestedBlocks bool
		if sibBody, ok := extraBlock.Body.(*hclsyntax.Body); ok {
			hasNestedBlocks = len(sibBody.Blocks) > 0
		}

		if !hasArguments && !hasNestedBlocks {
			result.Issues = append(result.Issues, Issue{
				Severity: SeverityWarning,
				Rule:     "terraform_extra_arguments_valid",
				Message:  "extra_arguments block should have 'arguments' or nested blocks",
				Location: extraBlock.TypeRange,
			})
		}
	}
}

var terraformDeprecatedFields = map[string]string{
	"terraform":   "Use 'source' instead",
	"before_hook": "Use 'before_hooks' (plural) instead",
	"after_hook":  "Use 'after_hooks' (plural) instead",
}

func (l *Linter) checkTerraformDeprecatedFields(result *Result, body hcl.Body, _ *hclsyntax.Block) {
	attrs, _ := body.JustAttributes()
	for name := range attrs {
		if msg, ok := terraformDeprecatedFields[name]; ok {
			result.Issues = append(result.Issues, Issue{
				Severity: SeverityWarning,
				Rule:     "terraform_deprecated_fields",
				Message:  fmt.Sprintf("field %q is deprecated: %s", name, msg),
				Location: attrs[name].Expr.Range(),
			})
		}
	}

	nestedBlocks := ast.GetBlockNestedBlocks(body, "before_hook")
	for _, nestedBlock := range nestedBlocks {
		result.Issues = append(result.Issues, Issue{
			Severity: SeverityWarning,
			Rule:     "terraform_deprecated_fields",
			Message:  "block 'before_hook' is deprecated: use 'before_hooks' (plural) instead",
			Location: nestedBlock.TypeRange,
		})
	}

	nestedBlocks = ast.GetBlockNestedBlocks(body, "after_hook")
	for _, nestedBlock := range nestedBlocks {
		result.Issues = append(result.Issues, Issue{
			Severity: SeverityWarning,
			Rule:     "terraform_deprecated_fields",
			Message:  "block 'after_hook' is deprecated: use 'after_hooks' (plural) instead",
			Location: nestedBlock.TypeRange,
		})
	}

	if _, ok := attrs["terraform"]; ok {
		result.Issues = append(result.Issues, Issue{
			Severity: SeverityWarning,
			Rule:     "terraform_deprecated_fields",
			Message:  "field 'terraform' is deprecated: Use 'source' instead",
			Location: attrs["terraform"].Expr.Range(),
		})
	}

	nestedTerraformBlocks := ast.GetBlockNestedBlocks(body, "terraform")
	for _, nestedBlock := range nestedTerraformBlocks {
		result.Issues = append(result.Issues, Issue{
			Severity: SeverityWarning,
			Rule:     "terraform_deprecated_fields",
			Message:  "block 'terraform' is deprecated: Use 'source' instead",
			Location: nestedBlock.TypeRange,
		})
	}
}
