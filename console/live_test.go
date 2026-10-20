package console

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func newLiveTest(opts Options) (*Console, *bytes.Buffer, *bytes.Buffer) {
	opts.IsTTY = true
	opts.ManualTick = true
	if opts.Now == nil {
		opts.Now, _ = fakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	}
	return newTest(opts)
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && (s[j] < 'A' || s[j] > 'Z') && (s[j] < 'a' || s[j] > 'z') {
				j++
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type countingWriter struct {
	writes int
	buf    bytes.Buffer
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.writes++
	return w.buf.Write(p)
}

func TestLiveSingleTaskShowsLabelAndSegment(t *testing.T) {
	c, _, errb := newLiveTest(Options{Color: ColorNever})
	tk := c.Start("Installing go v1.26.5")
	tk.Phase("downloading")
	c.Tick()

	got := errb.String()
	if !strings.Contains(got, "Installing go v1.26.5") || !strings.Contains(got, "downloading") {
		t.Errorf("live line missing label/phase: %q", got)
	}
	if !strings.Contains(got, "\x1b[2K") {
		t.Errorf("live line missing clear-line sequence: %q", got)
	}
	tk.Done("Installed go v1.26.5")
}

func TestLiveProgressAloneRendersNothingUntilStatusIsSet(t *testing.T) {
	c, _, errb := newLiveTest(Options{Color: ColorNever})
	tk := c.Start("Updating catalog")
	tk.Progress(3, 10, Items)
	c.Tick()

	if got := errb.String(); strings.Contains(got, "3/10") {
		t.Fatalf("Progress alone must not render: %q", got)
	}

	tk.Phase("copying")
	c.Tick()
	if got := errb.String(); !strings.Contains(got, "copying 3/10") {
		t.Fatalf("Phase set after Progress must render the count alongside it: %q", got)
	}
	tk.Done("Updated catalog")
}

func TestLiveSegmentChangeClearsStaleProgress(t *testing.T) {
	c, _, errb := newLiveTest(Options{Color: ColorNever})
	tk := c.Start("Filling go")
	tk.Phase("filling 1.0.0 linux/amd64")
	tk.Progress(10, 10, Bytes)
	c.Tick()
	if got := errb.String(); !strings.Contains(got, "filling 1.0.0 linux/amd64 100%") {
		t.Fatalf("progress under its phase must render: %q", got)
	}

	errb.Reset()
	tk.Phase("filling 1.0.0 linux/arm64")
	c.Tick()
	if got := errb.String(); strings.Contains(got, "100%") {
		t.Fatalf("a new phase must not inherit the previous phase's progress: %q", got)
	}

	errb.Reset()
	tk.Phase("filling 1.0.0 linux/arm64")
	tk.Progress(5, 10, Bytes)
	tk.Phase("filling 1.0.0 linux/arm64")
	c.Tick()
	if got := errb.String(); !strings.Contains(got, "filling 1.0.0 linux/arm64 50%") {
		t.Fatalf("setting the same phase again must keep its progress: %q", got)
	}
	tk.Done("Filled go")
}

func TestLiveDiscardShrinksBlockWithNoCompletionLine(t *testing.T) {
	c, _, errb := newLiveTest(Options{Color: ColorNever})
	first := c.Start("Mirroring curl")
	first.Phase("probing")
	second := c.Start("Mirroring go")
	second.Phase("copying 1.26.5")
	c.Tick()
	errb.Reset()

	first.Discard()

	got := errb.String()
	if strings.Contains(got, "Mirroring curl") {
		t.Errorf("discarded task's line still present: %q", got)
	}
	if strings.Contains(got, "OK ") || strings.Contains(got, "ERROR ") {
		t.Errorf("Discard rendered a completion line: %q", got)
	}
	if !strings.HasPrefix(got, "\x1b[2A\x1b[J") {
		t.Errorf("Discard did not clear the two-line block first: %q", got)
	}
	if !strings.Contains(got, "Mirroring go") {
		t.Errorf("surviving task not repainted after the discard: %q", got)
	}

	second.Done("Mirrored go (1 tag copied)")
}

func TestLiveTwoTasksInStartOrder(t *testing.T) {
	c, _, errb := newLiveTest(Options{Color: ColorNever})
	first := c.Start("Installing go v1.26.5")
	first.Phase("downloading")
	second := c.Start("Installing kubectl v1.34.1")
	second.Phase("extracting")
	c.Tick()

	got := errb.String()
	goIdx := strings.Index(got, "Installing go v1.26.5")
	kubectlIdx := strings.Index(got, "Installing kubectl v1.34.1")
	if goIdx == -1 || kubectlIdx == -1 {
		t.Fatalf("expected both task lines, got %q", got)
	}
	if goIdx > kubectlIdx {
		t.Errorf("tasks not painted in start order: %q", got)
	}

	first.Done("Installed go v1.26.5")
	second.Done("Installed kubectl v1.34.1")
}

func TestLiveDonePrintsCompletionAboveAndRemovesLine(t *testing.T) {
	c, _, errb := newLiveTest(Options{Color: ColorAlways})
	first := c.Start("Installing go v1.26.5")
	first.Phase("downloading")
	second := c.Start("Installing kubectl v1.34.1")
	second.Phase("extracting")
	c.Tick()
	errb.Reset()

	first.Done("Installed go v1.26.5")

	got := errb.String()
	const wantClear = "\x1b[2A\x1b[J"
	_, after, ok := strings.Cut(got, wantClear)
	if !ok {
		t.Fatalf("expected cursor-up-to-top + erase sequence %q, got %q", wantClear, got)
	}
	if strings.Contains(got, "\x1b[2K\n\x1b[2K\n") {
		t.Errorf("cursor left mid-block by a per-line clear loop instead of a single erase: %q", got)
	}
	doneIdx := strings.Index(after, "✓")
	if doneIdx == -1 {
		t.Fatalf("completion line missing right after the clear sequence: %q", after)
	}
	remainIdx := strings.Index(after, "Installing kubectl v1.34.1")
	if remainIdx == -1 {
		t.Fatalf("remaining task line missing after Done: %q", after)
	}
	if doneIdx > remainIdx {
		t.Errorf("completion did not print above the repainted block: %q", after)
	}
	if strings.Contains(after[:remainIdx], "Installing go v1.26.5") {
		t.Errorf("finished task's line was not removed from the block: %q", after)
	}

	second.Done("Installed kubectl v1.34.1")
}

func TestLiveDoneLineMatchesPlainSuccess(t *testing.T) {
	now, advance := fakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	c, _, errb := newLiveTest(Options{Color: ColorNever, Now: now})
	tk := c.Start("Installing go v1.26.5")
	errb.Reset()
	advance(3 * time.Second)
	tk.Done("Installed go v1.26.5")
	if got, want := errb.String(), "\x1b[1A\x1b[JOK Installed go v1.26.5 (3s)\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func TestLiveTruncatesAtWidth(t *testing.T) {
	c, _, errb := newLiveTest(Options{Color: ColorNever, Width: func() int { return 20 }})
	tk := c.Start("Installing go v1.26.5")
	tk.Phase("downloading a very long segment name")
	c.Tick()

	for line := range strings.SplitSeq(errb.String(), "\n") {
		if n := displayWidth(stripANSI(line)); n > 20 {
			t.Errorf("line exceeds width 20 (%d runes): %q", n, line)
		}
	}
	tk.Done("Installed go v1.26.5")
}

func TestLiveTruncatesLabelWithEllipsisKeepingTrailing(t *testing.T) {
	c, _, errb := newLiveTest(Options{Color: ColorNever, Width: func() int { return 40 }})
	tk := c.Start(strings.Repeat("a", 50))
	tk.Phase("downloading")
	tk.Progress(3, 10, Items)
	errb.Reset()
	c.Tick()

	line := strings.TrimSuffix(strings.TrimPrefix(errb.String(), "\x1b[1A\x1b[2K"), "\n")
	if n := displayWidth(line); n != 40 {
		t.Errorf("line is %d runes, want 40: %q", n, line)
	}
	if !strings.HasSuffix(line, "…  downloading 3/10") {
		t.Errorf("line = %q, want a label ending in an ellipsis and the trailing text kept whole", line)
	}
	tk.Done("done")
}

func TestLiveCutsTrailingWhenNothingElseFits(t *testing.T) {
	c, _, errb := newLiveTest(Options{Color: ColorNever, Width: func() int { return 12 }})
	tk := c.Start(strings.Repeat("a", 50))
	tk.Phase("downloading")
	tk.Progress(3, 10, Items)
	errb.Reset()
	c.Tick()

	if got, want := formatTaskLine("|", strings.Repeat("a", 50), "downloading 3/10", 12, false), "| …  downloa"; got != want {
		t.Errorf("formatTaskLine = %q, want %q", got, want)
	}
	line := strings.TrimSuffix(strings.TrimPrefix(errb.String(), "\x1b[1A\x1b[2K"), "\n")
	if want := "/ …  downloa"; line != want {
		t.Errorf("line = %q, want %q", line, want)
	}
	if n := displayWidth(line); n != 12 {
		t.Errorf("line is %d runes, want 12", n)
	}
	tk.Done("done")
}

func TestLiveNeverQueriesTheTerminal(t *testing.T) {
	c, _, errb := newLiveTest(Options{Color: ColorNever})
	tk := c.Start(strings.Repeat("b", 90))
	line := strings.TrimSuffix(strings.TrimPrefix(errb.String(), "\x1b[2K"), "\n")
	if n := displayWidth(line); n != 80 {
		t.Errorf("line is %d runes, want 80 with no Width set: %q", n, line)
	}
	if !strings.HasSuffix(line, "…") {
		t.Errorf("line = %q, want the label ellipsized", line)
	}
	tk.Done("done")
}

func TestLiveAutoElapsedAfterTenSeconds(t *testing.T) {
	now, advance := fakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	c, _, errb := newLiveTest(Options{Color: ColorNever, Now: now})

	tk := c.Start("Building erlang v27.2")
	tk.Phase("./make.bash")
	advance(11 * time.Second)
	c.Tick()

	if got := errb.String(); !strings.Contains(got, "(11s)") {
		t.Errorf("expected auto-elapsed suffix after 11s of silence: %q", got)
	}
	tk.Done("Built erlang v27.2")
}

func TestInfoDuringLiveBlockClearsPrintsAndRepaints(t *testing.T) {
	c, _, errb := newLiveTest(Options{Color: ColorNever})
	tk := c.Start("Installing go v1.26.5")
	tk.Phase("downloading")
	c.Tick()
	errb.Reset()

	c.Info("Resolving catalog dev")

	want := "\x1b[1A\x1b[JINFO Resolving catalog dev\n\x1b[2K- Installing go v1.26.5  downloading\n"
	if got := errb.String(); got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
	tk.Done("Installed go v1.26.5")
}

func TestLiveBlockAbsentWhenNotTTY(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorNever, ManualTick: true})
	tk := c.Start("Installing go v1.26.5")
	tk.Phase("downloading")
	c.Tick()

	if errb.Len() != 0 {
		t.Errorf("non-TTY console painted a live block: %q", errb.String())
	}
	tk.Done("Installed go v1.26.5")
}

