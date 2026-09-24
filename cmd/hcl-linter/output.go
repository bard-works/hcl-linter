package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bard-works/hcl-linter/internal/diag"
	"github.com/bard-works/hcl-linter/internal/rules"
)

// Output formats accepted by --output on lint and check.
const (
	outputText       = "text"
	outputJSON       = "json"
	outputSARIF      = "sarif"
	outputCheckstyle = "checkstyle"
)

var outputFormats = []string{outputText, outputJSON, outputSARIF, outputCheckstyle}

func validateOutputFormat(f string) error {
	for _, ok := range outputFormats {
		if f == ok {
			return nil
		}
	}
	return fmt.Errorf("invalid --output %q: must be one of %s", f, strings.Join(outputFormats, ", "))
}

// reportLintResults writes results in the selected format and returns whether
// any error-severity issue was found and how many files could not be linted.
func (a *app) reportLintResults(results []*diag.Result) (hasErrors bool, execFailures int, err error) {
	for _, r := range results {
		for _, issue := range r.Issues {
			if issue.Severity == diag.SeverityError {
				hasErrors = true
			}
			if issue.Rule == diag.RuleLinterError {
				execFailures++
			}
		}
	}
	switch a.outputFormat {
	case outputJSON:
		err = writeJSON(a, results)
	case outputSARIF:
		err = writeSARIF(a, results)
	case outputCheckstyle:
		err = writeCheckstyle(a, results)
	default:
		a.printLintResults(results)
	}
	return hasErrors, execFailures, err
}

// issuePath returns the file an issue points at, relative and slash-separated
// so reports are stable across machines and operating systems.
func issuePath(file string, issue diag.Issue) string {
	p := file
	if issue.Location.Filename != "" {
		p = issue.Location.Filename
	}
	return filepath.ToSlash(relPath(p))
}

type jsonIssue struct {
	File      string `json:"file"`
	Line      int    `json:"line,omitempty"`
	Column    int    `json:"column,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	EndColumn int    `json:"end_column,omitempty"`
	Severity  string `json:"severity"`
	Rule      string `json:"rule"`
	Message   string `json:"message"`
}

type jsonReport struct {
	Version int         `json:"version"`
	Issues  []jsonIssue `json:"issues"`
	Summary struct {
		Files    int `json:"files"`
		Issues   int `json:"issues"`
		Errors   int `json:"errors"`
		Warnings int `json:"warnings"`
	} `json:"summary"`
}

func writeJSON(a *app, results []*diag.Result) error {
	report := jsonReport{Version: 1, Issues: []jsonIssue{}}
	report.Summary.Files = len(results)
	for _, r := range results {
		for _, is := range r.Issues {
			report.Issues = append(report.Issues, jsonIssue{
				File:      issuePath(r.File, is),
				Line:      is.Location.Start.Line,
				Column:    is.Location.Start.Column,
				EndLine:   is.Location.End.Line,
				EndColumn: is.Location.End.Column,
				Severity:  string(is.Severity),
				Rule:      is.Rule,
				Message:   is.Message,
			})
			report.Summary.Issues++
			if is.Severity == diag.SeverityError {
				report.Summary.Errors++
			} else {
				report.Summary.Warnings++
			}
		}
	}
	enc := json.NewEncoder(a.out)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// SARIF 2.1.0, the subset GitHub code scanning reads.
type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string       `json:"id"`
	ShortDescription sarifMessage `json:"shortDescription"`
	HelpURI          string       `json:"helpUri"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysical `json:"physicalLocation"`
}

type sarifPhysical struct {
	ArtifactLocation sarifArtifact `json:"artifactLocation"`
	Region           *sarifRegion  `json:"region,omitempty"`
}

type sarifArtifact struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn,omitempty"`
	EndLine     int `json:"endLine,omitempty"`
	EndColumn   int `json:"endColumn,omitempty"`
}

