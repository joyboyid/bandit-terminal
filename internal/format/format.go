// Package format holds small display helpers shared by the panels.
package format

import (
	"fmt"
	"strings"
	"time"
)

// HumanBytes formats n as a short byte size, at most 5 characters
// for values up to the petabyte range (for example "1023B", "1.5K").
func HumanBytes(n uint64) string {
	if n < 1024 {
		return fmt.Sprintf("%dB", n)
	}
	units := []string{"K", "M", "G", "T", "P"}
	v := float64(n)
	u := -1
	for v >= 1024 && u < len(units)-1 {
		v /= 1024
		u++
	}
	if v >= 100 {
		return fmt.Sprintf("%.0f%s", v, units[u])
	}
	return fmt.Sprintf("%.1f%s", v, units[u])
}

// Uptime formats a duration the way a status bar wants it.
func Uptime(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	sec := int(d.Seconds())
	days := sec / 86400
	sec %= 86400
	hours := sec / 3600
	sec %= 3600
	mins := sec / 60
	sec %= 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %02dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %02dm", hours, mins)
	default:
		return fmt.Sprintf("%dm %02ds", mins, sec)
	}
}

// PrettyPath replaces a home-directory prefix with ~.
func PrettyPath(path, home string) string {
	if path == "" {
		return ""
	}
	if home != "" && (path == home || strings.HasPrefix(path, home+"/")) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

// ShellQuote quotes s for a POSIX shell, including empty strings.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
