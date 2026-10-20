package clix_test

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/vi-dev/clix"
	"github.com/vi-dev/clix/console"
)

func Example() {
	root := &cobra.Command{
		Use: "demo",
		RunE: func(cmd *cobra.Command, args []string) error {
			clix.ConsoleOf(cmd).Infof("hello")
			return nil
		},
	}
	clix.Run(root, clix.Options{
		App:       "demo",
		Version:   "0.1.0",
		EnvPrefix: "DEMO",
		Setup: func(ctx context.Context, c *console.Console) (context.Context, error) {
			return ctx, nil
		},
	})
}
