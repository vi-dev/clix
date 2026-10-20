package console

import (
	"bytes"
	"fmt"
	"io"
	"time"
	"unicode/utf8"
)

var (
	spinnerUnicode = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinnerASCII   = []string{"|", "/", "-", "\\"}
)

type liveRenderer struct {
	c     *Console
	lines int
	frame int
	buf   bytes.Buffer
}

func (r *liveRenderer) plain() plainRenderer { return plainRenderer{c: r.c} }

func (r *liveRenderer) event(e Event, t *task) {
	switch {
	case e.Event == "line":
		r.around(func(b *bytes.Buffer) { r.plain().writeLine(b, e) })
	case e.Event == "task" && e.State == "started":
		r.buf.Reset()
		r.appendBlock(r.c.now())
		r.flush()
	case e.Event == "task" && (e.State == "done" || e.State == "failed" || e.State == "discarded"):
		if !t.visible {
			r.buf.Reset()
			r.plain().writeTaskEnd(&r.buf, e)
			r.flush()
			return
		}
		r.buf.Reset()
		r.appendClear()
		r.plain().writeTaskEnd(&r.buf, e)
		r.appendBlock(r.c.now())
		r.flush()
	}
}

func (r *liveRenderer) around(print func(b *bytes.Buffer)) {
	r.buf.Reset()
	if len(r.c.tasks) == 0 {
		print(&r.buf)
		r.flush()
		return
	}
	r.appendClear()
	print(&r.buf)
	r.appendBlock(r.c.now())
	r.flush()
}

func (r *liveRenderer) output(_ *task, stream string, w io.Writer, p []byte) (int, error) {
	r.buf.Reset()
	r.appendClear()
	if stream != "stderr" {
		r.flush()
		return w.Write(p)
	}
	head := r.buf.Len()
	if head == 0 {
		return w.Write(p)
	}
	r.buf.Write(p)
	n, err := w.Write(r.buf.Bytes())
	r.buf.Reset()
	return min(max(n-head, 0), len(p)), err
}

func (r *liveRenderer) beforeData() {
	if !r.c.opts.OutIsTTY || !r.c.opts.IsTTY {
		return
	}
	r.buf.Reset()
	r.appendClear()
	r.flush()
}

func (r *liveRenderer) tick(now time.Time, _ int) {
	r.repaint(now)
}

func (*liveRenderer) wantsTicks() bool { return true }

func (r *liveRenderer) blank() {
	r.around(func(b *bytes.Buffer) { b.WriteString("\n") })
}

func (r *liveRenderer) finish() {
	r.clear()
}

func (*liveRenderer) taskEnd(*task) {}

func (r *liveRenderer) repaint(now time.Time) {
	r.buf.Reset()
	r.appendBlock(now)
	r.flush()
}

func (r *liveRenderer) clear() {
	r.buf.Reset()
	r.appendClear()
	r.flush()
}

func (r *liveRenderer) appendClear() {
	if r.lines == 0 {
		return
	}
	fmt.Fprintf(&r.buf, "\x1b[%dA\x1b[J", r.lines)
	r.lines = 0
}

func (r *liveRenderer) appendBlock(now time.Time) {
	if r.lines > 0 {
		fmt.Fprintf(&r.buf, "\x1b[%dA", r.lines)
	}
	width := r.c.width()
	spinner := spinnerFrame(r.frame, r.c.colored)
	r.frame++
	for _, t := range r.c.tasks {
		r.buf.WriteString("\x1b[2K")
		r.buf.WriteString(t.renderLine(now, width, r.c.colored, spinner))
		r.buf.WriteString("\n")
	}
	r.lines = len(r.c.tasks)
}

func (r *liveRenderer) flush() {
	if r.buf.Len() > 0 {
		_, _ = r.c.err.Write(r.buf.Bytes())
	}
	r.buf.Reset()
}

func spinnerFrame(i int, colored bool) string {
	frames := spinnerASCII
	if colored {
		frames = spinnerUnicode
	}
	return frames[i%len(frames)]
}

func formatTaskLine(spinner, label, trailing string, width int, colored bool) string {
	if width <= 0 {
		width = defaultWidth
	}
	tail := ""
	if trailing != "" {
		tail = "  " + trailing
	}
	line := spinner + " " + label + tail
	if displayWidth(line) > width {
		budget := width - displayWidth(spinner) - 1 - displayWidth(tail)
		if budget >= 2 {
			label = ellipsize(label, budget)
			line = spinner + " " + label + tail
		} else {
			label = "…"
			line = truncate(spinner+" "+label+tail, width)
		}
	}
	if !colored {
		return line
	}
	runes := []rune(line)
	spinnerLen := min(utf8.RuneCountInString(spinner), len(runes))
	head := ansiCyan + string(runes[:spinnerLen]) + ansiReset
	rest := runes[spinnerLen:]
	splitAt := utf8.RuneCountInString(label) + 3
	if trailing == "" || splitAt >= len(rest) {
		return head + string(rest)
	}
	return head + string(rest[:splitAt]) + ansiDim + string(rest[splitAt:]) + ansiReset
}
