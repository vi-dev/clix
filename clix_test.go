package clix

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/spf13/cobra"

	"github.com/vi-dev/clix/console"
)

type harness struct {
	root     *cobra.Command
	out, err *bytes.Buffer
}

func newHarness(t *testing.T, run func(cmd *cobra.Command, args []string) error, args ...string) *harness {
	t.Helper()
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("T_NARRATION", "")
	h := &harness{out: new(bytes.Buffer), err: new(bytes.Buffer)}
	h.root = &cobra.Command{Use: "tool", Version: "1.2.3", RunE: run}
	h.root.SetOut(h.out)
	h.root.SetErr(h.err)
	h.root.SetArgs(args)
	return h
}

func (h *harness) execute(ctx context.Context, opts Options) (int, error) {
	return execute(ctx, h.root, opts, nil, func() int { return 80 })
}

func ok(*cobra.Command, []string) error { return nil }

func events(t *testing.T, b *bytes.Buffer) []console.Event {
	t.Helper()
	evs, err := console.ReadEvents(b)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(evs) == 0 {
		t.Fatal("no events")
	}
	return evs
}

func TestExecuteReturnsZeroAndPrintsNothingOnSuccess(t *testing.T) {
	h := newHarness(t, ok)
	code, err := h.execute(t.Context(), Options{})
	if code != 0 || err != nil {
		t.Fatalf("code, err = %d, %v; want 0, nil", code, err)
	}
	if h.out.Len() != 0 || h.err.Len() != 0 {
		t.Fatalf("stdout %q stderr %q; want both empty", h.out, h.err)
	}
}

func TestExecuteExitErrorCodeIsSilent(t *testing.T) {
	h := newHarness(t, func(*cobra.Command, []string) error { return &ExitError{Code: 3} })
	code, err := h.execute(t.Context(), Options{})
	if code != 3 {
		t.Fatalf("code = %d, want 3", code)
	}
	if err == nil || err.Error() != "exit status 3" {
		t.Fatalf("err = %v, want exit status 3", err)
	}
	if h.err.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", h.err)
	}
}

func TestExecuteErrorAfterConsoleRendersWithHint(t *testing.T) {
	h := newHarness(t, func(*cobra.Command, []string) error { return errors.New("boom") })
	code, _ := h.execute(t.Context(), Options{Hint: func(error) string { return "Run `x`" }})
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if got, want := h.err.String(), "ERROR Boom\n  hint: Run `x`\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestExecuteFlagErrorBeforeConsoleExitsTwo(t *testing.T) {
	h := newHarness(t, ok, "--color", "bogus")
	code, err := h.execute(t.Context(), Options{})
	if code != 2 || err == nil {
		t.Fatalf("code, err = %d, %v; want 2 and an error", code, err)
	}
	if got, want := h.err.String(), "Error: invalid --color value \"bogus\" (want auto, always, or never)\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestExecutePreConsoleErrorInJSONWhenEnvAsks(t *testing.T) {
	h := newHarness(t, ok, "--color", "bogus")
	t.Setenv("T_NARRATION", "json")
	code, _ := h.execute(t.Context(), Options{EnvPrefix: "T"})
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	evs := events(t, h.err)
	if len(evs) != 2 {
		t.Fatalf("events = %+v, want two", evs)
	}
	if e := evs[0]; e.Event != "line" || e.Level != "error" || e.Text != "Invalid --color value \"bogus\" (want auto, always, or never)" {
		t.Fatalf("first event = %+v, want error line", e)
	}
	if e := evs[1]; e.Event != "exit" || e.Code != 2 {
		t.Fatalf("last event = %+v, want exit code 2", e)
	}
}

func TestExecuteSignalCauseExits130AndPrintsNonCancellationErrors(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	h := newHarness(t, func(*cobra.Command, []string) error {
		cancel(SignalError{Sig: syscall.SIGINT})
		return errors.New("checksum mismatch")
	})
	code, _ := h.execute(ctx, Options{})
	if code != 130 {
		t.Fatalf("code = %d, want 130", code)
	}
	if !strings.Contains(h.err.String(), "ERROR Checksum mismatch") {
		t.Fatalf("stderr = %q, want the error", h.err)
	}
}

func TestExecuteSignalCauseExits143ForSIGTERMSilentlyOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	h := newHarness(t, func(cmd *cobra.Command, _ []string) error {
		cancel(SignalError{Sig: syscall.SIGTERM})
		return cmd.Context().Err()
	})
	code, _ := h.execute(ctx, Options{})
	if code != 143 {
		t.Fatalf("code = %d, want 143", code)
	}
	if h.err.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", h.err)
	}
}