func TestLiveBlockAbsentWhenQuiet(t *testing.T) {
	c, _, errb := newLiveTest(Options{Quiet: true, Color: ColorNever})
	tk := c.Start("Installing go v1.26.5")
	tk.Phase("downloading")
	c.Tick()

	if errb.Len() != 0 {
		t.Errorf("quiet console painted a live block: %q", errb.String())
	}
	tk.Done("Installed go v1.26.5")
}

func TestLiveTaskIsNotAnnounced(t *testing.T) {
	c, out, errb := newLiveTest(Options{Color: ColorNever})
	tk := c.Start("Building jq 1.8.3")
	fmt.Fprint(c.Out(), "payload\n")
	tk.Done("Built jq 1.8.3")
	if strings.Contains(errb.String(), "INFO Building jq 1.8.3") {
		t.Errorf("live task printed a plain start line: %q", errb.String())
	}
	if got := strings.Count(errb.String(), "Building jq 1.8.3"); got == 0 {
		t.Errorf("live block never rendered the label: %q", errb.String())
	}
	if out.String() != "payload\n" {
		t.Errorf("stdout = %q, want the payload", out.String())
	}
}

func TestLiveRepaintsAdvanceASharedSpinnerFrame(t *testing.T) {
	c, _, errb := newLiveTest(Options{Color: ColorNever})
	a := c.Start("Installing go v1.26.5")
	b := c.Start("Installing kubectl v1.34.1")
	c.Tick()
	c.Tick()
	var frames []string
	for line := range strings.SplitSeq(errb.String(), "\n") {
		if i := strings.Index(line, "\x1b[2K"); i >= 0 {
			frames = append(frames, string([]rune(stripANSI(line[i+4:]))[:1]))
		}
	}
	if len(frames) < 6 {
		t.Fatalf("expected at least three repaints of two lines, got frames %v in %q", frames, errb.String())
	}
	last := frames[len(frames)-2:]
	prev := frames[len(frames)-4 : len(frames)-2]
	if last[0] != last[1] || prev[0] != prev[1] {
		t.Errorf("lines of one repaint must share a frame: prev=%v last=%v", prev, last)
	}
	if last[0] == prev[0] {
		t.Errorf("consecutive repaints must advance the frame: prev=%v last=%v", prev, last)
	}
	a.Done("Installed go v1.26.5")
	b.Done("Installed kubectl v1.34.1")
}

