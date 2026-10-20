package console

import (
	"io"
	"time"
)

type renderer interface {
	event(e Event, t *task)
	output(t *task, stream string, w io.Writer, p []byte) (int, error)
	beforeData()
	tick(now time.Time, n int)
	wantsTicks() bool
	blank()
	finish()
	taskEnd(t *task)
}

func newRenderer(c *Console) renderer {
	switch {
	case c.opts.Narration == NarrationJSON:
		return newJSONRenderer(c)
	case c.opts.Narration == NarrationAuto && c.opts.IsTTY:
		return &liveRenderer{c: c}
	default:
		return plainRenderer{c: c}
	}
}
