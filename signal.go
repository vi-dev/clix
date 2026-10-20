package clix

import (
	"os"
	"os/signal"
	"syscall"
)

// SignalError is the cause of the context's cancellation when Run receives a signal, with Sig naming that signal.
type SignalError struct{ Sig os.Signal }

// Error names the signal received.
func (e SignalError) Error() string {
	if e.Sig == nil {
		return "received signal"
	}
	return "received " + e.Sig.String()
}

func exitCodeFor(sig os.Signal) int {
	switch sig {
	case syscall.SIGINT:
		return 130
	case syscall.SIGTERM:
		return 143
	}
	return 1
}

func watchSignals(sig <-chan os.Signal, cancel func(cause error), exit func(code int, sig os.Signal)) {
	first, ok := <-sig
	if !ok {
		return
	}
	cancel(SignalError{Sig: first})
	second, ok := <-sig
	if !ok {
		return
	}
	exit(exitCodeFor(second), second)
}

func installSignalWatcher(cancel func(error), exit func(int, os.Signal)) {
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go watchSignals(sig, cancel, exit)
}

func watchResize(refresh func()) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGWINCH)
	go func() {
		for range sig {
			refresh()
		}
	}()
}
