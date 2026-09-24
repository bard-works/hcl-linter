package main

import (
	"encoding/json"
	"encoding/xml"
	"path/filepath"
	"strings"
	"testing"
)

const outputTestConfig = `rules {
  block_order {
    enabled = true
    order   = ["include", "terraform"]
  }
}
`

// lintWithOutput runs check with the given --output against a file that has
// one block_order error and returns the result stream.
func lintWithOutput(t *testing.T, format string) (string, error) {
	t.Helper()
	root := setupProject(t, "terraform {}\ninclude \"root\" {}\n", outputTestConfig)
	a, buf := newCaptureApp()
	a.configSrc = filepath.Join(root, ".hcl-linter")
	a.outputFormat = format
	err := a.runCheck(nil, []string{root})
	return buf.String(), err
}

func TestOutputJSON(t *testing.T) {
	out, err := lintWithOutput(t, outputJSON)
	if err == nil {
		t.Fatal("check should still fail on findings with --output json")
	}
	var report jsonReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, out)
	}
	if report.Summary.Errors != 1 || len(report.Issues) != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	is := report.Issues[0]
	if is.Rule != "block_order" || is.Line != 2 || !strings.HasSuffix(is.File, "terragrunt.hcl") {
		t.Errorf("unexpected issue: %+v", is)
	}
}

func TestOutputSARIF(t *testing.T) {
	out, _ := lintWithOutput(t, outputSARIF)
	var log sarifLog
	if err := json.Unmarshal([]byte(out), &log); err != nil {
		t.Fatalf("stdout is not SARIF JSON: %v\n%s", err, out)
	}
	if log.Version != "2.1.0" || len(log.Runs) != 1 {
		t.Fatalf("unexpected SARIF envelope: %+v", log)
	}
	run := log.Runs[0]
	if len(run.Results) != 1 || run.Results[0].Level != "error" || run.Results[0].Locations[0].PhysicalLocation.Region.StartLine != 2 {
		t.Fatalf("unexpected results: %+v", run.Results)
	}
	if len(run.Tool.Driver.Rules) != 1 || run.Tool.Driver.Rules[0].ID != "block_order" ||
		run.Tool.Driver.Rules[0].ShortDescription.Text == "block_order" {
		t.Errorf("rule metadata should carry the rule's summary: %+v", run.Tool.Driver.Rules)
	}
	if uri := run.Results[0].Locations[0].PhysicalLocation.ArtifactLocation.URI; !strings.HasPrefix(uri, "file:///") ||
		strings.Contains(uri, `\`) {
		t.Errorf("targets outside the working directory need a file:// URI with forward slashes, got %q", uri)
	}
}

func TestOutputCheckstyle(t *testing.T) {
	out, _ := lintWithOutput(t, outputCheckstyle)
	var report checkstyleReport
	if err := xml.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("stdout is not XML: %v\n%s", err, out)
	}
	if len(report.Files) != 1 || len(report.Files[0].Errors) != 1 ||
		report.Files[0].Errors[0].Source != "hcl-linter.block_order" {
		t.Fatalf("unexpected checkstyle report: %+v", report)
	}
}

func TestOutputFormatValidation(t *testing.T) {
	if err := validateOutputFormat("yaml"); err == nil {
		t.Error("expected an error for an unknown format")
	}
	for _, f := range outputFormats {
		if err := validateOutputFormat(f); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}

func TestSARIFURIRelative(t *testing.T) {
	if got := sarifURI("live/prod/terragrunt.hcl"); got != "live/prod/terragrunt.hcl" {
		t.Errorf("relative paths must stay relative, got %q", got)
	}
}