func TestRunTaskSetsStatusBeforeFnSoCountRenders(t *testing.T) {
	c, _, errb := newLiveTest(Options{Color: ColorNever})
	o := Outcomes{Done: "Pulled catalog", Failed: "Failed to pull catalog"}

	err := RunTask(NewContext(context.Background(), c), "Pulling catalog", "copying", o, func(count ProgressFunc) error {
		count(3, 10, Items)
		c.Tick()
		got := errb.String()
		if !strings.Contains(got, "Pulling catalog") || !strings.Contains(got, "copying 3/10") {
			t.Fatalf("live line missing label/phase/count: %q", got)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunTask: %v", err)
	}
	if got := errb.String(); !strings.Contains(got, "Pulled catalog") {
		t.Errorf("missing Done completion line: %q", got)
	}
}

func TestGuardedWriteClearsAndDoesNotRepaintUntilTheNextTick(t *testing.T) {
	c, _, errb := newLiveTest(Options{Color: ColorNever})
	tk := c.Start("Building erlang v27.2")
	tk.Phase("./make.bash")
	errb.Reset()

	fmt.Fprint(c.ErrOut(), "configure: checking for cc\n")

	const cleared = "\x1b[1A\x1b[Jconfigure: checking for cc\n"
	if got := errb.String(); got != cleared {
		t.Fatalf("after the guarded write stderr = %q, want %q", got, cleared)
	}

	c.Tick()
	if got, want := errb.String(), cleared+"\x1b[2K/ Building erlang v27.2  ./make.bash\n"; got != want {
		t.Errorf("after the tick stderr = %q, want %q", got, want)
	}
	tk.Done("Built erlang v27.2")
}

func TestGuardedWritersStayIntactWhileALiveTaskIsActive(t *testing.T) {
	var out, errb syncBuffer
	c := New(&out, &errb, Options{IsTTY: true, Color: ColorNever})

	tk := c.Start("Building erlang v27.2")
	tk.Phase("./make.bash")

	const streams, lines = 4, 25
	var wg sync.WaitGroup
	for s := range streams {
		wg.Go(func() {
			for i := range lines {
				fmt.Fprintf(c.ErrOut(), "err-%d-%d\n", s, i)
				fmt.Fprintf(c.Out(), "out-%d-%d\n", s, i)
				time.Sleep(time.Millisecond)
			}
		})
	}
	wg.Wait()
	tk.Done("Built erlang v27.2")

	gotErr := errb.String()
	for s := range streams {
		for i := range lines {
			payload := fmt.Sprintf("err-%d-%d\n", s, i)
			if n := strings.Count(gotErr, payload); n != 1 {
				t.Fatalf("payload %q written %d times, want 1", payload, n)
			}
			before := gotErr[:strings.Index(gotErr, payload)]
			if strings.HasSuffix(before, "\x1b[J") {
				continue
			}
			prev := before[strings.LastIndex(strings.TrimSuffix(before, "\n"), "\n")+1:]
			if !strings.HasSuffix(before, "\n") || !strings.Contains(prev, "err-") || strings.Contains(prev, "\x1b[2K") {
				t.Fatalf("payload %q not preceded by a clear or another payload line: %q", payload, before)
			}
		}
	}

	gotOut := out.String()
	if strings.Contains(gotOut, "\x1b") {
		t.Errorf("live-block escapes leaked into stdout: %q", gotOut)
	}
	for s := range streams {
		for i := range lines {
			want := fmt.Sprintf("out-%d-%d\n", s, i)
			if n := strings.Count(gotOut, want); n != 1 {
				t.Fatalf("stdout payload %q written %d times, want 1", want, n)
			}
		}
	}
}

func TestRepaintIsOneWrite(t *testing.T) {
	var out bytes.Buffer
	var errw countingWriter
	c := New(&out, &errw, Options{IsTTY: true, ManualTick: true, Color: ColorNever})
	tasks := []Task{c.Start("a"), c.Start("b"), c.Start("c")}
	for _, tk := range tasks {
		tk.Phase("working")
	}

	errw.writes = 0
	c.Tick()
	if errw.writes != 1 {
		t.Errorf("tick made %d writes, want 1", errw.writes)
	}

	errw.writes = 0
	c.Info("between")
	if errw.writes != 1 {
		t.Errorf("Info during the block made %d writes, want 1", errw.writes)
	}

	errw.writes = 0
	c.BlankErr()
	if errw.writes != 1 {
		t.Errorf("BlankErr during the block made %d writes, want 1", errw.writes)
	}
	if !strings.Contains(errw.buf.String(), "\x1b[3A\x1b[J\n\x1b[2K") {
		t.Errorf("BlankErr did not clear, print and repaint: %q", errw.buf.String())
	}

	errw.writes = 0
	fmt.Fprint(c.ErrOut(), "noise\n")
	if errw.writes != 1 {
		t.Errorf("guarded write during the block made %d writes, want 1", errw.writes)
	}

	errw.writes = 0
	tasks[0].Done("done a")
	if errw.writes != 1 {
		t.Errorf("Done during the block made %d writes, want 1", errw.writes)
	}
	tasks[1].Done("done b")
	tasks[2].Done("done c")
}

func TestDataWriteClearsBlockOnlyWhenStdoutIsTheTerminal(t *testing.T) {
	c, out, errb := newLiveTest(Options{Color: ColorNever, OutIsTTY: true})
	tk := c.Start("Listing")
	errb.Reset()

	c.Data("row\n")
	c.Data("row\n")
	if got, want := errb.String(), "\x1b[1A\x1b[J"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
	if got, want := out.String(), "row\nrow\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	c.Tick()
	if got, want := errb.String(), "\x1b[1A\x1b[J\x1b[2K/ Listing\n"; got != want {
		t.Errorf("after tick stderr = %q, want %q", got, want)
	}
	tk.Done("Listed")

	c2, out2, errb2 := newLiveTest(Options{Color: ColorNever})
	tk2 := c2.Start("Listing")
	errb2.Reset()
	c2.Data("row\n")
	if errb2.Len() != 0 {
		t.Errorf("stdout not a terminal: stderr = %q, want unchanged", errb2.String())
	}
	if out2.String() != "row\n" {
		t.Errorf("stdout = %q, want the row", out2.String())
	}
	tk2.Done("Listed")
}

func waitGoroutines(want int) int {
	deadline := time.Now().Add(time.Second)
	n := runtime.NumGoroutine()
	for n != want && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		n = runtime.NumGoroutine()
	}
	return n
}

func settledGoroutines() int {
	deadline := time.Now().Add(time.Second)
	n := runtime.NumGoroutine()
	stable := 0
	for stable < 5 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
		m := runtime.NumGoroutine()
		if m == n {
			stable++
		} else {
			n, stable = m, 0
		}
	}
	return n
}

