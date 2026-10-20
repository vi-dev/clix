package console

import (
	"testing"
	"time"
)

func TestPlural(t *testing.T) {
	cases := []struct {
		n    int
		noun string
		want string
	}{
		{0, "package", "0 packages"},
		{1, "package", "1 package"},
		{2, "tag", "2 tags"},
		{1, "project package", "1 project package"},
		{3, "project package", "3 project packages"},
	}
	for _, tc := range cases {
		if got := Plural(tc.n, tc.noun); got != tc.want {
			t.Errorf("Plural(%d, %q) = %q, want %q", tc.n, tc.noun, got, tc.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{500 * time.Millisecond, ""},
		{3 * time.Second, " (3s)"},
		{83 * time.Second, " (1m23s)"},
	}
	for _, c := range cases {
		if got := FormatDuration(c.d); got != c.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1 KB"},
		{1536, "2 KB"},
		{1048576, "1 MB"},
	}
	for _, c := range cases {
		if got := FormatBytes(c.n); got != c.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestCapitalizeLead(t *testing.T) {
	cases := map[string]string{"": "", "boom": "Boom", "Boom": "Boom", "élan": "Élan", "1 failed": "1 failed"}
	for in, want := range cases {
		if got := capitalizeLead(in); got != want {
			t.Errorf("capitalizeLead(%q) = %q, want %q", in, got, want)
		}
	}
}
