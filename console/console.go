package console

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// ColorMode chooses when narration is in colour.
type ColorMode int

const (
	// ColorAuto colours narration when stderr is a terminal.
	ColorAuto ColorMode = iota
	// ColorAlways colours narration everywhere.
	ColorAlways
	// ColorNever never colours narration.
	ColorNever
)

// Narration chooses how narration is written to stderr.
type Narration int

const (
	// NarrationAuto shows a live block on a terminal and plain mode elsewhere.
	NarrationAuto Narration = iota
	// NarrationPlain writes plain lines, also on a terminal.
	NarrationPlain
	// NarrationJSON writes one JSON event per line.
	NarrationJSON
)

// Options configures a Console.
type Options struct {
	// Quiet hides narration except warnings, errors, failed tasks, questions, and debug lines under Verbose.
	Quiet bool
	// Verbose shows debug lines.
	Verbose bool
	// Color chooses when narration is in colour.
	Color ColorMode
	// Narration chooses live, plain or JSON narration.
	Narration Narration
	// IsTTY reports whether stderr is a terminal.
	IsTTY bool
	// OutIsTTY reports whether stdout is a terminal.
	OutIsTTY bool
	// Width returns the terminal width in columns; nil means 80.
	Width func() int
	// Now returns the current time; nil means the system clock.
	Now func() time.Time
	// ManualTick makes narration advance only on Tick, for tests.
	ManualTick bool
}

// Console prints a run's narration to stderr and its data to stdout.
type Console struct {
	out, err io.Writer
	opts     Options
	colored  bool
	now      func() time.Time
	width    func() int
	mu       sync.Mutex
	r        renderer
	seq      int
	lastID   int
	tasks    []*task
	pending  []*task
	exited   bool
	tk       *ticker
	gen      int
	ticks    int
}

const (
	tickInterval = 100 * time.Millisecond
	defaultWidth = 80
	exitLockWait = 50 * time.Millisecond
)

type ticker struct {
	gen  int
	stop chan struct{}
	done chan struct{}
}

// New returns a console that prints data to stdout and narration to stderr.
func New(stdout, stderr io.Writer, opts Options) *Console {
	c := &Console{
		out:     stdout,
		err:     stderr,
		opts:    opts,
		colored: opts.Color == ColorAlways || (opts.Color == ColorAuto && opts.IsTTY),
		now:     opts.Now,
		width:   opts.Width,
	}
	if c.now == nil {
		c.now = time.Now
	}
	if c.width == nil {
		c.width = func() int { return defaultWidth }
	}
	c.r = newRenderer(c)
	return c
}

// Silent returns a console that prints nothing.
func Silent() *Console {
	return New(io.Discard, io.Discard, Options{Color: ColorNever})
}

func (c *Console) emit(e Event, t *task) bool {
	if t != nil {
		e.Task = t.id
	}
	if c.exited || !c.allowed(e) {
		return false
	}
	c.seq++
	e.Seq = c.seq
	e.Time = c.now()
	c.r.event(e, t)
	return true
}

func (c *Console) allowed(e Event) bool {
	switch e.Event {
	case "line":
		switch e.Level {
		case "debug":
			return c.opts.Verbose
		case "info", "success", "hint":
			return !c.opts.Quiet
		default:
			return true
		}
	case "task":
		return e.State == "failed" || !c.opts.Quiet
	default:
		return true
	}
}

func (c *Console) line(level string, t *task, text, hint string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if t != nil && t.isCompleted() {
		return
	}
	c.emit(Event{Event: "line", Level: level, Text: text, Hint: hint}, t)
}

func sprintln(a ...any) string {
	return strings.TrimSuffix(fmt.Sprintln(a...), "\n")
}

// Info prints an INFO line, joining its arguments like fmt.Sprintln without the newline.
func (c *Console) Info(a ...any) { c.line("info", nil, sprintln(a...), "") }

// Infof prints an INFO line formatted like fmt.Sprintf.
func (c *Console) Infof(format string, a ...any) {
	c.line("info", nil, fmt.Sprintf(format, a...), "")
}

// Success prints an OK line, joining its arguments like fmt.Sprintln without the newline.
func (c *Console) Success(a ...any) { c.line("success", nil, sprintln(a...), "") }

// Successf prints an OK line formatted like fmt.Sprintf.
func (c *Console) Successf(format string, a ...any) {
	c.line("success", nil, fmt.Sprintf(format, a...), "")
}

// Warn prints a WARN line, joining its arguments like fmt.Sprintln without the newline.
func (c *Console) Warn(a ...any) { c.line("warn", nil, sprintln(a...), "") }

