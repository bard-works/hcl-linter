package main

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/bard-works/hcl-linter/internal/rules"
	"github.com/bard-works/hcl-linter/internal/termcolor"
)

func (a *app) newExplainCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "explain [rule-name]",
		Short: "Describe a lint rule and its config options",
		Args:  cobra.MaximumNArgs(1),
		RunE:  a.runExplain,
	}
}

func (a *app) runExplain(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		a.printExplainTable()
		return nil
	}
	return a.printExplainDetail(args[0])
}

// printExplainTable prints a compact aligned table of all registered rules.
func (a *app) printExplainTable() {
	all := rules.DefaultRegistry().All()
	sort.Slice(all, func(i, j int) bool { return all[i].Name() < all[j].Name() })

	w := tabwriter.NewWriter(a.out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "RULE\tSEVERITY\tFIXABLE\tSUMMARY")
	for _, r := range all {
		doc := r.Doc()
		fixable := "no"
		if doc.Fixable {
			fixable = "yes"
		}
		sev := doc.Severity
		if termcolor.Enabled() {
			switch sev {
			case "error":
				sev = termcolor.Error(sev)
			case "warning":
				sev = termcolor.Warning(sev)
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", termcolor.Rule(r.Name()), sev, fixable, doc.Summary)
	}
	w.Flush()
}

// printExplainDetail prints the full documentation for one rule.
func (a *app) printExplainDetail(name string) error {
	r, ok := ruleByName(name)
	if !ok {
		return fmt.Errorf("unknown rule %q", name)
	}

	doc := r.Doc()

	// Header line: rule name + [severity, fixable?]
	tags := doc.Severity
	if doc.Fixable {
		tags += ", fixable"
	}
	fmt.Fprintf(a.out, "%s  [%s]\n", termcolor.Rule(r.Name()), tags)

	// Summary / description
	fmt.Fprintln(a.out)
	fmt.Fprintf(a.out, "  %s\n", doc.Summary)
	if doc.Description != "" {
		fmt.Fprintln(a.out)
		for _, line := range strings.Split(doc.Description, "\n") {
			fmt.Fprintf(a.out, "  %s\n", line)
		}
	}

	// Config fields
	if len(doc.ConfigFields) > 0 {
		fmt.Fprintln(a.out)
		fmt.Fprintf(a.out, "  Config (rules { %s { ... } }):\n", doc.ConfigBlock)
		w := tabwriter.NewWriter(a.out, 0, 0, 2, ' ', 0)
		for _, f := range doc.ConfigFields {
			req := "optional"
			if f.Required {
				req = "required"
			}
			def := ""
			if f.Default != "" {
				def = "  default: " + f.Default
			}
			fmt.Fprintf(w, "    %s\t%s\t%s\t%s%s\n", f.Name, f.Type, req, f.Doc, def)
		}
		w.Flush()
	}

	// Example
	if doc.Example.Violation != "" {
		fmt.Fprintln(a.out)
		fmt.Fprintln(a.out, "  Example violation:")
		for _, line := range strings.Split(doc.Example.Violation, "\n") {
			fmt.Fprintf(a.out, "    %s\n", line)
		}
	}
	if doc.Example.Fixed != "" {
		fmt.Fprintln(a.out)
		fmt.Fprintln(a.out, "  After fix:")
		for _, line := range strings.Split(doc.Example.Fixed, "\n") {
			fmt.Fprintf(a.out, "    %s\n", line)
		}
	}

	fmt.Fprintln(a.out)
	return nil
}

func ruleByName(name string) (rules.Rule, bool) {
	for _, r := range rules.DefaultRegistry().All() {
		if r.Name() == name {
			return r, true
		}
	}
	return nil, false
}
