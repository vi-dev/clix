package clix

import (
	"io"
	"slices"
	"testing"

	"github.com/spf13/cobra"
)

func TestValidateOutput(t *testing.T) {
	for _, ok := range []string{"text", "json"} {
		if err := ValidateOutput(ok); err != nil {
			t.Fatalf("ValidateOutput(%q) = %v", ok, err)
		}
	}
	err := ValidateOutput("yaml")
	want := `unknown output format "yaml" (want text or json)`
	if err == nil || err.Error() != want {
		t.Fatalf("ValidateOutput(yaml) = %v, want %s", err, want)
	}
}

func runOutputCmd(t *testing.T, args []string, pre func(*cobra.Command, []string) error, order *[]string) error {
	t.Helper()
	cmd := &cobra.Command{
		Use:           "x",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(*cobra.Command, []string) error {
			*order = append(*order, "run")
			return nil
		},
	}
	cmd.PreRunE = pre
	out := OutputFlag(cmd)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	err := cmd.Execute()
	if err == nil && *out == "" {
		t.Fatal("output value not set")
	}
	return err
}

func TestOutputFlagValidatesBeforeRun(t *testing.T) {
	var order []string
	err := runOutputCmd(t, []string{"--output", "yaml"}, nil, &order)
	want := `unknown output format "yaml" (want text or json)`
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %s", err, want)
	}
	if slices.Contains(order, "run") {
		t.Fatalf("RunE ran: %v", order)
	}
}

func TestOutputFlagRunsTheCommandsOwnPreRunAfterwards(t *testing.T) {
	var order []string
	pre := func(*cobra.Command, []string) error {
		order = append(order, "app")
		return nil
	}
	if err := runOutputCmd(t, []string{"--output", "json"}, pre, &order); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(order, []string{"app", "run"}) {
		t.Fatalf("order = %v", order)
	}
}

func TestOutputFlagWrapsPreRun(t *testing.T) {
	var order []string
	cmd := &cobra.Command{
		Use:    "x",
		PreRun: func(*cobra.Command, []string) { order = append(order, "app") },
		RunE: func(*cobra.Command, []string) error {
			order = append(order, "run")
			return nil
		},
	}
	OutputFlag(cmd)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(order, []string{"app", "run"}) {
		t.Fatalf("order = %v", order)
	}
}
