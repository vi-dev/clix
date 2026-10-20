package console

import (
	"context"
	"fmt"
	"io"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// Unit is what a task's progress counts.
type Unit int

const (
	// Items counts items, shown as done/total, or the count alone when the total is unknown.
	Items Unit = iota
	// Bytes counts bytes, shown as a percentage, or as a size when the total is unknown.
	Bytes
)

// Task is a piece of work shown while it runs and closed with an outcome.
type Task interface {
	Lines
	// Phase sets the step the task is on.
	Phase(phase string)
	// Progress reports how much of the current phase is done; a negative total means unknown.
	Progress(done, total int64, unit Unit)
	// Done closes the task with an OK outcome.
	Done(outcome string)
	// Fail closes the task with a failed outcome, shown even in quiet mode.
	Fail(outcome string)
	// Discard closes the task without printing anything.
	Discard()
	// End closes the task with the outcome from o that fits err and ctx, and returns err.
	End(ctx context.Context, err error, o Outcomes) error
	// Out returns a writer to stdout for output that belongs to the task.
	Out() io.Writer
	// ErrOut returns a writer to stderr for output that belongs to the task.
	ErrOut() io.Writer
}

// Outcomes holds the texts End closes a task with.
type Outcomes struct {
	// Done, Failed and Cancelled are for success, failure and cancellation; an empty Cancelled means Failed.
	Done, Failed, Cancelled string
}

// ProgressFunc reports progress like Task.Progress.
type ProgressFunc func(done, total int64, unit Unit)

const autoElapsedAfter = 10 * time.Second

type task struct {
	c          *Console
	id         int
	label      string
	start      time.Time
	done       atomic.Int64
	total      atomic.Int64
	unit       atomic.Int32
	mu         sync.Mutex
	phase      string
	phaseStart time.Time
	completed  bool
	visible    bool
}

// Start begins a task with label and returns it.
func (c *Console) Start(label string) Task {
	t := &task{c: c, label: label, start: c.now()}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastID++
	t.id = c.lastID
	c.tasks = append(c.tasks, t)
	if !c.emit(Event{Event: "task", State: "started", Label: label}, t) {
		c.tasks = c.tasks[:len(c.tasks)-1]
		return t
	}
	t.visible = true
	if len(c.tasks) == 1 && c.r.wantsTicks() {
		c.startTicker()
	}
	return t
}

func (t *task) Phase(phase string) {
	c := t.c
	c.mu.Lock()
	defer c.mu.Unlock()
	t.mu.Lock()
	if t.completed {
		t.mu.Unlock()
		return
	}
	if t.phase != "" && t.phase != phase {
		t.done.Store(0)
		t.total.Store(0)
		t.unit.Store(0)
	}
	t.phase = phase
	t.phaseStart = c.now()
	t.mu.Unlock()
	c.emit(Event{Event: "task", State: "phase", Phase: phase}, t)
}

func (t *task) Progress(done, total int64, unit Unit) {
	t.done.Store(done)
	t.total.Store(total)
	t.unit.Store(int32(unit))
}

func (t *task) Done(outcome string) { t.complete("done", outcome) }

func (t *task) Fail(outcome string) { t.complete("failed", outcome) }

func (t *task) Discard() { t.complete("discarded", "") }

func (t *task) End(ctx context.Context, err error, o Outcomes) error {
	switch {
	case err == nil:
		t.Done(o.Done)
	case Cancelled(ctx, err) && o.Cancelled != "":
		t.Fail(o.Cancelled)
	default:
		t.Fail(o.Failed)
	}
	return err
}

func (t *task) markCompleted() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.completed {
		return false
	}
	t.completed = true
	return true
}

func (t *task) isCompleted() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.completed
}

func (t *task) complete(state, outcome string) {
	if !t.markCompleted() {
		return
	}
	c := t.c
	c.mu.Lock()
	if i := slices.Index(c.tasks, t); i >= 0 {
		c.tasks = slices.Delete(c.tasks, i, i+1)
	}
	if i := slices.Index(c.pending, t); i >= 0 {
		c.pending = slices.Delete(c.pending, i, i+1)
	}
	c.r.taskEnd(t)
	e := Event{Event: "task", State: state}
	if state != "discarded" {
		e.Outcome = outcome
		e.ElapsedMS = c.now().Sub(t.start).Milliseconds()
	}
	c.emit(e, t)
	var done <-chan struct{}
	if t.visible && len(c.tasks) == 0 {
		done = c.stopTicker()
	}
	c.mu.Unlock()
	if done != nil {
		<-done
	}
}

func (t *task) snapshot(now time.Time) (phase, trailing string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.phase == "" {
		return "", ""
	}
	if p := progressText(t.done.Load(), t.total.Load(), Unit(t.unit.Load())); p != "" {
		return t.phase, t.phase + " " + p
	}
	if elapsed := now.Sub(t.phaseStart); elapsed >= autoElapsedAfter {
		return t.phase, t.phase + FormatDuration(elapsed)
	}
	return t.phase, t.phase
}

func (t *task) renderLine(now time.Time, width int, colored bool, spinner string) string {
	_, trailing := t.snapshot(now)
	return formatTaskLine(spinner, t.label, trailing, width, colored)
}

func progressText(done, total int64, unit Unit) string {
	switch {
	case unit == Items && total > 0:
		return fmt.Sprintf("%d/%d", done, total)
	case unit == Items && total < 0:
		return fmt.Sprintf("%d", done)
	case unit == Bytes && total > 0:
		return fmt.Sprintf("%d%%", int(float64(done)/float64(total)*100))
	case unit == Bytes && total < 0:
		return FormatBytes(done)
	default:
		return ""
	}
}

func (t *task) Info(a ...any) { t.c.line("info", t, sprintln(a...), "") }

func (t *task) Infof(format string, a ...any) {
	t.c.line("info", t, fmt.Sprintf(format, a...), "")
}

func (t *task) Success(a ...any) { t.c.line("success", t, sprintln(a...), "") }

func (t *task) Successf(format string, a ...any) {
	t.c.line("success", t, fmt.Sprintf(format, a...), "")
}

func (t *task) Warn(a ...any) { t.c.line("warn", t, sprintln(a...), "") }

func (t *task) Warnf(format string, a ...any) {
	t.c.line("warn", t, fmt.Sprintf(format, a...), "")
}

func (t *task) Debug(a ...any) { t.c.line("debug", t, sprintln(a...), "") }

func (t *task) Debugf(format string, a ...any) {
	t.c.line("debug", t, fmt.Sprintf(format, a...), "")
}

func (t *task) Hint(a ...any) { t.c.line("hint", t, sprintln(a...), "") }

func (t *task) Hintf(format string, a ...any) {
	t.c.line("hint", t, fmt.Sprintf(format, a...), "")
}

func (t *task) Out() io.Writer { return t.c.guarded(t.c.out, t, "stdout") }

func (t *task) ErrOut() io.Writer { return t.c.guarded(t.c.err, t, "stderr") }

// RunTask runs fn as a task with label and phase on the narrator in ctx, closes it with an outcome from o, and returns fn's error.
func RunTask(ctx context.Context, label, phase string, o Outcomes, fn func(progress ProgressFunc) error) error {
	t := FromContext(ctx).Start(label)
	t.Phase(phase)
	return t.End(ctx, fn(t.Progress), o)
}
