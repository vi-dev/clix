package console

import (
	"context"
	"testing"
)

func TestFromContextReturnsAttachedNarrator(t *testing.T) {
	c := Silent()
	ctx := NewContext(context.Background(), c)
	if got := FromContext(ctx); got != Narrator(c) {
		t.Fatalf("FromContext returned %v, want the attached narrator", got)
	}
}

func TestFromContextWithoutAttachmentIsSilentNotNil(t *testing.T) {
	n := FromContext(context.Background())
	if n == nil {
		t.Fatal("FromContext returned nil for a bare context")
	}
	n.Info("must not panic")
	n.Infof("must not %s", "panic")
	n.Start("must not panic").Done("ok")
}
