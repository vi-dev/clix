package console_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/vi-dev/clix/console"
	"github.com/vi-dev/clix/console/consoletest"
)

func TestTranscriptLiveFrames(t *testing.T) {
	got := consoletest.Transcript(t, console.Options{IsTTY: true, Color: console.ColorNever}, func(c *console.Console, tick func()) {
		task := c.Start("Installing jq 1.8.3")
		task.Phase("downloading")
		tick()
		tick()
		task.Done("Installed jq 1.8.3")
	})
	want := "\x1b[2K| Installing jq 1.8.3\n" +
		"\x1b[1A\x1b[2K/ Installing jq 1.8.3  downloading\n" +
		"\x1b[1A\x1b[2K- Installing jq 1.8.3  downloading\n" +
		"\x1b[1A\x1b[JOK Installed jq 1.8.3\n"
	if got != want {
		t.Errorf("transcript:\n got %q\nwant %q", got, want)
	}
}

func TestTranscriptPlain(t *testing.T) {
	got := consoletest.Transcript(t, console.Options{Color: console.ColorNever}, func(c *console.Console, tick func()) {
		task := c.Start("Building zlib")
		fmt.Fprint(task.ErrOut(), "cc: warning\n")
		task.Done("Built zlib")
	})
	if want := "INFO Building zlib\ncc: warning\nOK Built zlib\n"; got != want {
		t.Errorf("transcript:\n got %q\nwant %q", got, want)
	}
}

func TestTranscriptJSONStream(t *testing.T) {
	got := consoletest.Transcript(t, console.Options{Narration: console.NarrationJSON}, func(c *console.Console, tick func()) {
		c.Begin("nem", "1.0.0")
		task := c.Start("Installing jq")
		task.Phase("downloading")
		task.Progress(1, 4, console.Items)
		for range 10 {
			tick()
		}
		task.Done("Installed jq")
		c.Exit(0, nil)
	})
	events, err := console.ReadEvents(bytes.NewReader([]byte(got)))
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	var kinds []string
	for _, e := range events {
		kind := e.Event
		if e.State != "" {
			kind += " " + e.State
		}
		kinds = append(kinds, kind)
	}
	want := []string{"begin", "task started", "task phase", "task progress", "task done", "exit"}
	if strings.Join(kinds, ",") != strings.Join(want, ",") {
		t.Fatalf("events = %q, want %q", kinds, want)
	}
	if p := events[3]; p.Done != 1 || p.Total != 4 {
		t.Errorf("progress = %d/%d, want 1/4", p.Done, p.Total)
	}
	if d := events[4]; d.ElapsedMS != 1000 {
		t.Errorf("done elapsed_ms = %d, want 1000", d.ElapsedMS)
	}
}
