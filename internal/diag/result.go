package diag

import (
	"fmt"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
)

type Issue struct {
	Severity     Severity
	Rule         string
	Message      string
	Location     hcl.Range
	SuggestedFix string
}

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
	SeverityNotice  Severity = "notice"
)

type Result struct {
	File   string
	Issues []Issue
}

func (r *Result) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			return true
		}
	}
	return false
}

func (r *Result) Summary() string {
	return fmt.Sprintf("%s: %d issue(s)", filepath.Base(r.File), len(r.Issues))
}
