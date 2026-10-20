package console

import (
	"bytes"
	"io"
	"slices"
	"strings"
	"time"
)

const lineBufferCap = 8192

type bufferKey struct {
	t      *task
	stream string
}

type sample struct {
	phase       string
	done, total int64
	unit        Unit
}

type jsonRenderer struct {
	c       *Console
	enc     bytes.Buffer
	buffers map[bufferKey]*lineBuffer
	order   []bufferKey
	last    map[*task]sample
}

func newJSONRenderer(c *Console) *jsonRenderer {
	return &jsonRenderer{
		c:       c,
		buffers: map[bufferKey]*lineBuffer{},
		last:    map[*task]sample{},
	}
}

func (r *jsonRenderer) event(e Event, _ *task) {
	b, err := e.MarshalJSON()
	if err != nil {
		return
	}
	r.enc.Reset()
	r.enc.Write(b)
	r.enc.WriteByte('\n')
	_, _ = r.c.err.Write(r.enc.Bytes())
}

func (r *jsonRenderer) output(t *task, stream string, w io.Writer, p []byte) (int, error) {
	if t == nil && stream == "stdout" {
		return w.Write(p)
	}
	if r.c.exited {
		return len(p), nil
	}
	key := bufferKey{t: t, stream: stream}
	b, ok := r.buffers[key]
	if !ok {
		b = &lineBuffer{}
		r.buffers[key] = b
		r.order = append(r.order, key)
	}
	b.add(p, r.emitOutput(key))
	if t != nil && t.isCompleted() {
		r.flushKey(key)
	}
	return len(p), nil
}

func (r *jsonRenderer) emitOutput(key bufferKey) func(text string, partial bool) {
	return func(text string, partial bool) {
		r.c.emit(Event{
			Event:   "output",
			Stream:  key.stream,
			Text:    strings.ToValidUTF8(text, "�"),
			Partial: partial,
		}, key.t)
	}
}

func (r *jsonRenderer) flushKey(key bufferKey) {
	b, ok := r.buffers[key]
	if !ok {
		return
	}
	b.flush(r.emitOutput(key))
	delete(r.buffers, key)
	r.order = slices.DeleteFunc(r.order, func(k bufferKey) bool { return k == key })
}

func (r *jsonRenderer) taskEnd(t *task) {
	r.flushKey(bufferKey{t: t, stream: "stdout"})
	r.flushKey(bufferKey{t: t, stream: "stderr"})
	delete(r.last, t)
}

func (*jsonRenderer) beforeData() {}

func (r *jsonRenderer) tick(_ time.Time, n int) {
	if n%10 != 0 {
		return
	}
	for _, t := range r.c.tasks {
		t.mu.Lock()
		s := sample{
			phase: t.phase,
			done:  t.done.Load(),
			total: t.total.Load(),
			unit:  Unit(t.unit.Load()),
		}
		t.mu.Unlock()
		if s.phase == "" || progressText(s.done, s.total, s.unit) == "" || r.last[t] == s {
			continue
		}
		r.last[t] = s
		r.c.emit(Event{
			Event: "task",
			State: "progress",
			Phase: s.phase,
			Done:  s.done,
			Total: s.total,
			Unit:  unitName(s.unit),
		}, t)
	}
}

func (*jsonRenderer) wantsTicks() bool { return true }

func (*jsonRenderer) blank() {}

func (r *jsonRenderer) finish() {
	for _, key := range r.order {
		r.buffers[key].flush(r.emitOutput(key))
	}
	r.order = nil
	clear(r.buffers)
	clear(r.last)
}

func unitName(u Unit) string {
	if u == Bytes {
		return "bytes"
	}
	return "items"
}

type lineBuffer struct {
	buf []byte
}

func (b *lineBuffer) add(p []byte, emit func(text string, partial bool)) {
	for len(p) > 0 {
		room := lineBufferCap - len(b.buf)
		if i := bytes.IndexAny(p, "\r\n"); i >= 0 && i < room {
			end := i + 1
			if p[i] == '\r' && end < len(p) && end < room && p[end] == '\n' {
				end++
			}
			b.buf = append(b.buf, p[:end]...)
			p = p[end:]
			b.emit(emit, false)
			continue
		}
		if len(p) < room {
			b.buf = append(b.buf, p...)
			return
		}
		b.buf = append(b.buf, p[:room]...)
		p = p[room:]
		b.emit(emit, true)
	}
}

func (b *lineBuffer) flush(emit func(text string, partial bool)) {
	if len(b.buf) == 0 {
		return
	}
	b.emit(emit, false)
}

func (b *lineBuffer) emit(emit func(text string, partial bool), partial bool) {
	text := string(b.buf)
	b.buf = b.buf[:0]
	emit(text, partial)
}