func TestTickerStartsWithFirstVisibleTaskAndStopsWithLast(t *testing.T) {
	var out, errb syncBuffer
	c := New(&out, &errb, Options{IsTTY: true, Color: ColorNever})
	base := settledGoroutines()

	a := c.Start("a")
	if n := waitGoroutines(base + 1); n != base+1 {
		t.Fatalf("goroutines = %d with one open task, want %d", n, base+1)
	}
	b := c.Start("b")
	if n := runtime.NumGoroutine(); n != base+1 {
		t.Errorf("goroutines = %d with two open tasks, want %d", n, base+1)
	}
	a.Done("done a")
	if n := runtime.NumGoroutine(); n != base+1 {
		t.Errorf("goroutines = %d with one task left, want %d", n, base+1)
	}
	b.Done("done b")
	if n := waitGoroutines(base); n != base {
		t.Errorf("goroutines = %d after the last task, want %d", n, base)
	}

	c.Start("c")
	if n := waitGoroutines(base + 1); n != base+1 {
		t.Fatalf("goroutines = %d after a new task, want %d", n, base+1)
	}
	c.Exit(0, nil)
	if n := waitGoroutines(base); n != base {
		t.Errorf("goroutines = %d after Exit, want %d", n, base)
	}
}

func TestTickerNeverStartsWithManualTickOrPlain(t *testing.T) {
	base := settledGoroutines()
	for _, opts := range []Options{
		{IsTTY: true, ManualTick: true},
		{IsTTY: false},
		{IsTTY: true, Narration: NarrationPlain},
		{IsTTY: true, Quiet: true},
	} {
		c := New(&bytes.Buffer{}, &bytes.Buffer{}, opts)
		tk := c.Start("a")
		if n := runtime.NumGoroutine(); n != base {
			t.Errorf("%+v: goroutines = %d, want %d", opts, n, base)
		}
		tk.Done("done")
	}
}

