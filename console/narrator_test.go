package console

import (
	"strings"
	"testing"
)

func TestPrefixedPrependsLinesAndLabels(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorNever})
	p := Prefixed(c, "[jq] ")

	p.Infof("x %d", 1)
	p.Warn("a", "b")
	if got, want := errb.String(), "INFO [jq] x 1\nWARN [jq] a b\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}

	errb.Reset()
	task := p.Start("Building")
	if _, err := task.ErrOut().Write([]byte("log\n")); err != nil {
		t.Fatal(err)
	}
	if got := errb.String(); !strings.Contains(got, "[jq] Building") {
		t.Errorf("stderr = %q, want label [jq] Building", got)
	}
}

func TestPrefixedPassesStreamsThrough(t *testing.T) {
	c, outb, errb := newTest(Options{Color: ColorNever})
	p := Prefixed(c, "x ")
	if _, err := p.Out().Write([]byte("data\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ErrOut().Write([]byte("diag\n")); err != nil {
		t.Fatal(err)
	}
	if outb.String() != "data\n" || errb.String() != "diag\n" {
		t.Errorf("out = %q, err = %q", outb.String(), errb.String())
	}
}
