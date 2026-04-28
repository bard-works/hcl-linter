package main

import (
	"bytes"
	"testing"
)

func TestNewRootCmd_ListsSubcommands(t *testing.T) {
	resetFlags(t)

	cmd := newRootCmd()
	if cmd == nil {
		t.Fatal("newRootCmd returned nil")
	}

	want := []string{"lint", "check", "fix", "validate-config", "version"}
	got := map[string]bool{}
	for _, sub := range cmd.Commands() {
		got[sub.Name()] = true
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("root cmd missing subcommand %q", name)
		}
	}
}

func TestNewRootCmd_VersionSubcommand(t *testing.T) {
	resetFlags(t)

	cmd := newRootCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"version"})

	if err := cmd.Execute(); err != nil {
		t.Errorf("version subcommand returned error: %v", err)
	}
}

func TestNewRootCmd_HelpDefault(t *testing.T) {
	resetFlags(t)

	cmd := newRootCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Errorf("root cmd with no args returned error: %v", err)
	}
}
