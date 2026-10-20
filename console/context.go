package console

import "context"

type contextKey struct{}

var silent Narrator = Silent()

// NewContext returns a copy of ctx that carries n.
func NewContext(ctx context.Context, n Narrator) context.Context {
	return context.WithValue(ctx, contextKey{}, n)
}

// FromContext returns the narrator in ctx. Without one it returns a narrator that prints nothing.
func FromContext(ctx context.Context) Narrator {
	if n, ok := ctx.Value(contextKey{}).(Narrator); ok {
		return n
	}
	return silent
}
