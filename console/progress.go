package console

import "io"

type progressWriter struct {
	t     Task
	total int64
	sent  int64
}

// ProgressWriter returns a writer that reports the bytes written to it as t's progress toward total. A negative total means the size is unknown.
func ProgressWriter(t Task, total int64) io.Writer {
	return &progressWriter{t: t, total: total}
}

func (w *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	if n > 0 && w.t != nil {
		w.sent += int64(n)
		w.t.Progress(w.sent, w.total, Bytes)
	}
	return n, nil
}

type progressReader struct {
	t     Task
	r     io.Reader
	total int64
	sent  int64
}

// ProgressReader returns a reader over r that reports the bytes read as t's progress toward total. A negative total means the size is unknown.
func ProgressReader(t Task, r io.Reader, total int64) io.Reader {
	return &progressReader{t: t, r: r, total: total}
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 && p.t != nil {
		p.sent += int64(n)
		p.t.Progress(p.sent, p.total, Bytes)
	}
	return n, err
}
