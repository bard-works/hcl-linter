package fix

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type Fixer struct {
	configLoader *config.Loader
}

func NewFixer(configLoader *config.Loader) *Fixer {
	return &Fixer{
		configLoader: configLoader,
	}
}

type FixResult struct {
	File    string
	Changes int
	Content string
	Success bool
	Error   error
}

func (f *Fixer) FixFile(path string) (*FixResult, error) {
	cfg, err := f.configLoader.LoadForFile(path)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	result := &FixResult{
		File:    path,
		Changes: 0,
	}

	parser := ast.NewParser()
	file, diags := parser.ParseFile(path)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse error: %s", diags.Error())
	}

	blocks := ast.GetTopLevelBlocks(file)
	contentStr := string(content)

	if cfg.BlockOrder != nil && cfg.BlockOrder.Enabled {
		newContent := f.fixBlockOrder(contentStr, blocks, cfg.BlockOrder)
		if newContent != contentStr {
			contentStr = newContent
			result.Changes++
			parser = ast.NewParser()
			file, _ = parser.ParseContent([]byte(contentStr), path)
			blocks = ast.GetTopLevelBlocks(file)
		}
	}

	if cfg.NameValidation != nil && cfg.NameValidation.Enabled {
		pattern := cfg.NameValidation.Pattern
		if pattern == "" {
			pattern = `^[a-z][a-z0-9_]*$`
		}
		regex := regexp.MustCompile(pattern)

		blockSet := make(map[string]bool)
		for _, b := range cfg.NameValidation.Blocks {
			blockSet[b] = true
		}

		hasChanges := false
		for _, block := range blocks {
			if !blockSet[block.Type] {
				continue
			}

			for _, label := range block.Labels {
				if !regex.MatchString(label) && strings.Contains(label, "-") {
					newLabel := strings.ReplaceAll(label, "-", "_")
					contentStr = strings.ReplaceAll(contentStr, fmt.Sprintf("%q", label), fmt.Sprintf("%q", newLabel))
					hasChanges = true
				}
			}
		}
		if hasChanges {
			result.Changes++
			parser = ast.NewParser()
			file, _ = parser.ParseContent([]byte(contentStr), path)
			blocks = ast.GetTopLevelBlocks(file)
		}
	}

	if cfg.RequiredFields != nil && cfg.RequiredFields.Include != nil && cfg.RequiredFields.Include.Expose {
		for _, block := range blocks {
			if block.Type == "include" && len(block.Labels) > 0 {
				attrs := ast.GetBlockAttributes(block.Block.Body)
				if _, ok := attrs["expose"]; !ok {
					contentStr = f.addAttributeToBlockStr(contentStr, block, "expose = true")
					result.Changes++
					parser = ast.NewParser()
					_, _ = parser.ParseContent([]byte(contentStr), path)
				}
			}
		}
	}

	if cfg.ArrayFormat != nil && cfg.ArrayFormat.Enabled {
		newContent, changes := f.fixArrays(contentStr)
		if changes > 0 {
			contentStr = newContent
			result.Changes += changes
		}
	}

	if cfg.BlankLines != nil && cfg.BlankLines.Enabled && cfg.BlankLines.WithinBlocks {
		attrs := ast.GetTopLevelAttributes(file)
		newContent, changes := f.fixBlankLinesWithinBlocks(contentStr, blocks, attrs)
		if changes > 0 {
			contentStr = newContent
			result.Changes += changes
		}
	}

	if result.Changes > 0 {
		if err := os.WriteFile(path, []byte(contentStr), 0o644); err != nil {
			return nil, err
		}
	}

	result.Content = contentStr
	result.Success = true
	return result, nil
}

