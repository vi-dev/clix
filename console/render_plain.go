package console

import (
	"bytes"
	"io"
	"strings"
	"time"
)

const (
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiRed    = "\x1b[31m"
	ansiCyan   = "\x1b[36m"
	ansiDim    = "\x1b[2m"
	ansiReset  = "\x1b[0m"
)

type plainRenderer struct {
	c *Console
}

func (p plainRenderer) event(e Event, t *task) {
	var b bytes.Buffer
	switch {
	case e.Event == "line":
		p.writeLine(&b, e)
	case e.Event == "task" && e.State == "started":
		p.c.pending = append(p.c.pending, t)
	case e.Event == "task":
		p.writeTaskEnd(&b, e)
	}
	if b.Len() > 0 {
		_, _ = p.c.err.Write(b.Bytes())
	}
}

func (p plainRenderer) writeTaskEnd(b *bytes.Buffer, e Event) {
	switch e.State {
	case "done":
		elapsed := time.Duration(e.ElapsedMS) * time.Millisecond
		p.writeLine(b, Event{Level: "success", Text: e.Outcome + FormatDuration(elapsed)})
	case "failed":
		if p.c.colored {
			b.WriteString(ansiRed + "✗ " + e.Outcome + ansiReset + "\n")
		} else {
			b.WriteString("ERROR " + e.Outcome + "\n")
		}
	}
}

func (p plainRenderer) output(_ *task, _ string, w io.Writer, b []byte) (int, error) {
	if !p.c.exited {
		p.announcePending()
	}
	return w.Write(b)
}

func (plainRenderer) beforeData() {}

func (plainRenderer) tick(time.Time, int) {}

func (plainRenderer) wantsTicks() bool { return false }

func (plainRenderer) finish() {}

func (plainRenderer) taskEnd(*task) {}

func (p plainRenderer) blank() {
	_, _ = io.WriteString(p.c.err, "\n")
}

func (p plainRenderer) announcePending() {
	if len(p.c.pending) == 0 {
		return
	}
	var b bytes.Buffer
	for _, t := range p.c.pending {
		p.writeInfo(&b, t.label)
	}
	p.c.pending = nil
	_, _ = p.c.err.Write(b.Bytes())
}

func (p plainRenderer) writeLine(b *bytes.Buffer, e Event) {
	colored := p.c.colored
	switch e.Level {
	case "info":
		p.writeInfo(b, e.Text)
	case "success":
		if colored {
			b.WriteString(ansiGreen + "✓" + ansiReset + " " + e.Text + "\n")
		} else {
			b.WriteString("OK " + e.Text + "\n")
		}
	case "warn":
		if colored {
			b.WriteString(ansiYellow + "!" + ansiReset + " " + e.Text + "\n")
		} else {
			b.WriteString("WARN " + e.Text + "\n")
		}
	case "debug":
		if colored {
			b.WriteString(ansiDim + "· " + e.Text + ansiReset + "\n")
		} else {
			b.WriteString("DEBUG " + e.Text + "\n")
		}
	case "hint":
		p.writeHint(b, e.Text)
	case "prompt":
		if colored {
			b.WriteString(ansiCyan + "?" + ansiReset + " " + e.Text)
		} else {
			b.WriteString("? " + e.Text)
		}
	case "answer":
		b.WriteString(e.Text + "\n")
	case "error":
		p.writeError(b, e.Text)
		if e.Hint != "" {
			p.writeHint(b, e.Hint)
		}
	}
}

func (p plainRenderer) writeInfo(b *bytes.Buffer, text string) {
	if p.c.colored {
		b.WriteString(ansiCyan + "•" + ansiReset + " " + text + "\n")
	} else {
		b.WriteString("INFO " + text + "\n")
	}
}

func (p plainRenderer) writeHint(b *bytes.Buffer, text string) {
	if p.c.colored {
		b.WriteString("  " + ansiDim + "→ " + text + ansiReset + "\n")
	} else {
		b.WriteString("  hint: " + text + "\n")
	}
}

func (p plainRenderer) writeError(b *bytes.Buffer, text string) {
	first, rest, _ := strings.Cut(strings.TrimRight(text, "\n"), "\n")
	if p.c.colored {
		b.WriteString(ansiRed + "✗ " + first + ansiReset + "\n")
	} else {
		b.WriteString("ERROR " + first + "\n")
	}
	if rest == "" {
		return
	}
	for line := range strings.SplitSeq(rest, "\n") {
		if p.c.colored {
			b.WriteString(ansiRed + "  " + line + ansiReset + "\n")
		} else {
			b.WriteString("      " + line + "\n")
		}
	}
}
