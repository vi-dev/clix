package clix

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vi-dev/clix/console"
)

type flagValues struct {
	quiet     bool
	verbose   bool
	color     string
	narration string
}

func addFlags(root *cobra.Command, f *flagValues) {
	pf := root.PersistentFlags()
	pf.BoolVarP(&f.quiet, "quiet", "q", false, "suppress narration output")
	pf.BoolVar(&f.verbose, "verbose", false, "show debug output")
	pf.StringVar(&f.color, "color", "auto", "colorize output: auto, always, or never")
	pf.StringVar(&f.narration, "narration", "auto", "narration format: auto, plain, or json")
	_ = root.RegisterFlagCompletionFunc("color", cobra.FixedCompletions([]string{"auto", "always", "never"}, cobra.ShellCompDirectiveNoFileComp))
	_ = root.RegisterFlagCompletionFunc("narration", cobra.FixedCompletions([]string{"auto", "plain", "json"}, cobra.ShellCompDirectiveNoFileComp))
}

func resolveColor(flag string, noColor, forceColor bool) (console.ColorMode, error) {
	switch flag {
	case "always":
		return console.ColorAlways, nil
	case "never":
		return console.ColorNever, nil
	case "auto":
		if noColor {
			return console.ColorNever, nil
		}
		if forceColor {
			return console.ColorAlways, nil
		}
		return console.ColorAuto, nil
	}
	return console.ColorAuto, fmt.Errorf("invalid --color value %q (want auto, always, or never)", flag)
}

func parseNarration(value string) (console.Narration, bool) {
	switch value {
	case "auto":
		return console.NarrationAuto, true
	case "plain":
		return console.NarrationPlain, true
	case "json":
		return console.NarrationJSON, true
	}
	return console.NarrationAuto, false
}

func resolveNarration(flag, envName, envValue string) (console.Narration, error) {
	if flag != "auto" {
		n, ok := parseNarration(flag)
		if !ok {
			return console.NarrationAuto, fmt.Errorf("invalid --narration value %q (want auto, plain, or json)", flag)
		}
		return n, nil
	}
	if envValue == "" {
		return console.NarrationAuto, nil
	}
	n, ok := parseNarration(envValue)
	if !ok {
		return console.NarrationAuto, fmt.Errorf("invalid %s value %q (want auto, plain, or json)", envName, envValue)
	}
	return n, nil
}

func wantsJSON(args []string, envValue string) bool {
	seen := false
	for i, a := range args {
		if a == "--" {
			break
		}
		if v, ok := strings.CutPrefix(a, "--narration="); ok {
			if v == "json" {
				return true
			}
			if v != "auto" {
				seen = true
			}
			continue
		}
		if a == "--narration" {
			if i+1 >= len(args) {
				seen = true
				continue
			}
			if v := args[i+1]; v == "json" {
				return true
			} else if v != "auto" {
				seen = true
			}
		}
	}
	return !seen && envValue == "json"
}