func (f *Fixer) addAttributeToBlockStr(content string, block ast.BlockInfo, attr string) string {
	startLine := block.StartLine
	endLine := block.EndLine

	lines := strings.Split(content, "\n")

	if endLine >= len(lines) {
		endLine = len(lines) - 1
	}

	indent := ""
	if startLine < len(lines) {
		var indentSb161 strings.Builder
		for _, ch := range lines[startLine] {
			if ch != ' ' && ch != '\t' {
				break
			}
			indentSb161.WriteString(string(ch))
		}
		indent += indentSb161.String()
	}

	if startLine == endLine {
		line := lines[startLine]
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "{") && strings.Contains(trimmed, "}") {
			braceIdx := -1
			for i, ch := range line {
				if ch == '{' {
					braceIdx = i
					break
				}
			}
			if braceIdx >= 0 {
				beforeBrace := strings.TrimRight(line[:braceIdx], " \t")
				contentIndent := "  "
				if indent != "" {
					contentIndent = indent + "  "
				}
				var newLines []string
				newLines = append(newLines, beforeBrace+" {")
				newLines = append(newLines, contentIndent+attr)
				newLines = append(newLines, "}")
				newContent := strings.Join(newLines, "\n")
				if startLine < len(lines) {
					lines[startLine] = newContent
				}
				return strings.Join(lines, "\n")
			}
		}
	}

	insertIdx := startLine + 1
	for i := startLine + 1; i <= endLine && i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "}" || strings.HasPrefix(trimmed, "}") {
			insertIdx = i
			break
		}
	}

	contentIndent := indent + "  "
	for i := startLine + 1; i < insertIdx && i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" {
			line := lines[i]
			existingIndent := ""
			var existingIndentSb215 strings.Builder
			for _, ch := range line {
				if ch != ' ' && ch != '\t' {
					break
				}
				existingIndentSb215.WriteString(string(ch))
			}
			existingIndent += existingIndentSb215.String()
			if len(existingIndent) >= 2 {
				contentIndent = indent + existingIndent[:2]
			}
			break
		}
	}

	var newLines []string
	newLines = append(newLines, lines[:insertIdx]...)
	newLines = append(newLines, contentIndent+attr)
	if insertIdx < len(lines) {
		newLines = append(newLines, lines[insertIdx:]...)
	}

	return strings.Join(newLines, "\n")
}

func (f *Fixer) fixArrays(content string) (string, int) {
	lines := strings.Split(content, "\n")
	changes := 0
	var result strings.Builder

	i := 0
	for i < len(lines) {
		line := lines[i]

		if isArrayAssignment(line) && !isMultilineArrayStart(lines, i) {
			items := extractAndFormatArray(line)
			if len(items) >= 2 {
				result.WriteString(formatMultilineArray(line, items))
				changes++
				i++
				continue
			}
		}

		if isMultilineArrayStart(lines, i) {
			arrayLines, itemCount := collectMultilineArray(lines, i)
			if itemCount >= 2 && needsMultilineFix(arrayLines) {
				fixed := fixMultilineArray(arrayLines, line)
				result.WriteString(fixed)
				changes++
				i += len(arrayLines)
				continue
			}
			for _, l := range arrayLines {
				result.WriteString(l)
				result.WriteString("\n")
			}
			i += len(arrayLines)
			continue
		}

		result.WriteString(line)
		result.WriteString("\n")
		i++
	}

	return result.String(), changes
}

func isArrayAssignment(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.Contains(trimmed, "=") && strings.Contains(trimmed, "[") && strings.Contains(trimmed, "]")
}

func isMultilineArrayStart(lines []string, idx int) bool {
	if idx >= len(lines) {
		return false
	}
	trimmed := strings.TrimSpace(lines[idx])
	return strings.Contains(trimmed, "[") && !strings.Contains(trimmed, "]")
}

func collectMultilineArray(lines []string, start int) ([]string, int) {
	var arrayLines []string
	itemCount := 0
	braceCount := 0

	for i := start; i < len(lines); i++ {
		line := lines[i]
		arrayLines = append(arrayLines, line)

		braceCount += strings.Count(line, "[") - strings.Count(line, "]")
		if strings.Contains(line, "\"") {
			itemCount += strings.Count(line, "\"")
		}

		if braceCount <= 0 && strings.Contains(line, "]") {
			break
		}
	}

	return arrayLines, itemCount / 2
}

func extractAndFormatArray(line string) []string {
	trimmed := strings.TrimSpace(line)

	start := strings.Index(trimmed, "[")
	end := strings.LastIndex(trimmed, "]")
	if start == -1 || end == -1 || end <= start {
		return nil
	}

	inner := trimmed[start+1 : end]
	var items []string
	inQuote := false
	var current strings.Builder

	for _, ch := range inner {
		switch {
		case ch == '"':
			inQuote = !inQuote
			current.WriteRune(ch)
		case ch == ',' && !inQuote:
			item := strings.TrimSpace(current.String())
			if item != "" {
				items = append(items, item)
			}
			current.Reset()
		default:
			current.WriteRune(ch)
		}
	}

	item := strings.TrimSpace(current.String())
	if item != "" {
		items = append(items, item)
	}

	return items
}

