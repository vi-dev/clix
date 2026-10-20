package console

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"syscall"
	"testing"
	"time"
)

var jsonStart = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func newJSONTest(opts Options) (*Console, *bytes.Buffer, *bytes.Buffer, func(time.Duration)) {
	now, advance := fakeClock(jsonStart)
	opts.Now = now
	opts.ManualTick = true
	opts.Narration = NarrationJSON
	c, out, errb := newTest(opts)
	return c, out, errb, advance
}

func decode(t *testing.T, b *bytes.Buffer) []Event {
	t.Helper()
	events, err := ReadEvents(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	return events
}

func ofKind(events []Event, kind, state string) []Event {
	var got []Event
	for _, e := range events {
		if e.Event == kind && (state == "" || e.State == state) {
			got = append(got, e)
		}
	}
	return got
}

func TestJSONBeginIsFirstAndExitIsLast(t *testing.T) {
	c, _, errb, _ := newJSONTest(Options{})
	c.Begin("nem", "0.9.3")
	c.Infof("hello %s", "world")
	c.Exit(1, nil)
	events := decode(t, errb)
	if len(events) != 3 {
		t.Fatalf("events = %+v, want 3", events)
	}
	first := events[0]
	if first.Event != "begin" || first.App != "nem" || first.Version != "0.9.3" || first.Schema != 1 {
		t.Errorf("first = %+v, want begin nem 0.9.3 schema 1", first)
	}
	last := events[2]
	if last.Event != "exit" || last.Code != 1 || last.Signal != "" {
		t.Errorf("last = %+v, want exit code 1", last)
	}
	for i, e := range events {
		if e.Seq != i+1 {
			t.Errorf("events[%d].Seq = %d, want %d", i, e.Seq, i+1)
		}
	}
}

func TestJSONLineEventCarriesLevelTextAndHint(t *testing.T) {
	c, _, errb, _ := newJSONTest(Options{})
	c.Error(errors.New("parse nem.toml: bad"), "Run `nem status`")
	events := decode(t, errb)
	if len(events) != 1 {
		t.Fatalf("events = %+v, want 1", events)
	}
	e := events[0]
	if e.Event != "line" || e.Level != "error" || e.Text != "Parse nem.toml: bad" || e.Hint != "Run `nem status`" {
		t.Errorf("event = %+v", e)
	}
}

func TestJSONTaskLifecycle(t *testing.T) {
	c, _, errb, advance := newJSONTest(Options{})
	tk := c.Start("Installing jq 1.8.3")
	tk.Phase("downloading")
	advance(1240 * time.Millisecond)
	tk.Done("Installed jq 1.8.3")
	events := decode(t, errb)
	if len(events) != 3 {
		t.Fatalf("events = %+v, want 3", events)
	}
	if e := events[0]; e.Event != "task" || e.State != "started" || e.Task != 1 || e.Label != "Installing jq 1.8.3" {
		t.Errorf("started = %+v", e)
	}
	if e := events[1]; e.State != "phase" || e.Task != 1 || e.Phase != "downloading" {
		t.Errorf("phase = %+v", e)
	}
	if e := events[2]; e.State != "done" || e.Task != 1 || e.Outcome != "Installed jq 1.8.3" || e.ElapsedMS != 1240 {
		t.Errorf("done = %+v", e)
	}
}

func TestJSONFilteringMatchesPlain(t *testing.T) {
	c, _, errb, _ := newJSONTest(Options{Quiet: true})
	c.Info("info")
	c.Debug("debug")
	c.Warn("warn")
	tk := c.Start("work")
	tk.Fail("Failed work")
	events := decode(t, errb)
	if len(events) != 2 || events[0].Level != "warn" || events[1].State != "failed" {
		t.Errorf("quiet events = %+v, want warn and failed", events)
	}

	v, _, verrb, _ := newJSONTest(Options{Verbose: true})
	v.Debug("debug")
	if events := decode(t, verrb); len(events) != 1 || events[0].Level != "debug" {
		t.Errorf("verbose events = %+v, want debug", events)
	}

	qv, _, qverrb, _ := newJSONTest(Options{Quiet: true, Verbose: true})
	qv.Debug("debug")
	qv.Info("info")
	if events := decode(t, qverrb); len(events) != 1 || events[0].Level != "debug" {
		t.Errorf("quiet verbose events = %+v, want debug", events)
	}
}

func TestJSONAttributesTaskLinesAndOutput(t *testing.T) {
	c, out, errb, _ := newJSONTest(Options{})
	tk := c.Start("build")
	tk.Warnf("rpath %s", "libfoo")
	fmt.Fprint(tk.ErrOut(), "make: error\n")
	fmt.Fprint(tk.Out(), "compiled\n")
	events := decode(t, errb)
	lines := ofKind(events, "line", "")
	if len(lines) != 1 || lines[0].Task != 1 || lines[0].Level != "warn" {
		t.Errorf("lines = %+v, want one warn with task 1", lines)
	}
	outputs := ofKind(events, "output", "")
	if len(outputs) != 2 {
		t.Fatalf("outputs = %+v, want 2", outputs)
	}
	if e := outputs[0]; e.Task != 1 || e.Stream != "stderr" || e.Text != "make: error\n" || e.Partial {
		t.Errorf("stderr output = %+v", e)
	}
	if e := outputs[1]; e.Task != 1 || e.Stream != "stdout" || e.Text != "compiled\n" {
		t.Errorf("stdout output = %+v", e)
	}
	for line := range strings.SplitSeq(strings.TrimSuffix(errb.String(), "\n"), "\n") {
		if !strings.HasPrefix(line, "{") {
			t.Errorf("stderr has a raw line %q", line)
		}
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want nothing", out.String())
	}
}

func TestJSONNarratorOutIsDataPlane(t *testing.T) {
	c, out, errb, _ := newJSONTest(Options{})
	fmt.Fprint(c.Out(), "row\n")
	if got := out.String(); got != "row\n" {
		t.Errorf("stdout = %q, want %q", got, "row\n")
	}
	if events := decode(t, errb); len(events) != 0 {
		t.Errorf("events = %+v, want none for Out", events)
	}
	fmt.Fprint(c.ErrOut(), "x\n")
	events := decode(t, errb)
	if len(events) != 1 {
		t.Fatalf("events = %+v, want 1", events)
	}
	if e := events[0]; e.Event != "output" || e.Task != 0 || e.Stream != "stderr" || e.Text != "x\n" {
		t.Errorf("event = %+v", e)
	}
}

func TestJSONSamplesProgressEveryTenthTick(t *testing.T) {
	c, _, errb, _ := newJSONTest(Options{})
	tk := c.Start("download")
	tk.Phase("downloading")
	tk.Progress(1, 10, Items)
	progress := func() []Event { return ofKind(decode(t, errb), "task", "progress") }
	for range 9 {
		c.Tick()
	}
	if got := progress(); len(got) != 0 {
		t.Fatalf("after 9 ticks progress = %+v, want none", got)
	}
	c.Tick()
	got := progress()
	if len(got) != 1 {
		t.Fatalf("after 10 ticks progress = %+v, want 1", got)
	}
	if e := got[0]; e.Task != 1 || e.Phase != "downloading" || e.Done != 1 || e.Total != 10 || e.Unit != "items" {
		t.Errorf("progress = %+v", e)
	}
	for range 10 {
		c.Tick()
	}
	if got := progress(); len(got) != 1 {
		t.Errorf("unchanged progress emitted again: %+v", got)
	}
	tk.Progress(2, 10, Items)
	for range 10 {
		c.Tick()
	}
	if got := progress(); len(got) != 2 || got[1].Done != 2 {
		t.Errorf("progress = %+v, want a second event with done 2", got)
	}
}

func TestJSONNoProgressWithoutPhaseOrWithItemsTotalZero(t *testing.T) {
	c, _, errb, _ := newJSONTest(Options{})
	tk := c.Start("work")
	tk.Progress(5, 10, Items)
	for range 10 {
		c.Tick()
	}
	if got := ofKind(decode(t, errb), "task", "progress"); len(got) != 0 {
		t.Errorf("progress without phase = %+v, want none", got)
	}
	tk.Phase("counting")
	tk.Progress(5, 0, Items)
	for range 10 {
		c.Tick()
	}
	if got := ofKind(decode(t, errb), "task", "progress"); len(got) != 0 {
		t.Errorf("progress with items total 0 = %+v, want none", got)
	}
}

func TestJSONOutputSplitsOnCRAndLF(t *testing.T) {
	c, _, errb, _ := newJSONTest(Options{})
	tk := c.Start("fetch")
	w := tk.ErrOut()
	fmt.Fprint(w, "10%\r20%\r")
	fmt.Fprint(w, "done\n")
	outputs := ofKind(decode(t, errb), "output", "")
	want := []string{"10%\r", "20%\r", "done\n"}
	if len(outputs) != len(want) {
		t.Fatalf("outputs = %+v, want %d", outputs, len(want))
	}
	for i, e := range outputs {
		if e.Text != want[i] || e.Partial {
			t.Errorf("outputs[%d] = %+v, want text %q", i, e, want[i])
		}
	}
}

func TestJSONOutputCapsAt8KiB(t *testing.T) {
	c, _, errb, _ := newJSONTest(Options{})
	tk := c.Start("build")
	fmt.Fprint(tk.ErrOut(), strings.Repeat("a", 9000))
	outputs := ofKind(decode(t, errb), "output", "")
	if len(outputs) != 1 || len(outputs[0].Text) != 8192 || !outputs[0].Partial {
		t.Fatalf("outputs = %d events, want one of 8192 bytes marked partial", len(outputs))
	}
	tk.Done("Built")
	events := decode(t, errb)
	n := len(events)
	if n < 2 {
		t.Fatalf("events = %d, want the flush and the done event", n)
	}
	flushed, done := events[n-2], events[n-1]
	if flushed.Event != "output" || len(flushed.Text) != 808 || flushed.Partial || flushed.Task != 1 {
		t.Errorf("flushed = event %q, %d bytes, partial %v, task %d", flushed.Event, len(flushed.Text), flushed.Partial, flushed.Task)
	}
	if done.State != "done" {
		t.Errorf("last = %+v, want done", done)
	}
}

func TestJSONOutputReplacesInvalidUTF8(t *testing.T) {
	c, _, errb, _ := newJSONTest(Options{})
	tk := c.Start("build")
	_, _ = tk.ErrOut().Write([]byte{0xff, 'a', '\n'})
	outputs := ofKind(decode(t, errb), "output", "")
	if len(outputs) != 1 || outputs[0].Text != "�a\n" {
		t.Errorf("outputs = %+v, want text %q", outputs, "�a\n")
	}
}

func TestJSONWriteAfterTaskEndFlushesAtOnce(t *testing.T) {
	c, _, errb, _ := newJSONTest(Options{})
	tk := c.Start("build")
	tk.Done("built")
	fmt.Fprint(tk.ErrOut(), "tail")
	c.Info("later")
	before := ofKind(decode(t, errb), "output", "")
	if len(before) != 1 {
		t.Fatalf("outputs = %+v before Exit, want 1", before)
	}
	c.Exit(0, nil)
	events := decode(t, errb)
	outputs := ofKind(events, "output", "")
	if len(outputs) != 1 {
		t.Fatalf("outputs = %+v after Exit, want 1", outputs)
	}
	if e := outputs[0]; e.Task != 1 || e.Stream != "stderr" || e.Text != "tail" {
		t.Errorf("output = %+v, want task 1 stderr tail", e)
	}
	var outputAt, laterAt int
	for i, e := range events {
		switch {
		case e.Event == "output":
			outputAt = i
		case e.Event == "line" && e.Text == "later":
			laterAt = i
		}
	}
	if outputAt > laterAt {
		t.Errorf("output at %d after the later line at %d: %+v", outputAt, laterAt, events)
	}
}

func TestJSONExitFlushesNarratorStderrBuffer(t *testing.T) {
	c, _, errb, _ := newJSONTest(Options{})
	fmt.Fprint(c.ErrOut(), "tail")
	if events := decode(t, errb); len(events) != 0 {
		t.Fatalf("events = %+v before Exit, want none", events)
	}
	c.Exit(0, nil)
	events := decode(t, errb)
	if len(events) != 2 {
		t.Fatalf("events = %+v, want output then exit", events)
	}
	if e := events[0]; e.Event != "output" || e.Text != "tail" || e.Partial || e.Stream != "stderr" {
		t.Errorf("output = %+v", e)
	}
	if e := events[1]; e.Event != "exit" || e.Code != 0 {
		t.Errorf("exit = %+v", e)
	}
}

func TestJSONExitSignalName(t *testing.T) {
	for _, tc := range []struct {
		code int
		sig  syscall.Signal
		want string
	}{
		{130, syscall.SIGINT, "SIGINT"},
		{143, syscall.SIGTERM, "SIGTERM"},
	} {
		c, _, errb, _ := newJSONTest(Options{})
		c.Exit(tc.code, tc.sig)
		events := decode(t, errb)
		if len(events) != 1 || events[0].Code != tc.code || events[0].Signal != tc.want {
			t.Errorf("events = %+v, want exit %d %s", events, tc.code, tc.want)
		}
	}
}

func TestJSONTimeIsUTCMicroseconds(t *testing.T) {
	c, _, errb, _ := newJSONTest(Options{})
	c.Info("now")
	if got := errb.String(); !strings.Contains(got, `"time":"2026-01-01T12:00:00.000000Z"`) {
		t.Errorf("stderr = %q, want UTC microsecond time", got)
	}
}

func TestJSONEventIsOneWrite(t *testing.T) {
	var w countingWriter
	now, _ := fakeClock(jsonStart)
	c := New(&bytes.Buffer{}, &w, Options{Narration: NarrationJSON, ManualTick: true, Now: now})
	c.Info("one")
	if w.writes != 1 {
		t.Errorf("writes = %d, want 1", w.writes)
	}
	if !strings.HasSuffix(w.buf.String(), "}\n") {
		t.Errorf("stderr = %q, want one newline-terminated object", w.buf.String())
	}
}

func TestExitGivesUpWhenTheMutexIsHeld(t *testing.T) {
	c, _, errb, _ := newJSONTest(Options{})
	c.mu.Lock()
	returned := make(chan struct{})
	go func() {
		c.Exit(1, nil)
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		c.mu.Unlock()
		t.Fatal("Exit did not return while the mutex was held")
	}
	c.mu.Unlock()
	if events := ofKind(decode(t, errb), "exit", ""); len(events) != 0 {
		t.Errorf("exit events = %+v, want none", events)
	}
}

func TestTickerRunsInJSONMode(t *testing.T) {
	var out, errb syncBuffer
	c := New(&out, &errb, Options{Narration: NarrationJSON})
	base := settledGoroutines()
	tk := c.Start("a")
	if n := waitGoroutines(base + 1); n != base+1 {
		t.Fatalf("goroutines = %d with an open task, want %d", n, base+1)
	}
	tk.Done("done a")
	if n := waitGoroutines(base); n != base {
		t.Errorf("goroutines = %d after the task, want %d", n, base)
	}
}

func TestPlainAnnouncesNothingAfterExit(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorNever})
	c.Start("work")
	c.Exit(0, nil)
	fmt.Fprint(c.ErrOut(), "late\n")
	if got, want := errb.String(), "late\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func TestJSONNarratorLevelBuffersFlushInFirstWriteOrder(t *testing.T) {
	c, _, errb, _ := newJSONTest(Options{})
	tk := c.Start("build")
	fmt.Fprint(tk.ErrOut(), "task tail")
	fmt.Fprint(c.ErrOut(), "narrator tail")
	c.Exit(1, nil)
	outputs := ofKind(decode(t, errb), "output", "")
	if len(outputs) != 2 || outputs[0].Text != "task tail" || outputs[0].Task != 1 || outputs[1].Text != "narrator tail" {
		t.Errorf("outputs = %+v, want the task tail then the narrator tail", outputs)
	}
}

func TestJSONOutputTreatsCRLFAsOneEnding(t *testing.T) {
	c, _, errb, _ := newJSONTest(Options{})
	tk := c.Start("fetch")
	w := tk.ErrOut()
	fmt.Fprint(w, "a\r\nb\n")
	fmt.Fprint(w, "c\r")
	fmt.Fprint(w, "\nd\n")
	outputs := ofKind(decode(t, errb), "output", "")
	want := []string{"a\r\n", "b\n", "c\r", "\n", "d\n"}
	if len(outputs) != len(want) {
		t.Fatalf("outputs = %+v, want %d", outputs, len(want))
	}
	for i, e := range outputs {
		if e.Text != want[i] || e.Partial {
			t.Errorf("outputs[%d] = %+v, want text %q", i, e, want[i])
		}
	}
}
