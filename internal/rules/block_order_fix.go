package rules

import (
	"sort"
	"strings"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
)

func (r BlockOrderRule) Fix(ctx *Context) (int, error) {
	newContent := FixBlockOrder(string(ctx.Content), ctx.Blocks, ctx.Config.BlockOrder)
	if newContent == string(ctx.Content) {
		return 0, nil
	}
	ctx.Content = []byte(newContent)
	return 1, nil
}

// FixBlockOrder reorders top-level blocks according to cfg while keeping each
// block's leading comments attached and leaving interleaved non-block lines
// (attributes, blank-separated comments) at their original position.
func FixBlockOrder(content string, blocks []ast.BlockInfo, cfg *config.BlockOrderConfig) string {
	lines := strings.Split(content, "\n")
	ranges := computeBlockRanges(lines, blocks)
	segments := buildSegments(lines, ranges)
	reorderBlockSegments(segments, cfg)

	resultLines := flattenSegments(segments)
	resultLines = normalizeBlankLines(resultLines)
	resultLines = trimTrailingEmptyLines(resultLines)
	return strings.Join(resultLines, "\n") + "\n"
}

// blockRange is a block's line range after attaching its leading comments.
type blockRange struct {
	info  ast.BlockInfo
	start int
	end   int
}

func computeBlockRanges(lines []string, blocks []ast.BlockInfo) []blockRange {
	ranges := make([]blockRange, len(blocks))
	prevEnd := -1
	for i, b := range blocks {
		start := attachLeadingComments(lines, b.StartLine)
		if start <= prevEnd {
			start = prevEnd + 1
		}
		end := b.EndLine
		if end >= len(lines) {
			end = len(lines) - 1
		}
		if end < start {
			end = start
		}
		ranges[i] = blockRange{info: b, start: start, end: end}
		prevEnd = end
	}
	return ranges
}

// attachLeadingComments walks upward from a block's start line while it finds
// contiguous `#`/`//` comments or a `/* */` run, and returns the adjusted
// start line. A blank line breaks attachment.
func attachLeadingComments(lines []string, start int) int {
	line := start - 1
	for line >= 0 {
		trimmed := strings.TrimSpace(lines[line])
		if trimmed == "" {
			break
		}
		if isLineComment(trimmed) {
			line--
			continue
		}
		if strings.HasSuffix(trimmed, "*/") {
			opener := findBlockCommentOpener(lines, line)
			if opener == -1 {
				break
			}
			line = opener - 1
			continue
		}
		break
	}
	return line + 1
}

func isLineComment(trimmed string) bool {
	return strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//")
}

func findBlockCommentOpener(lines []string, endLine int) int {
	for i := endLine; i >= 0; i-- {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "/*") {
			return i
		}
	}
	return -1
}

// segment is one contiguous run of lines: either a block (with its attached
// comments) or a run of non-block lines (attributes, standalone comments).
type segment struct {
	isBlock bool
	info    ast.BlockInfo
	lines   []string
}

func buildSegments(lines []string, ranges []blockRange) []segment {
	var segments []segment
	pos := 0
	for _, r := range ranges {
		if r.start > pos {
			segments = append(segments, segment{lines: cloneLines(lines[pos:r.start])})
		}
		segments = append(segments, segment{isBlock: true, info: r.info, lines: cloneLines(lines[r.start : r.end+1])})
		pos = r.end + 1
	}
	if pos < len(lines) {
		segments = append(segments, segment{lines: cloneLines(lines[pos:])})
	}
	return segments
}

func cloneLines(lines []string) []string {
	return append([]string(nil), lines...)
}

// reorderBlockSegments sorts the content of block segments per cfg.Order and
// fills the (unchanged) block slots in that sorted order, so non-block
// segments keep their original position relative to the block sequence.
func reorderBlockSegments(segments []segment, cfg *config.BlockOrderConfig) {
	orderMap := make(map[string]int, len(cfg.Order))
	for i, name := range cfg.Order {
		orderMap[name] = i
	}

	var blockIdxs []int
	for i, seg := range segments {
		if seg.isBlock {
			blockIdxs = append(blockIdxs, i)
		}
	}

	sortOrder := make([]int, len(blockIdxs))
	for i := range sortOrder {
		sortOrder[i] = i
	}
	sort.SliceStable(sortOrder, func(a, b int) bool {
		infoA := segments[blockIdxs[sortOrder[a]]].info
		infoB := segments[blockIdxs[sortOrder[b]]].info
		return blockOrderLess(cfg.Order, orderMap, infoA, infoB)
	})

	contents := make([][]string, len(blockIdxs))
	for i, idx := range blockIdxs {
		contents[i] = segments[idx].lines
	}
	for i, segIdx := range blockIdxs {
		segments[segIdx].lines = contents[sortOrder[i]]
	}
}

func blockOrderLess(order []string, orderMap map[string]int, a, b ast.BlockInfo) bool {
	return blockOrderPosition(order, orderMap, a.Type) < blockOrderPosition(order, orderMap, b.Type)
}

func blockOrderPosition(order []string, orderMap map[string]int, blockType string) int {
	if pos, ok := orderMap[blockType]; ok {
		return pos
	}
	return len(order)
}

func flattenSegments(segments []segment) []string {
	var result []string
	for _, seg := range segments {
		result = append(result, seg.lines...)
	}
	return result
}