func formatMultilineArray(line string, items []string) string {
	indent := ""
	var indentSb357 strings.Builder
	for _, ch := range line {
		if ch != ' ' && ch != '\t' {
			break
		}
		indentSb357.WriteString(string(ch))
	}
	indent += indentSb357.String()

	idx := strings.Index(line, "[")
	if idx == -1 {
		return ""
	}
	key := strings.TrimSpace(line[:idx])

	var sb strings.Builder
	sb.WriteString(indent)
	sb.WriteString(key)
	sb.WriteString(" = [\n")

	sort.Strings(items)

	for _, item := range items {
		sb.WriteString(indent)
		sb.WriteString("  ")
		sb.WriteString(strings.TrimSpace(item))
		sb.WriteString(",\n")
	}

	sb.WriteString(indent)
	sb.WriteString("]\n")

	return sb.String()
}

func needsMultilineFix(lines []string) bool {
	if len(lines) < 2 {
		return false
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, ",") {
			return false
		}
		if strings.Count(trimmed, "\"") > 0 && !strings.Contains(trimmed, "]") {
			return true
		}
	}

	return false
}

func fixMultilineArray(lines []string, firstLine string) string {
	indent := ""
	var indentSb407 strings.Builder
	for _, ch := range firstLine {
		if ch != ' ' && ch != '\t' {
			break
		}
		indentSb407.WriteString(string(ch))
	}
	indent += indentSb407.String()

	idx := strings.Index(firstLine, "[")
	if idx == -1 {
		return ""
	}
	key := strings.TrimSpace(firstLine[:idx])

	var items []string
	for _, line := range lines[1 : len(lines)-1] {
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimSuffix(trimmed, ",")
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}

	sort.Strings(items)

	var sb strings.Builder
	sb.WriteString(indent)
	sb.WriteString(key)
	sb.WriteString(" = [\n")

	for _, item := range items {
		sb.WriteString(indent)
		sb.WriteString("  ")
		sb.WriteString(item)
		sb.WriteString(",\n")
	}

	sb.WriteString(indent)
	sb.WriteString("]\n")

	return sb.String()
}

func (f *Fixer) PreviewFix(path string) (string, error) {
	cfg, err := f.configLoader.LoadForFile(path)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	result := string(content)

	if cfg.BlockOrder != nil && cfg.BlockOrder.Enabled {
		parser := ast.NewParser()
		file, diags := parser.ParseFile(path)
		if !diags.HasErrors() {
			blocks := ast.GetTopLevelBlocks(file)
			result = f.fixBlockOrder(result, blocks, cfg.BlockOrder)
		}
	}

	if cfg.NameValidation != nil && cfg.NameValidation.Enabled {
		pattern := cfg.NameValidation.Pattern
		if pattern == "" {
			pattern = `^[a-z][a-z0-9_]*$`
		}
		regex := regexp.MustCompile(pattern)

		for _, blockType := range cfg.NameValidation.Blocks {
			re := regexp.MustCompile(fmt.Sprintf(`(%s)\s+"([^"]+)"`, blockType))
			result = re.ReplaceAllStringFunc(result, func(match string) string {
				parts := re.FindStringSubmatch(match)
				if len(parts) == 3 && strings.Contains(parts[2], "-") && !regex.MatchString(parts[2]) {
					newLabel := strings.ReplaceAll(parts[2], "-", "_")
					return fmt.Sprintf("%s %q", parts[1], newLabel)
				}
				return match
			})
		}
	}

	if cfg.ArrayFormat != nil && cfg.ArrayFormat.Enabled {
		result, _ = f.fixArrays(result)
	}

	return result, nil
}

