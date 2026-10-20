package console

import (
	"context"
	"errors"
)

// IsCancellation reports whether err comes from a cancelled or timed-out context.
func IsCancellation(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// Cancelled reports whether err is a cancellation or ctx is already done.
func Cancelled(ctx context.Context, err error) bool {
	return IsCancellation(err) || ctx.Err() != nil
}
