package clix

import (
	"errors"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestSignalErrorWithoutSignal(t *testing.T) {
	if got := (SignalError{}).Error(); got != "received signal" {
		t.Fatalf("Error() = %q, want %q", got, "received signal")
	}
}

func TestWatchSignalsCancelsOnFirstAndExitsOnSecond(t *testing.T) {
	sig := make(chan os.Signal, 2)
	causes := make(chan error, 1)
	cancel := func(cause error) { causes <- cause }
	type exitCall struct {
		code int
		sig  os.Signal
	}
	exited := make(chan exitCall, 1)
	exit := func(code int, s os.Signal) { exited <- exitCall{code, s} }

	done := make(chan struct{})
	go func() {
		watchSignals(sig, cancel, exit)
		close(done)
	}()

	sig <- os.Interrupt

	select {
	case cause := <-causes:
		var got SignalError
		if !errors.As(cause, &got) || got.Sig != os.Interrupt {
			t.Fatalf("cause = %v, want SignalError for interrupt", cause)
		}
		if cause.Error() != "received interrupt" {
			t.Fatalf("cause text = %q", cause.Error())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancel was not called after the first signal")
	}

	select {
	case call := <-exited:
		t.Fatalf("exit called after only one signal, code %d", call.code)
	case <-time.After(20 * time.Millisecond):
	}

	sig <- os.Interrupt

	select {
	case call := <-exited:
		if call.code != 130 || call.sig != os.Interrupt {
			t.Fatalf("exit = %d %v, want 130 interrupt", call.code, call.sig)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("exit was not called after the second signal")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchSignals did not return after exit")
	}
}

func TestWatchSignalsSIGTERMExits143(t *testing.T) {
	sig := make(chan os.Signal, 2)
	exited := make(chan int, 1)
	done := make(chan struct{})
	go func() {
		watchSignals(sig, func(error) {}, func(code int, _ os.Signal) { exited <- code })
		close(done)
	}()
	sig <- syscall.SIGTERM
	sig <- syscall.SIGTERM
	select {
	case code := <-exited:
		if code != 143 {
			t.Fatalf("exit code = %d, want 143", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("exit was not called")
	}
	<-done
}

func TestWatchSignalsReturnsOnClosedChannel(t *testing.T) {
	sig := make(chan os.Signal)
	close(sig)
	done := make(chan struct{})
	go func() {
		watchSignals(sig, func(error) { t.Error("cancel called") }, func(int, os.Signal) { t.Error("exit called") })
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchSignals did not return on a closed channel")
	}
}

func TestExitCodeFor(t *testing.T) {
	cases := []struct {
		sig  os.Signal
		want int
	}{
		{syscall.SIGINT, 130},
		{syscall.SIGTERM, 143},
		{syscall.SIGHUP, 1},
	}
	for _, c := range cases {
		if got := exitCodeFor(c.sig); got != c.want {
			t.Errorf("exitCodeFor(%v) = %d, want %d", c.sig, got, c.want)
		}
	}
}

func TestWatchResizeRefreshesOnSIGWINCH(t *testing.T) {
	refreshed := make(chan struct{}, 4)
	watchResize(func() { refreshed <- struct{}{} })
	if err := syscall.Kill(os.Getpid(), syscall.SIGWINCH); err != nil {
		t.Fatal(err)
	}
	select {
	case <-refreshed:
	case <-time.After(2 * time.Second):
		t.Fatal("refresh was not called after SIGWINCH")
	}
}
