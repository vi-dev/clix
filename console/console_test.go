package console

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
)

func newTest(opts Options) (*Console, *bytes.Buffer, *bytes.Buffer) {
	var out, errb bytes.Buffer
	return New(&out, &errb, opts), &out, &errb
}

func TestLinesAndStreams(t *testing.T) {
	c, out, errb := newTest(Options{Color: ColorNever})
	c.Success("Installed go v1.26.5")
	c.Warn("Catalog dev is stale")
	c.Info("Resolving")
	c.Debug("hidden without verbose")

	if out.Len() != 0 {
		t.Fatalf("narration leaked to stdout: %q", out.String())
	}
	got := errb.String()
	for _, want := range []string{"OK Installed go v1.26.5\n", "WARN Catalog dev is stale\n", "INFO Resolving\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("stderr missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "hidden") {
		t.Errorf("debug shown without verbose")
	}
}

func TestQuietSuppressesNarrationNotDiagnostics(t *testing.T) {
	c, _, errb := newTest(Options{Quiet: true, Color: ColorNever})
	c.Info("nope")
	c.Success("nope")
	c.Warn("still here")
	got := errb.String()
	if strings.Contains(got, "nope") {
		t.Errorf("quiet did not suppress narration: %q", got)
	}
	if !strings.Contains(got, "WARN still here") {
		t.Errorf("quiet suppressed diagnostics: %q", got)
	}
}

func TestErrorCapitalizesDisplayOnly(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorNever})
	c.Error(errors.New("parse nem.toml: bad value"), "Run `nem status`")
	got := errb.String()
	if !strings.Contains(got, "ERROR Parse nem.toml: bad value\n") {
		t.Errorf("error line wrong: %q", got)
	}
	if !strings.Contains(got, "  hint: Run `nem status`\n") {
		t.Errorf("hint line wrong: %q", got)
	}
}

func TestUnicodeGlyphsWhenColored(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorAlways})
	c.Success("Done")
	if !strings.Contains(errb.String(), "✓") {
		t.Errorf("expected unicode glyph, got %q", errb.String())
	}
}

func TestHintSuppressedByQuiet(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorNever})
	c.Hint("Run `nem catalog update dev`")
	if !strings.Contains(errb.String(), "  hint: Run `nem catalog update dev`\n") {
		t.Errorf("hint line wrong: %q", errb.String())
	}

	cq, _, errbq := newTest(Options{Quiet: true, Color: ColorNever})
	cq.Hint("nope")
	if strings.Contains(errbq.String(), "nope") {
		t.Errorf("quiet did not suppress hint: %q", errbq.String())
	}
}

func TestPromptIgnoresQuiet(t *testing.T) {
	c, out, errb := newTest(Options{Quiet: true, Color: ColorNever})
	c.Prompt("Remove these? [y/N]: ")
	if errb.String() != "? Remove these? [y/N]: " {
		t.Errorf("quiet suppressed a question the command blocks on: %q", errb.String())
	}
	if out.Len() != 0 {
		t.Errorf("prompt leaked to stdout: %q", out.String())
	}
}

func TestColorAutoFollowsTTY(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorAuto, IsTTY: true})
	c.Success("On")
	if !strings.Contains(errb.String(), "✓") {
		t.Errorf("ColorAuto with TTY should color: %q", errb.String())
	}
	c2, _, errb2 := newTest(Options{Color: ColorAuto, IsTTY: false})
	c2.Success("Off")
	if !strings.Contains(errb2.String(), "OK Off") {
		t.Errorf("ColorAuto without TTY should be plain: %q", errb2.String())
	}
}

func TestNarrationIsGoroutineSafe(t *testing.T) {
	c, _, _ := newTest(Options{Color: ColorNever, Verbose: true})
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			c.Success("s")
			c.Warn("w")
			c.Debug("d")
			c.Hint("h")
		})
	}
	wg.Wait()
}

func TestStreamAccessorsAreTheConstructorsWriters(t *testing.T) {
	c, out, errb := newTest(Options{Color: ColorNever})

	if _, err := io.WriteString(c.Out(), "artifact.tar.gz\n"); err != nil {
		t.Fatalf("Out write: %v", err)
	}
	if _, err := io.WriteString(c.ErrOut(), "cc: warning\n"); err != nil {
		t.Fatalf("ErrOut write: %v", err)
	}

	if got := out.String(); got != "artifact.tar.gz\n" {
		t.Errorf("Out wrote %q to the stdout buffer, want the payload unchanged", got)
	}
	if got := errb.String(); got != "cc: warning\n" {
		t.Errorf("ErrOut wrote %q to the stderr buffer, want the payload unchanged", got)
	}
}

func TestSilentConsoleStreamsDiscard(t *testing.T) {
	c := Silent()
	n, err := io.WriteString(c.Out(), "dropped")
	if err != nil || n != len("dropped") {
		t.Fatalf("Out write = %d, %v; want a full, error-free discard", n, err)
	}
	if n, err = io.WriteString(c.ErrOut(), "dropped"); err != nil || n != len("dropped") {
		t.Fatalf("ErrOut write = %d, %v; want a full, error-free discard", n, err)
	}
}

func TestBlankErrGoesToStderrAndRespectsQuiet(t *testing.T) {
	c, out, errb := newTest(Options{Color: ColorNever})
	c.BlankErr()
	if errb.String() != "\n" || out.String() != "" {
		t.Errorf("BlankErr wrote stderr=%q stdout=%q, want a single newline on stderr", errb.String(), out.String())
	}
	cq, _, errq := newTest(Options{Quiet: true, Color: ColorNever})
	cq.BlankErr()
	if errq.String() != "" {
		t.Errorf("quiet did not suppress BlankErr: %q", errq.String())
	}
}