func TestStaleTickEmitsNothing(t *testing.T) {
	var out, errb syncBuffer
	c := New(&out, &errb, Options{IsTTY: true, Color: ColorNever})
	tk := c.Start("a")
	c.mu.Lock()
	gen := c.tk.gen
	c.mu.Unlock()
	tk.Done("done a")
	before := errb.String()
	c.tick(gen)
	if got := errb.String(); got != before {
		t.Errorf("stale tick wrote %q", strings.TrimPrefix(got, before))
	}

	m, _, merr := newLiveTest(Options{Color: ColorNever})
	open := m.Start("b")
	m.mu.Lock()
	done := make(chan struct{})
	close(done)
	m.tk = &ticker{gen: 7, stop: make(chan struct{}), done: done}
	m.mu.Unlock()
	merr.Reset()
	m.tick(6)
	if merr.Len() != 0 {
		t.Errorf("stale tick with an open task wrote %q", merr.String())
	}
	m.tick(7)
	if !strings.Contains(merr.String(), "\x1b[2K") {
		t.Errorf("current tick did not repaint: %q", merr.String())
	}
	open.Done("done b")
}

func TestExitClearsTheBlock(t *testing.T) {
	c, _, errb := newLiveTest(Options{Color: ColorNever})
	c.Start("Building erlang v27.2")
	c.Tick()
	c.Exit(130, syscall.SIGINT)
	if got := errb.String(); !strings.HasSuffix(got, "\x1b[1A\x1b[J") {
		t.Errorf("stderr = %q, want it to end with the block cleared", got)
	}
	before := errb.String()
	c.Tick()
	c.Info("after")
	if got := errb.String(); got != before {
		t.Errorf("wrote after Exit: %q", strings.TrimPrefix(got, before))
	}
}