const rulesDocURL = "https://github.com/bard-works/hcl-linter/blob/main/docs/rules.md"

func sarifLevel(s diag.Severity) string {
	switch s {
	case diag.SeverityError:
		return "error"
	case diag.SeverityWarning:
		return "warning"
	default:
		return "note"
	}
}

func writeSARIF(a *app, results []*diag.Result) error {
	summaries := map[string]string{}
	for _, r := range rules.DefaultRegistry().All() {
		summaries[r.Name()] = r.Doc().Summary
	}

	seenRules := map[string]bool{}
	var ruleIDs []string
	out := []sarifResult{}
	for _, r := range results {
		for _, is := range r.Issues {
			if !seenRules[is.Rule] {
				seenRules[is.Rule] = true
				ruleIDs = append(ruleIDs, is.Rule)
			}
			phys := sarifPhysical{ArtifactLocation: sarifArtifact{URI: issuePath(r.File, is)}}
			if is.Location.Start.Line > 0 {
				phys.Region = &sarifRegion{
					StartLine:   is.Location.Start.Line,
					StartColumn: is.Location.Start.Column,
					EndLine:     is.Location.End.Line,
					EndColumn:   is.Location.End.Column,
				}
			}
			out = append(out, sarifResult{
				RuleID:    is.Rule,
				Level:     sarifLevel(is.Severity),
				Message:   sarifMessage{Text: is.Message},
				Locations: []sarifLocation{{PhysicalLocation: phys}},
			})
		}
	}
	sort.Strings(ruleIDs)
	sarifRules := []sarifRule{}
	for _, id := range ruleIDs {
		desc := summaries[id]
		if desc == "" {
			desc = id
		}
		sarifRules = append(sarifRules, sarifRule{ID: id, ShortDescription: sarifMessage{Text: desc}, HelpURI: rulesDocURL})
	}

	log := sarifLog{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool: sarifTool{Driver: sarifDriver{
				Name:           "hcl-linter",
				Version:        Version,
				InformationURI: "https://github.com/bard-works/hcl-linter",
				Rules:          sarifRules,
			}},
			Results: out,
		}},
	}
	enc := json.NewEncoder(a.out)
	enc.SetIndent("", "  ")
	return enc.Encode(log)
}

type checkstyleReport struct {
	XMLName xml.Name         `xml:"checkstyle"`
	Version string           `xml:"version,attr"`
	Files   []checkstyleFile `xml:"file"`
}

type checkstyleFile struct {
	Name   string            `xml:"name,attr"`
	Errors []checkstyleError `xml:"error"`
}

type checkstyleError struct {
	Line     int    `xml:"line,attr"`
	Column   int    `xml:"column,attr,omitempty"`
	Severity string `xml:"severity,attr"`
	Message  string `xml:"message,attr"`
	Source   string `xml:"source,attr"`
}

func writeCheckstyle(a *app, results []*diag.Result) error {
	byFile := map[string]*checkstyleFile{}
	var order []string
	for _, r := range results {
		for _, is := range r.Issues {
			name := issuePath(r.File, is)
			f, ok := byFile[name]
			if !ok {
				f = &checkstyleFile{Name: name}
				byFile[name] = f
				order = append(order, name)
			}
			sev := "warning"
			if is.Severity == diag.SeverityError {
				sev = "error"
			}
			f.Errors = append(f.Errors, checkstyleError{
				Line:     is.Location.Start.Line,
				Column:   is.Location.Start.Column,
				Severity: sev,
				Message:  is.Message,
				Source:   "hcl-linter." + is.Rule,
			})
		}
	}
	report := checkstyleReport{Version: "4.3"}
	for _, name := range order {
		report.Files = append(report.Files, *byFile[name])
	}
	if _, err := fmt.Fprint(a.out, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(a.out)
	enc.Indent("", "  ")
	if err := enc.Encode(report); err != nil {
		return err
	}
	_, err := fmt.Fprintln(a.out)
	return err
}