func TestExecuteCancelledWithoutSignalIsAnOrdinaryError(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	h := newHarness(t, func(*cobra.Command, []string) error {
		cancel()
		return fmt.Errorf("fetch: %w", context.Canceled)
	})
	code, _ := h.execute(ctx, Options{})
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if got, want := h.err.String(), "ERROR Fetch: context canceled\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestExecuteWrapsExistingRootHookClixFirst(t *testing.T) {
	var order []string
	h := newHarness(t, func(*cobra.Command, []string) error {
		order = append(order, "run")
		return nil
	})
	h.root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		if ConsoleOf(cmd) == nil {
			t.Error("ConsoleOf returned nil in the app hook")
		}
		order = append(order, "app hook")
		return nil
	}
	opts := Options{Setup: func(ctx context.Context, _ *console.Console) (context.Context, error) {
		order = append(order, "setup")
		return ctx, nil
	}}
	if code, err := h.execute(t.Context(), opts); code != 0 {
		t.Fatalf("code, err = %d, %v; want 0", code, err)
	}
	if want := []string{"setup", "app hook", "run"}; !slices.Equal(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
}

func TestExecuteTraverseRunHooksKeepsConsoleForSubcommandWithOwnHook(t *testing.T) {
	h := newHarness(t, nil, "sub")
	var got *console.Console
	sub := &cobra.Command{
		Use: "sub",
		PersistentPreRunE: func(*cobra.Command, []string) error {
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			got = ConsoleOf(cmd)
			return nil
		},
	}
	h.root.AddCommand(sub)
	if code, err := h.execute(t.Context(), Options{}); code != 0 {
		t.Fatalf("code, err = %d, %v; want 0", code, err)
	}
	if got == nil {
		t.Fatal("subcommand saw no console")
	}
}

type setupKey struct{}

func TestExecuteSetupReplacesContextAndCanFail(t *testing.T) {
	t.Run("replaces", func(t *testing.T) {
		var seen any
		h := newHarness(t, func(cmd *cobra.Command, _ []string) error {
			seen = cmd.Context().Value(setupKey{})
			return nil
		})
		opts := Options{Setup: func(ctx context.Context, _ *console.Console) (context.Context, error) {
			return context.WithValue(ctx, setupKey{}, "ready"), nil
		}}
		if code, err := h.execute(t.Context(), opts); code != 0 {
			t.Fatalf("code, err = %d, %v; want 0", code, err)
		}
		if seen != "ready" {
			t.Fatalf("RunE saw %v, want ready", seen)
		}
	})
	t.Run("fails", func(t *testing.T) {
		ran := false
		h := newHarness(t, func(*cobra.Command, []string) error {
			ran = true
			return nil
		})
		opts := Options{Setup: func(ctx context.Context, _ *console.Console) (context.Context, error) {
			return ctx, errors.New("config broken")
		}}
		code, _ := h.execute(t.Context(), opts)
		if code != 1 {
			t.Fatalf("code = %d, want 1", code)
		}
		if ran {
			t.Fatal("RunE ran after Setup failed")
		}
		if got, want := h.err.String(), "ERROR Config broken\n"; got != want {
			t.Fatalf("stderr = %q, want %q", got, want)
		}
	})
}

func TestExecuteBeginAndExitFrameTheJSONStream(t *testing.T) {
	h := newHarness(t, func(cmd *cobra.Command, _ []string) error {
		ConsoleOf(cmd).Info("working")
		return nil
	}, "--narration", "json")
	if code, err := h.execute(t.Context(), Options{}); code != 0 {
		t.Fatalf("code, err = %d, %v; want 0", code, err)
	}
	evs := events(t, h.err)
	if e := evs[0]; e.Event != "begin" || e.App != "tool" || e.Version != "1.2.3" {
		t.Fatalf("first event = %+v, want begin tool 1.2.3", e)
	}
	if e := evs[len(evs)-1]; e.Event != "exit" || e.Code != 0 {
		t.Fatalf("last event = %+v, want exit code 0", e)
	}
}

