package clix

import (
	"slices"
	"testing"

	"github.com/spf13/cobra"

	"github.com/vi-dev/clix/console"
)

func TestResolveColor(t *testing.T) {
	tests := []struct {
		name    string
		flag    string
		noColor bool
		force   bool
		want    console.ColorMode
		wantErr string
	}{
		{"auto plain", "auto", false, false, console.ColorAuto, ""},
		{"auto no color and force", "auto", true, true, console.ColorNever, ""},
		{"auto force", "auto", false, true, console.ColorAlways, ""},
		{"never beats force", "never", false, true, console.ColorNever, ""},
		{"always beats no color", "always", true, false, console.ColorAlways, ""},
		{"bogus", "bogus", false, false, console.ColorAuto, `invalid --color value "bogus" (want auto, always, or never)`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveColor(tt.flag, tt.noColor, tt.force)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveNarration(t *testing.T) {
	tests := []struct {
		name    string
		flag    string
		env     string
		want    console.Narration
		wantErr string
	}{
		{"auto empty env", "auto", "", console.NarrationAuto, ""},
		{"auto json env", "auto", "json", console.NarrationJSON, ""},
		{"plain beats env", "plain", "json", console.NarrationPlain, ""},
		{"bad env", "auto", "bogus", console.NarrationAuto, `invalid NEM_NARRATION value "bogus" (want auto, plain, or json)`},
		{"bad flag", "bogus", "", console.NarrationAuto, `invalid --narration value "bogus" (want auto, plain, or json)`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveNarration(tt.flag, "NEM_NARRATION", tt.env)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWantsJSON(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  string
		want bool
	}{
		{"equals form", []string{"sync", "--narration=json"}, "", true},
		{"separate form", []string{"--narration", "json"}, "", true},
		{"flag plain beats env", []string{"--narration", "plain"}, "json", false},
		{"env only", nil, "json", true},
		{"partial value", []string{"--narration=jso"}, "", false},
		{"nothing", []string{"sync"}, "", false},
		{"flag auto equals defers to env", []string{"--narration=auto"}, "json", true},
		{"flag auto separate defers to env", []string{"--narration", "auto"}, "json", true},
		{"after separator", []string{"--", "--narration=json"}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wantsJSON(tt.args, tt.env); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddFlagsRegistersCompletions(t *testing.T) {
	root := &cobra.Command{Use: "app"}
	var f flagValues
	addFlags(root, &f)

	for _, name := range []string{"quiet", "verbose", "color", "narration"} {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Fatalf("flag %q is not registered", name)
		}
	}
	if root.PersistentFlags().ShorthandLookup("q") == nil {
		t.Fatal("shorthand q is not registered")
	}

	cases := map[string][]string{
		"color":     {"auto", "always", "never"},
		"narration": {"auto", "plain", "json"},
	}
	for name, want := range cases {
		fn, ok := root.GetFlagCompletionFunc(name)
		if !ok {
			t.Fatalf("no completion for %q", name)
		}
		got, _ := fn(root, nil, "")
		if !slices.Equal(got, want) {
			t.Fatalf("completion for %q = %v, want %v", name, got, want)
		}
	}
}