func TestNarrationPlainOnTTYHasNoCursorMovement(t *testing.T) {
	for _, color := range []ColorMode{ColorNever, ColorAlways} {
		c, _, errb := newTest(Options{IsTTY: true, Narration: NarrationPlain, Color: color})
		tk := c.Start("Installing go v1.26.5")
		tk.Phase("downloading")
		c.Tick()
		tk.Done("Installed go v1.26.5")
		got := errb.String()
		if color == ColorAlways {
			for _, code := range []string{ansiGreen, ansiYellow, ansiRed, ansiCyan, ansiDim, ansiReset} {
				got = strings.ReplaceAll(got, code, "")
			}
		}
		if strings.Contains(got, "\x1b[") {
			t.Errorf("color %v: plain narration on a terminal moved the cursor: %q", color, errb.String())
		}
	}
}

func TestFormatTaskLinePlainSpinner(t *testing.T) {
	got := formatTaskLine("|", "Installing go v1.26.5", "downloading 42%", 80, false)
	if want := "| Installing go v1.26.5  downloading 42%"; got != want {
		t.Errorf("plain = %q, want %q", got, want)
	}
	if got, want := formatTaskLine("/", "Reclaiming", "", 80, false), "/ Reclaiming"; got != want {
		t.Errorf("plain without phase = %q, want %q", got, want)
	}
}

