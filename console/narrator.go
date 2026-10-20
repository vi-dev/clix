package console

import (
	"fmt"
	"io"
)

// Lines prints narration lines at each level.
type Lines interface {
	// Info prints an INFO line, joining its arguments like fmt.Sprintln without the newline.
	Info(a ...any)
	// Infof prints an INFO line formatted like fmt.Sprintf.
	Infof(format string, a ...any)
	// Success prints an OK line, joining its arguments like fmt.Sprintln without the newline.
	Success(a ...any)
	// Successf prints an OK line formatted like fmt.Sprintf.
	Successf(format string, a ...any)
	// Warn prints a WARN line, joining its arguments like fmt.Sprintln without the newline.
	Warn(a ...any)
	// Warnf prints a WARN line formatted like fmt.Sprintf.
	Warnf(format string, a ...any)
	// Debug prints a DEBUG line, joining its arguments like fmt.Sprintln without the newline.
	Debug(a ...any)
	// Debugf prints a DEBUG line formatted like fmt.Sprintf.
	Debugf(format string, a ...any)
	// Hint prints a hint line, joining its arguments like fmt.Sprintln without the newline.
	Hint(a ...any)
	// Hintf prints a hint line formatted like fmt.Sprintf.
	Hintf(format string, a ...any)
}

// Starter starts tasks.
type Starter interface {
	// Start begins a task with label and returns it.
	Start(label string) Task
}

// Streams gives writers for output from other programs, such as a child process.
type Streams interface {
	// Out returns a writer to stdout.
	Out() io.Writer
	// ErrOut returns a writer to stderr.
	ErrOut() io.Writer
}

// Narrator is the narration a console offers to code below a command: lines, tasks and streams.
type Narrator interface {
	Lines
	Starter
	Streams
}

var _ Narrator = (*Console)(nil)

type prefixed struct {
	n      Narrator
	prefix string
}

// Prefixed returns a narrator that passes to n every line and task label with prefix in front.
func Prefixed(n Narrator, prefix string) Narrator {
	return &prefixed{n: n, prefix: prefix}
}

func (p *prefixed) Info(a ...any)  { p.n.Info(p.prefix + sprintln(a...)) }
func (p *prefixed) Warn(a ...any)  { p.n.Warn(p.prefix + sprintln(a...)) }
func (p *prefixed) Debug(a ...any) { p.n.Debug(p.prefix + sprintln(a...)) }
func (p *prefixed) Hint(a ...any)  { p.n.Hint(p.prefix + sprintln(a...)) }

func (p *prefixed) Success(a ...any) { p.n.Success(p.prefix + sprintln(a...)) }

func (p *prefixed) Infof(format string, a ...any) {
	p.n.Info(p.prefix + fmt.Sprintf(format, a...))
}

func (p *prefixed) Successf(format string, a ...any) {
	p.n.Success(p.prefix + fmt.Sprintf(format, a...))
}

func (p *prefixed) Warnf(format string, a ...any) {
	p.n.Warn(p.prefix + fmt.Sprintf(format, a...))
}

func (p *prefixed) Debugf(format string, a ...any) {
	p.n.Debug(p.prefix + fmt.Sprintf(format, a...))
}

func (p *prefixed) Hintf(format string, a ...any) {
	p.n.Hint(p.prefix + fmt.Sprintf(format, a...))
}

func (p *prefixed) Start(label string) Task { return p.n.Start(p.prefix + label) }

func (p *prefixed) Out() io.Writer { return p.n.Out() }

func (p *prefixed) ErrOut() io.Writer { return p.n.ErrOut() }
