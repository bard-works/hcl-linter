package fix

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/papaya/hcl-linter/internal/ast"
	"github.com/papaya/hcl-linter/internal/config"
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
					contentStr = strings.ReplaceAll(contentStr, fmt.Sprintf(`"%s"`, label), fmt.Sprintf(`"%s"`, newLabel))
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
					file, _ = parser.ParseContent([]byte(contentStr), path)
					blocks = ast.GetTopLevelBlocks(file)
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

	if result.Changes > 0 {
		if err := os.WriteFile(path, []byte(contentStr), 0644); err != nil {
			return nil, err
		}
	}

	result.Content = contentStr
	result.Success = true
	return result, nil
}

func blocksMatchOrder(a, b []ast.BlockInfo) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Type != b[i].Type || len(a[i].Labels) != len(b[i].Labels) {
			return false
		}
	}
	return true
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
		for _, ch := range lines[startLine] {
			if ch == ' ' || ch == '\t' {
				indent += string(ch)
			} else {
				break
			}
		}
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
			for _, ch := range line {
				if ch == ' ' || ch == '\t' {
					existingIndent += string(ch)
				} else {
					break
				}
			}
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
		if ch == '"' {
			inQuote = !inQuote
			current.WriteRune(ch)
		} else if ch == ',' && !inQuote {
			item := strings.TrimSpace(current.String())
			if item != "" {
				items = append(items, item)
			}
			current.Reset()
		} else {
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
	for _, ch := range line {
		if ch == ' ' || ch == '\t' {
			indent += string(ch)
		} else {
			break
		}
	}

	key := strings.TrimSpace(line[:strings.Index(line, "[")])

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
	for _, ch := range firstLine {
		if ch == ' ' || ch == '\t' {
			indent += string(ch)
		} else {
			break
		}
	}

	key := strings.TrimSpace(firstLine[:strings.Index(firstLine, "[")])

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
					return fmt.Sprintf(`%s "%s"`, parts[1], newLabel)
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

	sortedBlocks := make([]ast.BlockInfo, len(blocks))
	copy(sortedBlocks, blocks)

	for i := 0; i < len(sortedBlocks); i++ {
		for j := i + 1; j < len(sortedBlocks); j++ {
			posI := len(cfg.Order)
			posJ := len(cfg.Order)
			if idx, ok := orderMap[sortedBlocks[i].Type]; ok {
				posI = idx
			}
			if idx, ok := orderMap[sortedBlocks[j].Type]; ok {
				posJ = idx
			}
			if posI > posJ {
				sortedBlocks[i], sortedBlocks[j] = sortedBlocks[j], sortedBlocks[i]
			}
		}
	}

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

	var resultLines []string
	for _, block := range sortedBlocks {
		for l := block.StartLine; l <= block.EndLine; l++ {
			resultLines = append(resultLines, lines[l])
		}
		resultLines = append(resultLines, "")
	}

	if len(nonBlockLines) > 0 && strings.TrimSpace(nonBlockLines[len(nonBlockLines)-1]) != "" {
		nonBlockLines = nonBlockLines[:len(nonBlockLines)-1]
	}
	resultLines = append(resultLines, nonBlockLines...)

	for len(resultLines) > 0 && resultLines[len(resultLines)-1] == "" {
		resultLines = resultLines[:len(resultLines)-1]
	}

	return strings.Join(resultLines, "\n")
}

var _ = hclsyntax.TupleConsExpr{}