func TestFormatTaskLineColoredSpinnerAndDimTrailing(t *testing.T) {
	got := formatTaskLine("⠋", "Installing go v1.26.5", "downloading", 80, true)
	want := "\x1b[36m⠋\x1b[0m Installing go v1.26.5  \x1b[2mdownloading\x1b[0m"
	if got != want {
		t.Errorf("colored = %q, want %q", got, want)
	}
	if got, want := formatTaskLine("⠋", "Reclaiming", "", 80, true), "\x1b[36m⠋\x1b[0m Reclaiming"; got != want {
		t.Errorf("colored without phase = %q, want %q", got, want)
	}
}

func TestFormatTaskLineColoredAfterTruncation(t *testing.T) {
	got := formatTaskLine("⠋", strings.Repeat("a", 30), "downloading", 20, true)
	want := "\x1b[36m⠋\x1b[0m aaaa…  \x1b[2mdownloading\x1b[0m"
	if got != want {
		t.Errorf("colored ellipsized = %q, want %q", got, want)
	}
	got = formatTaskLine("⠋", strings.Repeat("a", 30), "downloading", 8, true)
	want = "\x1b[36m⠋\x1b[0m …  \x1b[2mdow\x1b[0m"
	if got != want {
		t.Errorf("colored cut = %q, want %q", got, want)
	}
}

func TestSpinnerFrameSetsFollowColorMode(t *testing.T) {
	if got := spinnerFrame(0, true); got != "⠋" {
		t.Errorf("colored frame 0 = %q, want braille", got)
	}
	if got := spinnerFrame(0, false); got != "|" {
		t.Errorf("plain frame 0 = %q, want ASCII", got)
	}
	if spinnerFrame(len(spinnerUnicode), true) != spinnerFrame(0, true) || spinnerFrame(len(spinnerASCII), false) != spinnerFrame(0, false) {
		t.Error("frames must wrap around")
	}
}
