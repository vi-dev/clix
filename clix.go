package clix

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync/atomic"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/vi-dev/clix/console"
)

// Options configures Execute and Run.
type Options struct {
	// App names the program in JSON narration; empty means the root command's name.
	App string
	// Version is the program version in JSON narration; empty means the root command's Version.
	Version string
	// EnvPrefix makes <EnvPrefix>_NARRATION choose the narration format; empty means no variable is read.
	EnvPrefix string
	// Hint returns the hint shown under the final error, or an empty string for none; nil means no hints.
	Hint func(error) string
	// Setup runs once the console is ready and returns the context the command runs with; an error from it ends the run like an error from the command.
	Setup func(ctx context.Context, c *console.Console) (context.Context, error)
}

// ExitError ends the run with exit code Code and no error line.
type ExitError struct{ Code int }

// Error reports the exit code.
func (e *ExitError) Error() string { return fmt.Sprintf("exit status %d", e.Code) }

// Execute runs root like Run but returns the exit code and the command's error instead of exiting, and does not handle signals.
func Execute(ctx context.Context, root *cobra.Command, opts Options) (int, error) {
	return execute(ctx, root, opts, nil, new(widthCache).get)
}

// Run executes root with the global flags, the console and signal handling, then exits the process with the run's exit code.
func Run(root *cobra.Command, opts Options) {
	ctx, cancel := context.WithCancelCause(context.Background())
	var held atomic.Pointer[console.Console]
	w := new(widthCache)
	watchResize(w.reset)
	installSignalWatcher(cancel, func(code int, sig os.Signal) {
		if c := held.Load(); c != nil {
			c.Exit(code, sig)
		}
		os.Exit(code)
	})
	code, _ := execute(ctx, root, opts, held.Store, w.get)
	cancel(nil)
	os.Exit(code)
}

type consoleKey struct{}

// ConsoleOf returns the console of a command run through Run or Execute, and panics for any other command.
func ConsoleOf(cmd *cobra.Command) *console.Console {
	if ctx := cmd.Context(); ctx != nil {
		if c, ok := ctx.Value(consoleKey{}).(*console.Console); ok && c != nil {
			return c
		}
	}
	panic("clix: no console on the command; run the command through clix.Run or clix.Execute")
}

func execute(ctx context.Context, root *cobra.Command, opts Options, hold func(*console.Console), width func() int) (int, error) {
	cobra.EnableTraverseRunHooks = true
	root.SilenceUsage = true
	root.SilenceErrors = true

	var c *console.Console
	defer func() {
		if r := recover(); r != nil {
			if c != nil {
				c.Exit(2, nil)
			}
			panic(r)
		}
	}()

	envName := ""
	if opts.EnvPrefix != "" {
		envName = opts.EnvPrefix + "_NARRATION"
	}
	envValue := func() string {
		if envName == "" {
			return ""
		}
		return os.Getenv(envName)
	}

	f := new(flagValues)
	addFlags(root, f)
	previous := root.PersistentPreRunE
	hookRan := false
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		hookRan = true
		color, err := resolveColor(f.color, os.Getenv("NO_COLOR") != "", os.Getenv("FORCE_COLOR") != "")
		if err != nil {
			return err
		}
		narration, err := resolveNarration(f.narration, envName, envValue())
		if err != nil {
			return err
		}
		con := console.New(cmd.OutOrStdout(), cmd.ErrOrStderr(), console.Options{
			Quiet:     f.quiet,
			Verbose:   f.verbose,
			Color:     color,
			Narration: narration,
			IsTTY:     isTerminal(cmd.ErrOrStderr()),
			OutIsTTY:  isTerminal(cmd.OutOrStdout()),
			Width:     width,
		})
		c = con
		app, version := opts.App, opts.Version
		if app == "" {
			app = root.Name()
		}
		if version == "" {
			version = root.Version
		}
		con.Begin(app, version)
		if hold != nil {
			hold(con)
		}
		hookCtx := context.WithValue(console.NewContext(cmd.Context(), con), consoleKey{}, con)
		if opts.Setup != nil {
			hookCtx, err = opts.Setup(hookCtx, con)
			if err != nil {
				return err
			}
		}
		cmd.SetContext(hookCtx)
		if previous != nil {
			return previous(cmd, args)
		}
		return nil
	}

	err := root.ExecuteContext(ctx)
	_, silent := errors.AsType[*ExitError](err)
	if err != nil && !silent && c == nil {
		if preConsoleJSON(hookRan, f.narration, envName, envValue()) {
			jc := console.New(io.Discard, root.ErrOrStderr(), console.Options{Narration: console.NarrationJSON})
			jc.Error(err, "")
			jc.Exit(2, nil)
		} else {
			fmt.Fprintln(root.ErrOrStderr(), "Error:", err) //nolint:forbidigo // no console exists yet
		}
		return 2, err
	}
	code, sig := exitCode(ctx, err)
	if err != nil && !silent && (sig == nil || !console.IsCancellation(err)) {
		hint := ""
		if opts.Hint != nil {
			hint = opts.Hint(err)
		}
		c.Error(err, hint)
	}
	if c != nil {
		c.Exit(code, sig)
	}
	return code, err
}

func preConsoleJSON(hookRan bool, flag, envName, envValue string) bool {
	if !hookRan {
		return wantsJSON(os.Args[1:], envValue)
	}
	n, _ := resolveNarration(flag, envName, envValue)
	return n == console.NarrationJSON
}

func exitCode(ctx context.Context, err error) (int, os.Signal) {
	if err == nil {
		return 0, nil
	}
	if exitErr, ok := errors.AsType[*ExitError](err); ok {
		return exitErr.Code, nil
	}
	var se SignalError
	if errors.As(context.Cause(ctx), &se) {
		return exitCodeFor(se.Sig), se.Sig
	}
	return 1, nil
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}
