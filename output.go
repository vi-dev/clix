package clix

import (
	"fmt"

	"github.com/spf13/cobra"
)

const (
	outputText = "text"
	outputJSON = "json"
)

// AddOutputFlag adds the --output flag to cmd and stores its value in output. The default is text.
func AddOutputFlag(cmd *cobra.Command, output *string) {
	cmd.Flags().StringVar(output, "output", outputText, "output format: text or json")
	_ = cmd.RegisterFlagCompletionFunc("output",
		cobra.FixedCompletions([]string{outputText, outputJSON}, cobra.ShellCompDirectiveNoFileComp))
}

// ValidateOutput returns an error unless output is text or json.
func ValidateOutput(output string) error {
	if output != outputText && output != outputJSON {
		return fmt.Errorf("unknown output format %q (want %s or %s)", output, outputText, outputJSON)
	}
	return nil
}

// OutputFlag adds the --output flag to cmd, rejects any value other than text or json before the command runs, and returns the value.
func OutputFlag(cmd *cobra.Command) *string {
	output := new(string)
	AddOutputFlag(cmd, output)
	previousE := cmd.PreRunE
	previous := cmd.PreRun
	cmd.PreRun = nil
	cmd.PreRunE = func(c *cobra.Command, args []string) error {
		if err := ValidateOutput(*output); err != nil {
			return err
		}
		if previousE != nil {
			return previousE(c, args)
		}
		if previous != nil {
			previous(c, args)
		}
		return nil
	}
	return output
}
