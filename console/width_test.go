package console

import "testing"

func TestDisplayWidthCountsRunes(t *testing.T) {
	cases := map[string]int{"abc": 3, "héllo": 5, "→ x": 3, "": 0}
	for s, want := range cases {
		if got := displayWidth(s); got != want {
			t.Errorf("displayWidth(%q) = %d, want %d", s, got, want)
		}
	}
}

func TestEllipsize(t *testing.T) {
	cases := []struct {
		s     string
		width int
		want  string
	}{
		{"Installing jq", 20, "Installing jq"},
		{"Installing jq 1.8.3", 10, "Installin…"},
		{"abc", 1, "…"},
		{"abc", 0, ""},
	}
	for _, c := range cases {
		if got := ellipsize(c.s, c.width); got != c.want {
			t.Errorf("ellipsize(%q, %d) = %q, want %q", c.s, c.width, got, c.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		s     string
		width int
		want  string
	}{
		{"héllo", 3, "hél"},
		{"hi", 5, "hi"},
	}
	for _, c := range cases {
		if got := truncate(c.s, c.width); got != c.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", c.s, c.width, got, c.want)
		}
	}
}
