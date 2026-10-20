package console

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestIsCancellation(t *testing.T) {
	if !IsCancellation(context.Canceled) {
		t.Error("context.Canceled must be classified as cancellation")
	}
	if !IsCancellation(fmt.Errorf("wrap: %w", context.DeadlineExceeded)) {
		t.Error("wrapped context.DeadlineExceeded must be classified as cancellation")
	}
	if IsCancellation(errors.New("real failure")) {
		t.Error("an unrelated error must not be classified as cancellation")
	}
	if IsCancellation(nil) {
		t.Error("a nil error must not be classified as cancellation")
	}
}

func TestCancelled(t *testing.T) {
	done, cancel := context.WithCancel(context.Background())
	cancel()
	cases := []struct {
		name string
		ctx  context.Context
		err  error
		want bool
	}{
		{"live nil", context.Background(), nil, false},
		{"live canceled error", context.Background(), context.Canceled, true},
		{"done nil", done, nil, true},
		{"done other error", done, errors.New("x"), true},
		{"live wrapped deadline", context.Background(), fmt.Errorf("wrap: %w", context.DeadlineExceeded), true},
	}
	for _, c := range cases {
		if got := Cancelled(c.ctx, c.err); got != c.want {
			t.Errorf("%s: Cancelled = %v, want %v", c.name, got, c.want)
		}
	}
}
