package fix

import (
	"sort"
	"strings"
)

// fixArrays converts single-line arrays with 2+ items to multiline format and sorts items.
func fixArrays(content string) (string, int) {
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

// isArrayAssignment checks if a line contains an array assignment.
func isArrayAssignment(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.Contains(trimmed, "=") && strings.Contains(trimmed, "[") && strings.Contains(trimmed, "]")
}

// isMultilineArrayStart checks if a line starts a multiline array.
func isMultilineArrayStart(lines []string, idx int) bool {
	if idx >= len(lines) {
		return false
	}
	trimmed := strings.TrimSpace(lines[idx])
	return strings.Contains(trimmed, "[") && !strings.Contains(trimmed, "]")
}

// collectMultilineArray collects all lines belonging to a multiline array.
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

// extractAndFormatArray extracts items from a single-line array assignment.
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

// formatMultilineArray formats items as a multiline array with proper indentation.
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

// needsMultilineFix checks if a multiline array needs fixing.
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

// fixMultilineArray fixes a multiline array by sorting and reformatting items.
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
