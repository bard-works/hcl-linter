package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"

	"github.com/bard-works/hcl-linter/internal/termcolor"
)

// diffOut is the sink for printDiff output. Tests replace this to capture
// output without touching os.Stdout.
var diffOut io.Writer = os.Stdout

// printDiff writes a colourised unified diff for one file to diffOut.
// Returns true iff before != after (i.e. something would change).
func printDiff(relPath, before, after string) bool {
	if before == after {
		return false
	}
	edits := myers.ComputeEdits(span.URIFromPath(relPath), before, after)
	unified := fmt.Sprint(gotextdiff.ToUnified("a/"+relPath, "b/"+relPath, before, edits))
	if unified == "" {
		return false
	}
	for _, line := range strings.SplitAfter(unified, "\n") {
		fmt.Fprint(diffOut, colourDiffLine(line))
	}
	return true
}

// colourDiffLine applies the termcolor semantic for a single unified-diff line.
// The `+++`/`---` header check must come before the `+`/`-` content check so
// file headers aren't styled as added/removed lines.
func colourDiffLine(line string) string {
	switch {
	case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
		return termcolor.Path(line)
	case strings.HasPrefix(line, "@@"):
		return termcolor.Rule(line)
	case strings.HasPrefix(line, "+"):
		return termcolor.Success(line)
	case strings.HasPrefix(line, "-"):
		return termcolor.Error(line)
	default:
		return line
	}
}
