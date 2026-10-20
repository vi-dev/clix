package main

import (
	"testing"

	"github.com/vi-dev/clix/console"
	"github.com/vi-dev/clix/console/consoletest"
)

func TestGreetTranscript(t *testing.T) {
	got := consoletest.Transcript(t, console.Options{Color: console.ColorNever}, func(c *console.Console, tick func()) {
		con = c
		root := newRoot()
		root.SetArgs([]string{"greet", "world"})
		if err := root.ExecuteContext(t.Context()); err != nil {
			t.Fatal(err)
		}
	})
	if want := "INFO Hello, world\nOK Greeted world\n"; got != want {
		t.Errorf("transcript:\n got %q\nwant %q", got, want)
	}
}
