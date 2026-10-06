package android

import "strings"

// SanitizeText prepares one line of CLI output for the panes. A carriage return
// — a progress counter being animated in place, which `android install` emits —
// sends the terminal's cursor back to column 0, so the line would be repainted
// from the left edge over the pane next to it. Every control character is
// dropped, including the ESC that begins an ANSI sequence: colours the CLI
// emits would otherwise leak into the interface's own styling.
func SanitizeText(line string) string {
	if isPlain(line) {
		return line
	}
	var b strings.Builder
	b.Grow(len(line))
	for _, r := range line {
		if r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// isPlain reports whether the line can be used as-is, which is the common case
// and keeps the allocation off the hot path.
func isPlain(s string) bool {
	for i := range len(s) {
		if s[i] < 0x20 || s[i] == 0x7f {
			return false
		}
	}
	return true
}
