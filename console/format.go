package console

import (
	"fmt"
	"time"
	"unicode"
)

// FormatDuration returns d as a suffix such as " (5s)" or " (2m05s)". It returns an empty string for less than a second.
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return ""
	}
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf(" (%ds)", int(d.Seconds()))
	}
	return fmt.Sprintf(" (%dm%02ds)", int(d.Minutes()), int(d.Seconds())%60)
}

// FormatBytes returns n as a size in powers of 1024, such as "512 B" or "3 MB".
func FormatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.0f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// Plural returns n followed by noun, with an s added unless n is 1.
func Plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func capitalizeLead(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