func TestExecuteNarrationEnvDefaultLosesToFlag(t *testing.T) {
	h := newHarness(t, func(cmd *cobra.Command, _ []string) error {
		ConsoleOf(cmd).Warn("careful")
		return nil
	}, "--narration", "plain")
	t.Setenv("T_NARRATION", "json")
	if code, err := h.execute(t.Context(), Options{EnvPrefix: "T"}); code != 0 {
		t.Fatalf("code, err = %d, %v; want 0", code, err)
	}
	if got, want := h.err.String(), "WARN careful\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestExecuteRecoversPanicEmitsExitTwoAndRepanics(t *testing.T) {
	h := newHarness(t, func(*cobra.Command, []string) error { panic("kaboom") }, "--narration", "json")
	func() {
		defer func() {
			if r := recover(); r != "kaboom" {
				t.Fatalf("recovered %v, want kaboom", r)
			}
		}()
		_, _ = h.execute(t.Context(), Options{})
		t.Fatal("execute returned instead of panicking")
	}()
	evs := events(t, h.err)
	if e := evs[len(evs)-1]; e.Event != "exit" || e.Code != 2 {
		t.Fatalf("last event = %+v, want exit code 2", e)
	}
}

func TestConsoleOfPanicsWithoutHook(t *testing.T) {
	cmd := &cobra.Command{Use: "tool"}
	cmd.SetContext(t.Context())
	defer func() {
		want := "clix: no console on the command; run the command through clix.Run or clix.Execute"
		if r := recover(); r != want {
			t.Fatalf("panic = %v, want %q", r, want)
		}
	}()
	ConsoleOf(cmd)
}

func TestConsoleOfPanicsWithoutContext(t *testing.T) {
	defer func() {
		want := "clix: no console on the command; run the command through clix.Run or clix.Execute"
		if r := recover(); r != want {
			t.Fatalf("panic = %v, want %q", r, want)
		}
	}()
	ConsoleOf(&cobra.Command{Use: "tool"})
}

func TestIsTerminalIsFalseForBuffersAndPipes(t *testing.T) {
	if isTerminal(&bytes.Buffer{}) {
		t.Fatal("a buffer is a terminal")
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if isTerminal(w) {
		t.Fatal("a pipe is a terminal")
	}
}

func TestExecuteAppAndVersionOverrideTheRoot(t *testing.T) {
	h := newHarness(t, ok, "--narration", "json")
	if code, err := h.execute(t.Context(), Options{App: "other", Version: "9.9.9"}); code != 0 {
		t.Fatalf("code, err = %d, %v; want 0", code, err)
	}
	if e := events(t, h.err)[0]; e.Event != "begin" || e.App != "other" || e.Version != "9.9.9" {
		t.Fatalf("first event = %+v, want begin other 9.9.9", e)
	}
}

func TestConsoleOfSurvivesSetupReplacingTheNarrator(t *testing.T) {
	h := newHarness(t, func(cmd *cobra.Command, _ []string) error {
		ConsoleOf(cmd).Infof("x")
		console.FromContext(cmd.Context()).Infof("y")
		return nil
	})
	opts := Options{Setup: func(ctx context.Context, c *console.Console) (context.Context, error) {
		return console.NewContext(ctx, console.Prefixed(c, "[dev] ")), nil
	}}
	if code, err := h.execute(t.Context(), opts); code != 0 {
		t.Fatalf("code, err = %d, %v; want 0", code, err)
	}
	if got, want := h.err.String(), "INFO x\nINFO [dev] y\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestExecutePreConsoleErrorInJSONWhenFlagAsks(t *testing.T) {
	h := newHarness(t, ok, "--narration", "json", "--color", "bogus")
	code, _ := h.execute(t.Context(), Options{EnvPrefix: "T"})
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	evs := events(t, h.err)
	if len(evs) != 2 {
		t.Fatalf("events = %+v, want two", evs)
	}
	if e := evs[0]; e.Event != "line" || e.Level != "error" || e.Text != "Invalid --color value \"bogus\" (want auto, always, or never)" {
		t.Fatalf("first event = %+v, want error line", e)
	}
	if e := evs[1]; e.Event != "exit" || e.Code != 2 {
		t.Fatalf("last event = %+v, want exit code 2", e)
	}
}

func TestExecutePreConsoleErrorStaysPlainWhenFlagIsPlainDespiteEnv(t *testing.T) {
	h := newHarness(t, ok, "--narration", "plain", "--color", "bogus")
	t.Setenv("T_NARRATION", "json")
	code, _ := h.execute(t.Context(), Options{EnvPrefix: "T"})
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if got, want := h.err.String(), "Error: invalid --color value \"bogus\" (want auto, always, or never)\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}
