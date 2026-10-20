package main

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/vi-dev/clix"
	"github.com/vi-dev/clix/console"
	"github.com/vi-dev/clix/console/consoletest"
)

func TestCheckRecordsOutcomes(t *testing.T) {
	ctx, n := consoletest.Context(t.Context())
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	err := checkPackages(cmd, []string{"tool", "broken"})
	if _, ok := errors.AsType[*clix.ExitError](err); !ok {
		t.Fatalf("err = %v, want an exit error", err)
	}
	if done, _, outcome := n.TaskFor("Checking tool").Snapshot(); !done || outcome != "Checked tool (1 step)" {
		t.Errorf("tool: done %v, outcome %q", done, outcome)
	}
	broken := n.TaskFor("Checking broken")
	if _, failed, outcome := broken.Snapshot(); !failed || outcome != "Failed to check broken" {
		t.Errorf("broken: failed %v, outcome %q", failed, outcome)
	}
	if got, want := broken.Phases(), []string{"testing"}; !slices.Equal(got, want) {
		t.Errorf("broken phases = %q, want %q", got, want)
	}
	if got, want := broken.Warns(), []string{`broken: test step 1 ("false"): exit status 1`}; !slices.Equal(got, want) {
		t.Errorf("broken warns = %q, want %q", got, want)
	}
	if got, want := n.Successes(), []string{"Checked 1 package, 1 failed"}; !slices.Equal(got, want) {
		t.Errorf("successes = %q, want %q", got, want)
	}
	if got, want := n.Hints(), []string{"Retry with `app check broken`"}; !slices.Equal(got, want) {
		t.Errorf("hints = %q, want %q", got, want)
	}
}

func TestInstallTranscript(t *testing.T) {
	got := consoletest.Transcript(t, console.Options{Color: console.ColorNever}, func(c *console.Console, tick func()) {
		if err := install(console.NewContext(t.Context(), c), "jq", "1.8.3"); err != nil {
			t.Fatal(err)
		}
	})
	if want := "OK Installed jq 1.8.3\n"; got != want {
		t.Errorf("transcript:\n got %q\nwant %q", got, want)
	}
}

func TestInstallEvents(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := newRoot()
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"install", "jq@1.8.3", "--narration", "json"})
	if code, err := clix.Execute(t.Context(), root, clix.Options{App: "app", Version: "0.1.0"}); code != 0 {
		t.Fatalf("code, err = %d, %v; want 0", code, err)
	}
	events, err := console.ReadEvents(&stderr)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, e := range events {
		if e.State != "progress" {
			kinds = append(kinds, strings.TrimSpace(e.Event+" "+e.State))
		}
	}
	want := []string{"begin", "task started", "task phase", "task phase", "task done", "exit"}
	if !slices.Equal(kinds, want) {
		t.Errorf("events = %q, want %q", kinds, want)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

func TestBadColorExitsTwo(t *testing.T) {
	var stderr bytes.Buffer
	root := newRoot()
	root.SetErr(&stderr)
	root.SetArgs([]string{"--color", "bogus", "status"})
	if code, _ := clix.Execute(t.Context(), root, clix.Options{App: "app", Version: "0.1.0"}); code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if got, want := stderr.String(), "Error: invalid --color value \"bogus\" (want auto, always, or never)\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}
