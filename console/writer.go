package console

import "io"

type guardedWriter struct {
	c      *Console
	w      io.Writer
	t      *task
	stream string
}

func (g *guardedWriter) Write(p []byte) (int, error) {
	g.c.mu.Lock()
	defer g.c.mu.Unlock()
	return g.c.r.output(g.t, g.stream, g.w, p)
}

func (c *Console) guarded(w io.Writer, t *task, stream string) io.Writer {
	return &guardedWriter{c: c, w: w, t: t, stream: stream}
}

// Out returns a writer to stdout for output from another program, such as a child process.
func (c *Console) Out() io.Writer { return c.guarded(c.out, nil, "stdout") }

// ErrOut returns a writer to stderr for output from another program, such as a child process.
func (c *Console) ErrOut() io.Writer { return c.guarded(c.err, nil, "stderr") }