func (f *Fixer) fixBlockOrder(content string, blocks []ast.BlockInfo, cfg *config.BlockOrderConfig) string {
	lines := strings.Split(content, "\n")

	orderMap := make(map[string]int)
	for i, name := range cfg.Order {
		orderMap[name] = i
	}

	type blockWithContent struct {
		info    ast.BlockInfo
		content []string
	}

	blockContents := make([]blockWithContent, len(blocks))
	for i, block := range blocks {
		var blockLines []string
		for l := block.StartLine; l <= block.EndLine && l < len(lines); l++ {
			blockLines = append(blockLines, lines[l])
		}
		blockContents[i] = blockWithContent{block, blockLines}
	}

	sort.SliceStable(blockContents, func(i, j int) bool {
		posI := orderMap[blockContents[i].info.Type]
		posJ := orderMap[blockContents[j].info.Type]
		if posI == 0 && blockContents[i].info.Type != cfg.Order[0] {
			posI = len(cfg.Order)
		}
		if posJ == 0 && blockContents[j].info.Type != cfg.Order[0] {
			posJ = len(cfg.Order)
		}
		return posI < posJ
	})

	usedLines := make(map[int]bool)
	for _, block := range blocks {
		for l := block.StartLine; l <= block.EndLine; l++ {
			usedLines[l] = true
		}
	}

	var nonBlockLines []string
	for i, line := range lines {
		if !usedLines[i] {
			nonBlockLines = append(nonBlockLines, line)
		}
	}
	nonBlockLines = trimTrailingEmptyLines(nonBlockLines)

	var resultLines []string
	for i, bc := range blockContents {
		resultLines = append(resultLines, bc.content...)
		if i < len(blockContents)-1 {
			resultLines = append(resultLines, "")
		}
	}

	if len(nonBlockLines) > 0 {
		if len(resultLines) > 0 && strings.TrimSpace(resultLines[len(resultLines)-1]) != "" {
			resultLines = append(resultLines, "")
		}
		resultLines = append(resultLines, nonBlockLines...)
	}

	resultLines = normalizeBlankLines(resultLines)

	for len(resultLines) > 0 && strings.TrimSpace(resultLines[len(resultLines)-1]) == "" {
		resultLines = resultLines[:len(resultLines)-1]
	}

	return strings.Join(resultLines, "\n") + "\n"
}

func normalizeBlankLines(lines []string) []string {
	var result []string
	prevWasBlank := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if !prevWasBlank {
				result = append(result, line)
				prevWasBlank = true
			}
		} else {
			result = append(result, line)
			prevWasBlank = false
		}
	}
	return result
}

func trimTrailingEmptyLines(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func (f *Fixer) fixBlankLinesWithinBlocks(content string, blocks []ast.BlockInfo, attrs []ast.AttributeInfo) (string, int) {
	lines := strings.Split(content, "\n")
	changes := 0
	usedLines := make(map[int]bool)
	for _, block := range blocks {
		for l := block.StartLine; l <= block.EndLine; l++ {
			usedLines[l] = true
		}
	}
	type objectAttr struct {
		attr      ast.AttributeInfo
		firstLine int
	}
	var objectAttrs []objectAttr
	for _, attr := range attrs {
		if ast.IsObjectAttribute(attr.Expr) {
			startLine, endLine := ast.GetAttributeRange(attr.Expr)
			for l := startLine; l <= endLine; l++ {
				usedLines[l] = true
			}
			objectAttrs = append(objectAttrs, objectAttr{attr, startLine})
		}
	}
	var result []string
	processedLines := make(map[int]bool)
	i := 0
	for i < len(lines) {
		lineIdx := i
		if usedLines[lineIdx] && !processedLines[lineIdx] {
			var contentStart, contentEnd int
			var prefixLines []string
			isObjAttr := false
			for _, oa := range objectAttrs {
				if oa.firstLine != lineIdx {
					continue
				}
				_, contentEnd = ast.GetAttributeRange(oa.attr.Expr)
				prefixLines = []string{lines[lineIdx]}
				contentStart = lineIdx + 1
				isObjAttr = true
			}
			if isObjAttr {
				for j := lineIdx; j <= contentEnd; j++ {
					processedLines[j] = true
				}
				var blockLines []string
				blockLines = append(blockLines, prefixLines...)
				for j := contentStart; j <= contentEnd; j++ {
					blockLines = append(blockLines, lines[j])
				}
				fixedBlockLines := removeBlankLinesWithinBlock(blockLines)
				if len(fixedBlockLines) != len(blockLines) {
					changes++
				}
				result = append(result, fixedBlockLines...)
				i = contentEnd + 1
				continue
			}
			hasNestedBlock := false
			for _, block := range blocks {
				if block.StartLine != lineIdx {
					continue
				}
				prefixLines = []string{lines[lineIdx]}
				contentStart = blockContentStart(lines, lineIdx)
				contentEnd = block.EndLine
				for j := lineIdx; j <= contentEnd; j++ {
					processedLines[j] = true
				}
				if contentStart < contentEnd {
					for k := contentStart; k < contentEnd; k++ {
						for _, b := range blocks {
							if b.StartLine == k {
								hasNestedBlock = true
								break
							}
						}
						if hasNestedBlock {
							break
						}
					}
				}
			}
			if len(prefixLines) > 0 && contentEnd > contentStart && !hasNestedBlock {
				var blockLines []string
				blockLines = append(blockLines, prefixLines...)
				for j := contentStart; j <= contentEnd; j++ {
					blockLines = append(blockLines, lines[j])
				}
				fixedBlockLines := removeBlankLinesWithinBlock(blockLines)
				if len(fixedBlockLines) != len(blockLines) {
					changes++
				}
				result = append(result, fixedBlockLines...)
				i = contentEnd + 1
				continue
			}
		}
		result = append(result, lines[i])
		processedLines[i] = true
		i++
	}
	return strings.Join(result, "\n") + "\n", changes
}

func blockContentStart(lines []string, blockStart int) int {
	for i := blockStart; i < len(lines); i++ {
		if strings.Contains(strings.TrimSpace(lines[i]), "{") {
			return i + 1
		}
	}
	return blockStart + 1
}

func removeBlankLinesWithinBlock(lines []string) []string {
	if len(lines) < 2 {
		return lines
	}
	var result []string
	var lastWasBlank bool
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		isLastLine := i == len(lines)-1
		isFirstLine := i == 0
		isBlockHeader := isFirstLine && strings.Contains(line, "{")

		if trimmed == "" {
			if isLastLine {
				continue
			}
			nextTrimmed := strings.TrimSpace(lines[i+1])
			if nextTrimmed == "}" || strings.HasPrefix(nextTrimmed, "}") {
				continue
			}
			if isBlockHeader {
				continue
			}
			if !lastWasBlank {
				result = append(result, line)
				lastWasBlank = true
			}
		} else {
			result = append(result, line)
			lastWasBlank = false
		}
	}
	return result
}

