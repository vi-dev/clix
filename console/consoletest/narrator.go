package consoletest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/vi-dev/clix/console"
)

type recorder struct {
	mu        sync.Mutex
	infos     []string
	successes []string
	warns     []string
	hints     []string
}

func (r *recorder) add(list *[]string, text string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	*list = append(*list, text)
}

func (r *recorder) read(list *[]string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), *list...)
}

// Info records an info line, joining its arguments like fmt.Sprintln without the newline.
func (r *recorder) Info(a ...any) { r.add(&r.infos, sprintln(a...)) }

// Infof records an info line formatted like fmt.Sprintf.
func (r *recorder) Infof(format string, a ...any) { r.add(&r.infos, fmt.Sprintf(format, a...)) }

// Success records an OK line, joining its arguments like fmt.Sprintln without the newline.
func (r *recorder) Success(a ...any) { r.add(&r.successes, sprintln(a...)) }

// Successf records an OK line formatted like fmt.Sprintf.
func (r *recorder) Successf(format string, a ...any) {
	r.add(&r.successes, fmt.Sprintf(format, a...))
}

// Warn records a warning, joining its arguments like fmt.Sprintln without the newline.
func (r *recorder) Warn(a ...any) { r.add(&r.warns, sprintln(a...)) }

// Warnf records a warning formatted like fmt.Sprintf.
func (r *recorder) Warnf(format string, a ...any) { r.add(&r.warns, fmt.Sprintf(format, a...)) }

// Debug drops the line.
func (r *recorder) Debug(...any) {}

// Debugf drops the line.
func (r *recorder) Debugf(string, ...any) {}

// Hint records a hint, joining its arguments like fmt.Sprintln without the newline.
func (r *recorder) Hint(a ...any) { r.add(&r.hints, sprintln(a...)) }

// Hintf records a hint formatted like fmt.Sprintf.
func (r *recorder) Hintf(format string, a ...any) { r.add(&r.hints, fmt.Sprintf(format, a...)) }

// Infos returns the info lines recorded so far.
func (r *recorder) Infos() []string { return r.read(&r.infos) }

// Successes returns the OK lines recorded so far.
func (r *recorder) Successes() []string { return r.read(&r.successes) }

// Warns returns the warnings recorded so far.
func (r *recorder) Warns() []string { return r.read(&r.warns) }

// Hints returns the hints recorded so far.
func (r *recorder) Hints() []string { return r.read(&r.hints) }

func sprintln(a ...any) string {
	return strings.TrimSuffix(fmt.Sprintln(a...), "\n")
}

// Narrator records narration for a test to check; it drops debug lines and anything written to its streams.
type Narrator struct {
	recorder
	tasksMu sync.Mutex
	tasks   []*Task
}

var _ console.Narrator = (*Narrator)(nil)

// Start records a new task with label and returns it.
func (n *Narrator) Start(label string) console.Task {
	t := &Task{Label: label}
	n.tasksMu.Lock()
	defer n.tasksMu.Unlock()
	n.tasks = append(n.tasks, t)
	return t
}

// Out returns a writer that drops what is written.
func (n *Narrator) Out() io.Writer { return io.Discard }

// ErrOut returns a writer that drops what is written.
func (n *Narrator) ErrOut() io.Writer { return io.Discard }

// Tasks returns the tasks started so far, in order.
func (n *Narrator) Tasks() []*Task {
	n.tasksMu.Lock()
	defer n.tasksMu.Unlock()
	return append([]*Task(nil), n.tasks...)
}

// TaskFor returns the first task started with label, or nil.
func (n *Narrator) TaskFor(label string) *Task {
	n.tasksMu.Lock()
	defer n.tasksMu.Unlock()
	for _, t := range n.tasks {
		if t.Label == label {
			return t
		}
	}
	return nil
}

// ProgressCall is one recorded call to Task.Progress.
type ProgressCall struct {
	// Done and Total are the amounts passed.
	Done, Total int64
	// Unit is the unit passed.
	Unit console.Unit
}

// Task records what code did with a task for a test to check.
type Task struct {
	// Label is the label the task started with.
	Label string

	recorder
	stateMu   sync.Mutex
	phases    []string
	progress  []ProgressCall
	done      bool
	failed    bool
	discarded bool
	outcome   string
	output    bytes.Buffer
}

var _ console.Task = (*Task)(nil)

// Phase records a phase.
func (t *Task) Phase(phase string) {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	t.phases = append(t.phases, phase)
}

// Progress records a progress call.
func (t *Task) Progress(done, total int64, unit console.Unit) {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	t.progress = append(t.progress, ProgressCall{Done: done, Total: total, Unit: unit})
}

// Done records the task as done with outcome.
func (t *Task) Done(outcome string) {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	t.done = true
	t.outcome = outcome
}

// Fail records the task as failed with outcome.
func (t *Task) Fail(outcome string) {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	t.failed = true
	t.outcome = outcome
}

// Discard records the task as discarded.
func (t *Task) Discard() {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	t.discarded = true
}

// End records the outcome from o that fits err and ctx, as a console task would, and returns err.
func (t *Task) End(ctx context.Context, err error, o console.Outcomes) error {
	switch {
	case err == nil:
		t.Done(o.Done)
	case console.Cancelled(ctx, err) && o.Cancelled != "":
		t.Fail(o.Cancelled)
	default:
		t.Fail(o.Failed)
	}
	return err
}

// Out returns a writer whose text Output reads back.
func (t *Task) Out() io.Writer { return taskWriter{t} }

// ErrOut returns a writer whose text Output reads back.
func (t *Task) ErrOut() io.Writer { return taskWriter{t} }

// Phases returns the phases recorded, in order.
func (t *Task) Phases() []string {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	return append([]string(nil), t.phases...)
}

// ProgressCalls returns the progress calls recorded, in order.
func (t *Task) ProgressCalls() []ProgressCall {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	return append([]ProgressCall(nil), t.progress...)
}

// Snapshot reports whether the task is done or failed, and the outcome it was given.
func (t *Task) Snapshot() (done, failed bool, outcome string) {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	return t.done, t.failed, t.outcome
}

// Discarded reports whether the task was discarded.
func (t *Task) Discarded() bool {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	return t.discarded
}

// Output returns everything written to Out and ErrOut.
func (t *Task) Output() string {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	return t.output.String()
}

type taskWriter struct{ t *Task }

func (w taskWriter) Write(p []byte) (int, error) {
	w.t.stateMu.Lock()
	defer w.t.stateMu.Unlock()
	return w.t.output.Write(p)
}
