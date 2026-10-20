package consoletest

import (
	"context"

	"github.com/vi-dev/clix/console"
)

// Context returns a copy of ctx that carries a new recording Narrator, and that Narrator.
func Context(ctx context.Context) (context.Context, *Narrator) {
	n := &Narrator{}
	return console.NewContext(ctx, n), n
}
