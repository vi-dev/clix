package console

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func fakeClock(t0 time.Time) (now func() time.Time, advance func(time.Duration)) {
	var mu sync.Mutex
	cur := t0
	now = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return cur
	}
	advance = func(d time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		cur = cur.Add(d)
	}
	return now, advance
}

func TestTaskDoneAtOrOverOneSecondShowsSuffix(t *testing.T) {
	now, advance := fakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	c, _, errb := newTest(Options{Color: ColorNever, Now: now})

	task := c.Start("Installing go v1.26.5")
	advance(3 * time.Second)
	task.Done("Installed go v1.26.5")

	if got, want := errb.String(), "OK Installed go v1.26.5 (3s)\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func TestTaskDoneUnderOneSecondHasNoSuffix(t *testing.T) {
	now, advance := fakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	c, _, errb := newTest(Options{Color: ColorAlways, Now: now})

	task := c.Start("Installing go v1.26.5")
	advance(999 * time.Millisecond)
	task.Done("Installed go v1.26.5")

	if got, want := errb.String(), "\x1b[32m✓\x1b[0m Installed go v1.26.5\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func TestTaskFailEmitsUnderQuiet(t *testing.T) {
	c, _, errb := newTest(Options{Quiet: true, Color: ColorNever})
	task := c.Start("Installing go v1.26.5")
	task.Fail("Failed to install go v1.26.5")

	if got, want := errb.String(), "ERROR Failed to install go v1.26.5\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func TestTaskFailColoredShowsGlyph(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorAlways})
	task := c.Start("Installing go v1.26.5")
	task.Fail("Failed to install go v1.26.5")

	if got, want := errb.String(), "\x1b[31m✗ Failed to install go v1.26.5\x1b[0m\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func TestTaskDoneSuppressedByQuiet(t *testing.T) {
	c, _, errb := newTest(Options{Quiet: true, Color: ColorNever})
	task := c.Start("Installing go v1.26.5")
	task.Done("Installed go v1.26.5")

	if errb.Len() != 0 {
		t.Errorf("quiet did not suppress task success: %q", errb.String())
	}
}

func TestTaskDoneAfterFailIsNoOp(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorNever})
	task := c.Start("Installing go v1.26.5")
	task.Fail("Failed to install go v1.26.5")
	errb.Reset()

	task.Done("Installed go v1.26.5")
	task.Fail("Failed again")
	task.Discard()

	if errb.Len() != 0 {
		t.Errorf("terminal calls after Fail must be no-ops, got %q", errb.String())
	}
}

func TestTaskDiscardEmitsNothing(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorNever})
	task := c.Start("Mirroring curl")
	task.Phase("probing")

	task.Discard()

	if errb.Len() != 0 {
		t.Errorf("Discard produced output: %q", errb.String())
	}
}

func TestTaskConcurrentUpdatesRaceClean(t *testing.T) {
	c, _, _ := newTest(Options{Color: ColorNever})
	task := c.Start("Installing go v1.26.5")

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Go(func() {
			task.Phase("downloading")
			task.Progress(int64(i), 100, Bytes)
			task.Progress(int64(i), 10, Items)
			task.Infof("step %d", i)
			fmt.Fprint(task.ErrOut(), "noise\n")
		})
	}
	wg.Go(func() { task.Done("Installed go v1.26.5") })
	wg.Go(func() { task.Fail("Failed to install go v1.26.5") })
	wg.Go(func() { c.Start("Other").Discard() })
	wg.Wait()
}

func TestRunTaskFailReturnsErrUnchanged(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorNever})
	o := Outcomes{Done: "Pushed catalog", Failed: "Failed to push catalog"}
	sentinel := errors.New("boom")

	err := RunTask(NewContext(context.Background(), c), "Pushing catalog", "copying", o, func(ProgressFunc) error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Fatalf("RunTask error = %v, want sentinel unchanged", err)
	}

	got := errb.String()
	if !strings.Contains(got, "Failed to push catalog") {
		t.Errorf("missing Fail line: %q", got)
	}
	if strings.Contains(got, "Pushed catalog") {
		t.Errorf("unexpected Done line on failure: %q", got)
	}
}

