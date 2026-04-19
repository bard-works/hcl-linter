package fix

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
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
		newContent := fixBlockOrder(contentStr, blocks, cfg.BlockOrder)
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
		newContent, changes := fixArrays(contentStr)
		if changes > 0 {
			contentStr = newContent
			result.Changes += changes
			parser = ast.NewParser()
			file, _ = parser.ParseContent([]byte(contentStr), path)
			blocks = ast.GetTopLevelBlocks(file)
		}
	}

	if cfg.BlankLines != nil && cfg.BlankLines.Enabled && cfg.BlankLines.WithinBlocks {
		attrs := ast.GetTopLevelAttributes(file)
		newContent, changes := fixBlankLinesWithinBlocks(contentStr, blocks, attrs)
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
	return fixBlockOrder(content, blocks, cfg)
}

func (f *Fixer) fixArrays(content string) (string, int) {
	return fixArrays(content)
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

	newContent := fixBlockOrder(contentStr, blocks, defaultFormatBlockOrder)
	if newContent != contentStr {
		contentStr = newContent
		result.Changes++
		parser = ast.NewParser()
		file, _ = parser.ParseContent([]byte(contentStr), path)
		blocks = ast.GetTopLevelBlocks(file)
	}

	newContent2, changes := fixArrays(contentStr)
	if changes > 0 {
		contentStr = newContent2
		result.Changes += changes
		parser = ast.NewParser()
		file, _ = parser.ParseContent([]byte(contentStr), path)
		blocks = ast.GetTopLevelBlocks(file)
	}

	attrs := ast.GetTopLevelAttributes(file)
	newContent3, changes2 := fixBlankLinesWithinBlocks(contentStr, blocks, attrs)
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
