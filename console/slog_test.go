package console

import (
	"log/slog"
	"testing"
)

func TestSlogHandlerMapsLevels(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorNever, Verbose: true})
	l := slog.New(SlogHandler(c))
	l.Info("msg", "k", "v")
	l.Warn("msg")
	l.Error("msg")
	want := "DEBUG msg k=v\nWARN msg\nWARN msg\n"
	if got := errb.String(); got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func TestSlogHandlerEnabledFollowsVerbose(t *testing.T) {
	c, _, _ := newTest(Options{Color: ColorNever})
	h := SlogHandler(c)
	if h.Enabled(t.Context(), slog.LevelInfo) {
		t.Error("Info enabled on a non-verbose console")
	}
	if !h.Enabled(t.Context(), slog.LevelWarn) {
		t.Error("Warn disabled")
	}
	v, _, _ := newTest(Options{Color: ColorNever, Verbose: true})
	if !SlogHandler(v).Enabled(t.Context(), slog.LevelDebug) {
		t.Error("Debug disabled on a verbose console")
	}
}

func TestSlogHandlerGroupsFlattenWithDots(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorNever, Verbose: true})
	l := slog.New(SlogHandler(c)).WithGroup("http").With("status", 200)
	l.Info("fetched")
	l.Info("again", slog.Group("hdr", "n", 1))
	want := "DEBUG fetched http.status=200\nDEBUG again http.status=200 http.hdr.n=1\n"
	if got := errb.String(); got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}
