package console

import (
	"context"
	"log/slog"
	"strings"
)

type slogHandler struct {
	c      *Console
	attrs  []slog.Attr
	groups []string
}

// SlogHandler returns a slog handler that prints warnings and errors on c as WARN lines and other records as DEBUG lines.
func SlogHandler(c *Console) slog.Handler {
	return &slogHandler{c: c}
}

func (h *slogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= slog.LevelWarn || h.c.opts.Verbose
}

func (h *slogHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Message)
	for _, a := range h.attrs {
		writeAttr(&b, a)
	}
	prefix := strings.Join(h.groups, ".")
	r.Attrs(func(a slog.Attr) bool {
		writeAttr(&b, prefixAttr(prefix, a))
		return true
	})
	if r.Level >= slog.LevelWarn {
		h.c.Warn(b.String())
	} else {
		h.c.Debug(b.String())
	}
	return nil
}

func (h *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	prefix := strings.Join(h.groups, ".")
	next := *h
	next.attrs = append([]slog.Attr(nil), h.attrs...)
	for _, a := range attrs {
		next.attrs = append(next.attrs, prefixAttr(prefix, a))
	}
	return &next
}

func (h *slogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	next := *h
	next.groups = append(append([]string(nil), h.groups...), name)
	return &next
}

func prefixAttr(prefix string, a slog.Attr) slog.Attr {
	if prefix != "" {
		a.Key = prefix + "." + a.Key
	}
	return a
}

func writeAttr(b *strings.Builder, a slog.Attr) {
	v := a.Value.Resolve()
	if v.Kind() == slog.KindGroup {
		for _, g := range v.Group() {
			if a.Key != "" {
				g.Key = a.Key + "." + g.Key
			}
			writeAttr(b, g)
		}
		return
	}
	if a.Equal(slog.Attr{}) {
		return
	}
	b.WriteString(" ")
	b.WriteString(a.Key)
	b.WriteString("=")
	b.WriteString(v.String())
}