func TestPendingTaskAnnouncedOnceBeforeFirstGuardedWrite(t *testing.T) {
	c, out, errb := newTest(Options{Color: ColorNever})
	task := c.Start("Building jq 1.8.3")
	w := c.Out()
	fmt.Fprint(w, "line1\n")
	fmt.Fprint(w, "line2\n")
	task.Done("Built jq 1.8.3")
	if got, want := errb.String(), "INFO Building jq 1.8.3\nOK Built jq 1.8.3\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
	if got, want := out.String(), "line1\nline2\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestPendingTasksAnnouncedInStartOrder(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorNever})
	a := c.Start("Building a 1")
	b := c.Start("Building b 2")
	fmt.Fprint(c.ErrOut(), "compiler noise\n")
	a.Done("Built a 1")
	b.Done("Built b 2")
	if got, want := errb.String(), "INFO Building a 1\nINFO Building b 2\ncompiler noise\nOK Built a 1\nOK Built b 2\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func TestPendingTaskWithoutOutputIsNeverAnnounced(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorNever})
	c.Start("Checking jq").Discard()
	c.Start("Installing jq 1.8.3").Done("Installed jq 1.8.3")
	fmt.Fprint(c.Out(), "later\n")
	if got, want := errb.String(), "OK Installed jq 1.8.3\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func TestPendingAnnouncementSuppressedByQuiet(t *testing.T) {
	c, out, errb := newTest(Options{Quiet: true, Color: ColorNever})
	task := c.Start("Building jq 1.8.3")
	fmt.Fprint(c.Out(), "payload\n")
	task.Done("Built jq 1.8.3")
	if errb.String() != "" {
		t.Errorf("quiet stderr = %q, want empty", errb.String())
	}
	if out.String() != "payload\n" {
		t.Errorf("stdout = %q, want the payload", out.String())
	}
}

func TestEndPicksOneOutcome(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	full := Outcomes{Done: "Installed jq 1.8.3", Failed: "Failed to install jq 1.8.3", Cancelled: "Cancelled jq 1.8.3"}
	noCancelText := Outcomes{Done: "Installed jq 1.8.3", Failed: "Failed to install jq 1.8.3"}
	cases := []struct {
		name string
		ctx  context.Context
		err  error
		o    Outcomes
		want string
	}{
		{"nil error under a cancelled context", cancelled, nil, full, "OK Installed jq 1.8.3\n"},
		{"cancellation error", context.Background(), context.Canceled, full, "ERROR Cancelled jq 1.8.3\n"},
		{"cancelled context with an error", cancelled, errors.New("boom"), full, "ERROR Cancelled jq 1.8.3\n"},
		{"other error", context.Background(), errors.New("boom"), full, "ERROR Failed to install jq 1.8.3\n"},
		{"cancellation without cancelled text", context.Background(), context.Canceled, noCancelText, "ERROR Failed to install jq 1.8.3\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _, errb := newTest(Options{Color: ColorNever})
			task := c.Start("Installing jq 1.8.3")
			got := task.End(tc.ctx, tc.err, tc.o)
			if got != tc.err {
				t.Errorf("End returned %v, want %v unchanged", got, tc.err)
			}
			if tc.err != nil && !errors.Is(got, tc.err) {
				t.Errorf("End returned %v, want errors.Is %v", got, tc.err)
			}
			if errb.String() != tc.want {
				t.Errorf("stderr = %q, want %q", errb.String(), tc.want)
			}
			task.End(tc.ctx, errors.New("later"), tc.o)
			if errb.String() != tc.want {
				t.Errorf("second End wrote: stderr = %q, want %q", errb.String(), tc.want)
			}
		})
	}
}