func TestInfoPrefixInBothModes(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorNever})
	c.Infof("Would push %s", "jq:1.8.3")
	if got, want := errb.String(), "INFO Would push jq:1.8.3\n"; got != want {
		t.Errorf("plain = %q, want %q", got, want)
	}
	cc, _, errc := newTest(Options{Color: ColorAlways})
	cc.Infof("Would push %s", "jq:1.8.3")
	if got, want := errc.String(), "\x1b[36m•\x1b[0m Would push jq:1.8.3\n"; got != want {
		t.Errorf("colored = %q, want %q", got, want)
	}
}

func TestPromptAndAnswerShareALine(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorNever})
	c.Prompt("Remove these? [y/N]: ")
	c.Answer("y")
	if got, want := errb.String(), "? Remove these? [y/N]: y\n"; got != want {
		t.Errorf("plain = %q, want %q", got, want)
	}
	cc, _, errc := newTest(Options{Color: ColorAlways})
	cc.Promptf("Remove %s? [y/N]: ", "these")
	cc.Answer("y")
	if got, want := errc.String(), "\x1b[36m?\x1b[0m Remove these? [y/N]: y\n"; got != want {
		t.Errorf("colored = %q, want %q", got, want)
	}
}

func TestAnswerIgnoresQuiet(t *testing.T) {
	c, _, errb := newTest(Options{Quiet: true, Color: ColorNever})
	c.Answer("")
	if errb.String() != "\n" {
		t.Errorf("quiet suppressed the answer line: %q", errb.String())
	}
}

func TestDebugPrefixInBothModes(t *testing.T) {
	c, _, errb := newTest(Options{Verbose: true, Color: ColorNever})
	c.Debugf("Skipping %s: no versionDiscovery", "jq")
	if got, want := errb.String(), "DEBUG Skipping jq: no versionDiscovery\n"; got != want {
		t.Errorf("plain = %q, want %q", got, want)
	}
	cc, _, errc := newTest(Options{Verbose: true, Color: ColorAlways})
	cc.Debugf("Skipping %s: no versionDiscovery", "jq")
	if got, want := errc.String(), "\x1b[2m· Skipping jq: no versionDiscovery\x1b[0m\n"; got != want {
		t.Errorf("colored = %q, want %q", got, want)
	}
}

func TestColoredGlyphsMatchNem(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorAlways})
	c.Successf("Installed %s", "jq")
	c.Warnf("Catalog %s is stale", "dev")
	c.Hintf("Run `%s`", "nem status")
	c.Error(errors.New("boom"), "Retry")
	want := "\x1b[32m✓\x1b[0m Installed jq\n" +
		"\x1b[33m!\x1b[0m Catalog dev is stale\n" +
		"  \x1b[2m→ Run `nem status`\x1b[0m\n" +
		"\x1b[31m✗ Boom\x1b[0m\n" +
		"  \x1b[2m→ Retry\x1b[0m\n"
	if got := errb.String(); got != want {
		t.Errorf("colored = %q, want %q", got, want)
	}
}

func TestUnformattedLinesUseSprintlnSpacing(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorNever})
	c.Info("Resolving", 3, "packages")
	c.Hint("Run `nem status`")
	if got, want := errb.String(), "INFO Resolving 3 packages\n  hint: Run `nem status`\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func TestQuietVerboseStillPrintsDebug(t *testing.T) {
	c, _, errb := newTest(Options{Quiet: true, Verbose: true, Color: ColorNever})
	c.Debug("shown")
	c.Info("hidden")
	if got, want := errb.String(), "DEBUG shown\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func TestMultiLineErrorIndentsContinuationLines(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorNever})
	c.Error(errors.New("first\nsecond"), "")
	if got, want := errb.String(), "ERROR First\n      second\n"; got != want {
		t.Errorf("plain = %q, want %q", got, want)
	}
	cc, _, errc := newTest(Options{Color: ColorAlways})
	cc.Error(errors.New("first\nsecond"), "")
	if got, want := errc.String(), "\x1b[31m✗ First\x1b[0m\n\x1b[31m  second\x1b[0m\n"; got != want {
		t.Errorf("colored = %q, want %q", got, want)
	}
	ct, _, errt := newTest(Options{Color: ColorNever})
	ct.Error(errors.New("first\nsecond\n"), "")
	if got, want := errt.String(), "ERROR First\n      second\n"; got != want {
		t.Errorf("trailing newline = %q, want %q", got, want)
	}
}

type failingWriter struct{ t *testing.T }

func (w failingWriter) Write(p []byte) (int, error) {
	w.t.Errorf("unexpected write %q", p)
	return len(p), nil
}

func TestNewDoesNoIO(t *testing.T) {
	before := settledGoroutines()
	for _, opts := range []Options{
		{},
		{IsTTY: true, OutIsTTY: true},
		{Narration: NarrationPlain, Color: ColorAlways},
		{Narration: NarrationJSON},
	} {
		New(failingWriter{t}, failingWriter{t}, opts)
	}
	if after := settledGoroutines(); after != before {
		t.Errorf("goroutines = %d after New, want %d", after, before)
	}
}

func TestNothingNarratedAfterExit(t *testing.T) {
	c, _, errb := newTest(Options{Color: ColorNever})
	c.Info("before")
	c.Exit(0, nil)
	c.Info("after")
	c.Warn("after")
	c.Error(errors.New("after"), "")
	if got, want := errb.String(), "INFO before\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}