// Warnf prints a WARN line formatted like fmt.Sprintf.
func (c *Console) Warnf(format string, a ...any) {
	c.line("warn", nil, fmt.Sprintf(format, a...), "")
}

// Debug prints a DEBUG line, joining its arguments like fmt.Sprintln without the newline.
func (c *Console) Debug(a ...any) { c.line("debug", nil, sprintln(a...), "") }

// Debugf prints a DEBUG line formatted like fmt.Sprintf.
func (c *Console) Debugf(format string, a ...any) {
	c.line("debug", nil, fmt.Sprintf(format, a...), "")
}

// Hint prints a hint line, joining its arguments like fmt.Sprintln without the newline.
func (c *Console) Hint(a ...any) { c.line("hint", nil, sprintln(a...), "") }

// Hintf prints a hint line formatted like fmt.Sprintf.
func (c *Console) Hintf(format string, a ...any) {
	c.line("hint", nil, fmt.Sprintf(format, a...), "")
}

// Error prints the ERROR line for err, with hint under it unless hint is empty.
func (c *Console) Error(err error, hint string) {
	c.line("error", nil, capitalizeLead(err.Error()), hint)
}

// Prompt prints a question and leaves the line open, joining its arguments like fmt.Sprintln without the newline.
func (c *Console) Prompt(a ...any) { c.line("prompt", nil, sprintln(a...), "") }

// Promptf prints a question formatted like fmt.Sprintf and leaves the line open.
func (c *Console) Promptf(format string, a ...any) {
	c.line("prompt", nil, fmt.Sprintf(format, a...), "")
}

// Answer prints the answer to a question and ends the line.
func (c *Console) Answer(text string) { c.line("answer", nil, text, "") }

// BlankErr prints a blank line on stderr, except in quiet mode.
func (c *Console) BlankErr() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.opts.Quiet || c.exited {
		return
	}
	c.r.blank()
}

// Tick moves narration on by one step: it redraws the live block, or in JSON mode may write progress events.
func (c *Console) Tick() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.exited {
		return
	}
	c.step()
}

func (c *Console) step() {
	c.ticks++
	c.r.tick(c.now(), c.ticks)
}

func (c *Console) tick(gen int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.exited || c.tk == nil || c.tk.gen != gen {
		return
	}
	c.step()
}

func (c *Console) startTicker() {
	if c.opts.ManualTick || c.tk != nil {
		return
	}
	c.gen++
	tk := &ticker{gen: c.gen, stop: make(chan struct{}), done: make(chan struct{})}
	c.tk = tk
	go c.runTicker(tk)
}

func (c *Console) runTicker(tk *ticker) {
	defer close(tk.done)
	t := time.NewTicker(tickInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			c.tick(tk.gen)
		case <-tk.stop:
			return
		}
	}
}

func (c *Console) stopTicker() (done <-chan struct{}) {
	tk := c.tk
	if tk == nil {
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	c.tk = nil
	close(tk.stop)
	return tk.done
}

// Begin opens a JSON narration stream with the app name and version, and prints nothing in other modes.
func (c *Console) Begin(app, version string) {
	if c.opts.Narration != NarrationJSON {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.emit(Event{Event: "begin", App: app, Version: version, Schema: schemaVersion}, nil)
}

// Exit ends narration with the exit code and the signal that ended the run, or nil for none; nothing prints after it.
func (c *Console) Exit(code int, sig os.Signal) {
	if !c.lockWithin(exitLockWait) {
		return
	}
	if c.exited {
		c.mu.Unlock()
		return
	}
	c.r.finish()
	c.emit(Event{Event: "exit", Code: code, Signal: signalName(sig)}, nil)
	c.exited = true
	done := c.stopTicker()
	c.mu.Unlock()
	timer := time.NewTimer(exitLockWait)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	}
}

func (c *Console) lockWithin(d time.Duration) bool {
	var state atomic.Int32
	won := make(chan struct{})
	go func() {
		c.mu.Lock()
		if !state.CompareAndSwap(0, 1) {
			c.mu.Unlock()
			return
		}
		close(won)
	}()
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-won:
		return true
	case <-timer.C:
		if state.CompareAndSwap(0, 2) {
			return false
		}
		<-won
		return true
	}
}

func signalName(sig os.Signal) string {
	switch sig {
	case nil:
		return ""
	case syscall.SIGINT:
		return "SIGINT"
	case syscall.SIGTERM:
		return "SIGTERM"
	default:
		return sig.String()
	}
}
