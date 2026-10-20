package consoletest

import (
	"context"
	"testing"

	"github.com/vi-dev/clix/console"
)

func TestContextCarriesTheReturnedNarrator(t *testing.T) {
	ctx, n := Context(context.Background())
	if got := console.FromContext(ctx); got != console.Narrator(n) {
		t.Fatalf("context carries %v, want the returned *Narrator", got)
	}
}
