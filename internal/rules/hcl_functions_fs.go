package rules

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// findFileInAncestors walks up from dir looking for filename, stopping at the
// boundary computed by ancestorWalkBoundary. It returns ("", nil) when the
// walk completes without finding the file, and ("", ErrCircuitOpen) when the
// breaker aborts the walk early — callers must treat that case as
// unresolvable, not as a genuine "not found".
func findFileInAncestors(cb *CircuitBreaker, dir, filename string) (string, error) {
	stop := ancestorWalkBoundary(dir)
	current := dir
	for {
		testPath := filepath.Join(current, filename)
		if _, err := cb.Stat(testPath); err == nil {
			return testPath, nil
		} else if errors.Is(err, ErrCircuitOpen) {
			return "", ErrCircuitOpen
		}

		if current == stop {
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", nil
}

// ancestorWalkBoundary returns the last directory findFileInAncestors may
// inspect: the enclosing git work tree root when one exists, else the user
// home directory when dir is under it, else "" (walk all the way to the
// filesystem root, as before this phase).
func ancestorWalkBoundary(dir string) string {
	if root := findGitRoot(dir); root != "" {
		return root
	}
	if home, err := os.UserHomeDir(); err == nil && isWithinDir(home, dir) {
		return home
	}
	return ""
}

// findGitRoot walks up from dir looking for a directory containing a `.git`
// entry. These are cheap, local lookups on the lint target's own tree, so
// they use plain os.Stat rather than the circuit breaker.
func findGitRoot(dir string) string {
	current := dir
	for {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

// isWithinDir reports whether target is dir itself or a descendant of dir.
func isWithinDir(dir, target string) bool {
	rel, err := filepath.Rel(dir, target)
	if err != nil {
		return false
	}
	return rel == "." || !strings.HasPrefix(rel, "..")
}
