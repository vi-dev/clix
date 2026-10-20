package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/vi-dev/clix"
	"github.com/vi-dev/clix/console"
)

var con *console.Console

var errNoSuchPerson = errors.New("no such person")

func main() {
	clix.Run(newRoot(), clix.Options{
		App:       "hello",
		Version:   "0.1.0",
		EnvPrefix: "HELLO",
		Hint:      hint,
		Setup: func(ctx context.Context, c *console.Console) (context.Context, error) {
			con = c
			return ctx, nil
		},
	})
}

func newRoot() *cobra.Command {
	root := &cobra.Command{Use: "hello", Short: "Greet people"}
	root.AddCommand(&cobra.Command{
		Use:   "greet <name>",
		Short: "Greet someone by name",
		Args:  cobra.ExactArgs(1),
		RunE:  greet,
	})
	return root
}

func greet(cmd *cobra.Command, args []string) error {
	name := args[0]
	if name == "nobody" {
		return greetError(name)
	}
	con.Infof("Hello, %s", name)
	return greetTask(cmd.Context(), name)
}

func greetTask(ctx context.Context, name string) error {
	t := con.Start("Greeting " + name)
	t.Phase("thinking")
	o := console.Outcomes{Done: "Greeted " + name, Failed: "Failed to greet " + name, Cancelled: "Cancelled greeting " + name}
	var err error
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case <-time.After(300 * time.Millisecond):
	}
	return t.End(ctx, err, o)
}

func greetError(name string) error {
	return fmt.Errorf("%w: %s", errNoSuchPerson, name)
}

func hint(err error) string {
	if errors.Is(err, errNoSuchPerson) {
		return "Try `hello greet world`"
	}
	return ""
}