func TestProgressAfterDoneIsIgnored(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorNever})
	task := c.Start("Installing jq 1.8.3")
	task.Done("Installed jq 1.8.3")
	task.Phase("downloading")
	task.Progress(5, 10, Items)
	task.Infof("late %s", "line")
	task.Warn("late warning")
	if got, want := errb.String(), "OK Installed jq 1.8.3\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func TestTaskLinesRenderLikeNarratorLines(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorNever, Verbose: true})
	task := c.Start("Installing jq 1.8.3")
	task.Warnf("%s: slow", "jq")
	task.Info("step", 2)
	task.Success("fine")
	task.Debugf("detail %d", 1)
	task.Hint("Run it again")
	want := "WARN jq: slow\nINFO step 2\nOK fine\nDEBUG detail 1\n  hint: Run it again\n"
	if got := errb.String(); got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func TestTaskWritersAnnouncePendingLikeNarratorWriters(t *testing.T) {
	c, out, errb := newTest(Options{Color: ColorNever})
	task := c.Start("Building jq 1.8.3")
	fmt.Fprint(task.ErrOut(), "cc: warning\n")
	fmt.Fprint(task.Out(), "artifact\n")
	if got, want := errb.String(), "INFO Building jq 1.8.3\ncc: warning\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
	if got, want := out.String(), "artifact\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestRunTaskSetsPhaseAndEnds(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorNever})
	ctx := NewContext(context.Background(), c)
	o := Outcomes{Done: "Pulled catalog", Failed: "Failed to pull catalog", Cancelled: "Cancelled pull"}

	err := RunTask(ctx, "Pulling catalog", "copying", o, func(progress ProgressFunc) error {
		progress(3, 10, Items)
		c.mu.Lock()
		defer c.mu.Unlock()
		if len(c.tasks) != 1 {
			t.Fatalf("open tasks = %d, want 1", len(c.tasks))
		}
		if phase, trailing := c.tasks[0].snapshot(c.now()); phase != "copying" || trailing != "copying 3/10" {
			t.Errorf("snapshot = %q, %q; want copying, copying 3/10", phase, trailing)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunTask: %v", err)
	}
	if got, want := errb.String(), "OK Pulled catalog\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}

	errb.Reset()
	err = RunTask(ctx, "Pulling catalog", "copying", o, func(ProgressFunc) error { return context.Canceled })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunTask error = %v, want context.Canceled", err)
	}
	if got, want := errb.String(), "ERROR Cancelled pull\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func TestTaskIDsStartAtOnePerConsole(t *testing.T) {
	for range 2 {
		c, _, _ := newTest(Options{Color: ColorNever})
		a := c.Start("a").(*task)
		b := c.Start("b").(*task)
		if a.id != 1 || b.id != 2 {
			t.Errorf("ids = %d, %d; want 1, 2", a.id, b.id)
		}
	}
}

func TestSnapshotTrailingText(t *testing.T) {
	now, advance := fakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	c, _, _ := newTest(Options{Color: ColorNever, Now: now})
	tk := c.Start("Installing jq 1.8.3").(*task)

	if phase, trailing := tk.snapshot(now()); phase != "" || trailing != "" {
		t.Errorf("no phase: snapshot = %q, %q; want empty", phase, trailing)
	}
	tk.Phase("downloading")
	if _, trailing := tk.snapshot(now()); trailing != "downloading" {
		t.Errorf("trailing = %q, want downloading", trailing)
	}
	advance(autoElapsedAfter)
	if _, trailing := tk.snapshot(now()); trailing != "downloading (10s)" {
		t.Errorf("trailing = %q, want downloading (10s)", trailing)
	}
	tk.Progress(512, 1024, Bytes)
	if _, trailing := tk.snapshot(now()); trailing != "downloading 50%" {
		t.Errorf("trailing = %q, want downloading 50%%", trailing)
	}
	tk.Phase("extracting")
	if _, trailing := tk.snapshot(now()); trailing != "extracting" {
		t.Errorf("phase change must reset progress: trailing = %q", trailing)
	}
}

func TestProgressText(t *testing.T) {
	cases := []struct {
		done, total int64
		unit        Unit
		want        string
	}{
		{3, 10, Items, "3/10"},
		{3, -1, Items, "3"},
		{3, 0, Items, ""},
		{512, 1024, Bytes, "50%"},
		{2048, -1, Bytes, "2 KB"},
		{0, 0, Bytes, ""},
	}
	for _, tc := range cases {
		if got := progressText(tc.done, tc.total, tc.unit); got != tc.want {
			t.Errorf("progressText(%d, %d, %d) = %q, want %q", tc.done, tc.total, tc.unit, got, tc.want)
		}
	}
}
