package consoletest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/vi-dev/clix/console"
)

func TestNarratorRecordsFromManyGoroutines(t *testing.T) {
	n := &Narrator{}
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n.Infof("info %d", i)
			n.Success("success")
			n.Warn("warn")
			n.Hint("hint")
			n.Debug("debug")
			task := n.Start(fmt.Sprintf("task %d", i))
			task.Info("task info")
			task.Phase("working")
			task.Progress(1, 2, console.Items)
			fmt.Fprint(task.Out(), "out ")
			fmt.Fprint(task.ErrOut(), "err ")
			task.Done("finished")
			_ = n.Tasks()
			_ = n.Infos()
		}()
	}
	wg.Wait()
	if got := len(n.Infos()); got != 16 {
		t.Errorf("Infos = %d, want 16", got)
	}
	if got := len(n.Successes()); got != 16 {
		t.Errorf("Successes = %d, want 16", got)
	}
	if got := len(n.Warns()); got != 16 {
		t.Errorf("Warns = %d, want 16", got)
	}
	if got := len(n.Hints()); got != 16 {
		t.Errorf("Hints = %d, want 16", got)
	}
	if got := len(n.Tasks()); got != 16 {
		t.Fatalf("Tasks = %d, want 16", got)
	}
	task := n.TaskFor("task 3")
	if task == nil {
		t.Fatal("TaskFor(task 3) = nil")
	}
	if got := task.Infos(); len(got) != 1 || got[0] != "task info" {
		t.Errorf("task Infos = %q", got)
	}
	if got := task.Phases(); len(got) != 1 || got[0] != "working" {
		t.Errorf("Phases = %q", got)
	}
	if got := task.ProgressCalls(); len(got) != 1 || got[0] != (ProgressCall{Done: 1, Total: 2, Unit: console.Items}) {
		t.Errorf("ProgressCalls = %v", got)
	}
	if got := task.Output(); got != "out err " {
		t.Errorf("Output = %q, want %q", got, "out err ")
	}
	if done, failed, outcome := task.Snapshot(); !done || failed || outcome != "finished" {
		t.Errorf("Snapshot = %v %v %q", done, failed, outcome)
	}
	if n.TaskFor("missing") != nil {
		t.Error("TaskFor(missing) is not nil")
	}
}

func TestTaskEndRecordsOutcome(t *testing.T) {
	o := console.Outcomes{Done: "Built", Failed: "Build failed", Cancelled: "Build cancelled"}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	boom := errors.New("boom")
	tests := []struct {
		name        string
		ctx         context.Context
		err         error
		o           console.Outcomes
		wantDone    bool
		wantFailed  bool
		wantOutcome string
	}{
		{"success", context.Background(), nil, o, true, false, "Built"},
		{"failure", context.Background(), boom, o, false, true, "Build failed"},
		{"cancelled", cancelled, context.Canceled, o, false, true, "Build cancelled"},
		{"cancelled without text", cancelled, context.Canceled, console.Outcomes{Done: "Built", Failed: "Build failed"}, false, true, "Build failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &Narrator{}
			task := n.Start("Building")
			if got := task.End(tt.ctx, tt.err, tt.o); got != tt.err {
				t.Errorf("End returned %v, want %v", got, tt.err)
			}
			done, failed, outcome := n.TaskFor("Building").Snapshot()
			if done != tt.wantDone || failed != tt.wantFailed || outcome != tt.wantOutcome {
				t.Errorf("Snapshot = %v %v %q, want %v %v %q", done, failed, outcome, tt.wantDone, tt.wantFailed, tt.wantOutcome)
			}
		})
	}
}

func TestTaskDiscardIsRecorded(t *testing.T) {
	n := &Narrator{}
	n.Start("Checking").Discard()
	if !n.TaskFor("Checking").Discarded() {
		t.Error("Discarded = false, want true")
	}
}