func (f *Fixer) FixFiles(paths []string, maxConcurrency int) []*FixResult {
	if maxConcurrency <= 0 {
		maxConcurrency = runtime.NumCPU()
	}

	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	results := make(chan *FixResult, len(paths))

	for _, path := range paths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := f.FixFile(p)
			if err != nil {
				result = &FixResult{
					File:  p,
					Error: err,
				}
			}
			results <- result
		}(path)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var allResults []*FixResult
	for r := range results {
		allResults = append(allResults, r)
	}
	return allResults
}

var defaultFormatBlockOrder = &config.BlockOrderConfig{
	Enabled: true,
	Order:   []string{"include", "locals", "terraform", "dependency", "inputs"},
}

func (f *Fixer) FormatFixFile(path string) (*FixResult, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	result := &FixResult{
		File:    path,
		Changes: 0,
	}

	parser := ast.NewParser()
	file, diags := parser.ParseFile(path)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse error: %s", diags.Error())
	}

	blocks := ast.GetTopLevelBlocks(file)
	contentStr := string(content)

	newContent := f.fixBlockOrder(contentStr, blocks, defaultFormatBlockOrder)
	if newContent != contentStr {
		contentStr = newContent
		result.Changes++
		parser = ast.NewParser()
		file, _ = parser.ParseContent([]byte(contentStr), path)
		blocks = ast.GetTopLevelBlocks(file)
	}

	newContent2, changes := f.fixArrays(contentStr)
	if changes > 0 {
		contentStr = newContent2
		result.Changes += changes
		parser = ast.NewParser()
		file, _ = parser.ParseContent([]byte(contentStr), path)
		blocks = ast.GetTopLevelBlocks(file)
	}

	attrs := ast.GetTopLevelAttributes(file)
	newContent3, changes2 := f.fixBlankLinesWithinBlocks(contentStr, blocks, attrs)
	if changes2 > 0 {
		contentStr = newContent3
		result.Changes += changes2
	}

	// Apply hclwrite.Format for final whitespace cleanup
	formattedBytes := hclwrite.Format([]byte(contentStr))
	if !bytes.Equal(formattedBytes, []byte(contentStr)) {
		contentStr = string(formattedBytes)
		result.Changes++
	}

	if result.Changes > 0 {
		if err := os.WriteFile(path, []byte(contentStr), 0o644); err != nil {
			return nil, err
		}
	}

	result.Content = contentStr
	result.Success = true
	return result, nil
}

func (f *Fixer) FormatFixFiles(paths []string, maxConcurrency int) []*FixResult {
	if maxConcurrency <= 0 {
		maxConcurrency = runtime.NumCPU()
	}

	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	results := make(chan *FixResult, len(paths))

	for _, path := range paths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := f.FormatFixFile(p)
			if err != nil {
				result = &FixResult{
					File:  p,
					Error: err,
				}
			}
			results <- result
		}(path)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var allResults []*FixResult
	for r := range results {
		allResults = append(allResults, r)
	}
	return allResults
}

var _ = hclsyntax.TupleConsExpr{}
