package console

import "unicode/utf8"

func displayWidth(s string) int {
	return utf8.RuneCountInString(s)
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if displayWidth(s) <= width {
		return s
	}
	return string([]rune(s)[:width])
}

func ellipsize(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if displayWidth(s) <= width {
		return s
	}
	return truncate(s, width-1) + "…"
}
