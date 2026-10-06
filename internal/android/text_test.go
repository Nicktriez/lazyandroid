package android

import "testing"

// `android install` animates a progress counter with carriage returns. A CR
// reaching the pane would send the cursor to column 0 and repaint the line over
// the pane to its left, so it has to go.
func TestSanitizeTextDropsCarriageReturns(t *testing.T) {
	got := SanitizeText("\rProgress: 32%\rProgress: 64%")
	if want := "Progress: 32%Progress: 64%"; got != want {
		t.Errorf("SanitizeText = %q, want %q", got, want)
	}
}

func TestSanitizeTextDropsEscapes(t *testing.T) {
	got := SanitizeText("\x1b[31mError\x1b[0m: nope")
	if want := "[31mError[0m: nope"; got != want {
		t.Errorf("SanitizeText = %q, want %q", got, want)
	}
}

func TestSanitizeTextDropsOtherControlCharacters(t *testing.T) {
	got := SanitizeText("a\tb\x00c\x7fd")
	if want := "abcd"; got != want {
		t.Errorf("SanitizeText = %q, want %q", got, want)
	}
}

// Ordinary output must come back untouched, allocation-free.
func TestSanitizeTextLeavesPlainTextAlone(t *testing.T) {
	const line = "Success — installed on Pixel 6a (34.1 MB)"
	if got := SanitizeText(line); got != line {
		t.Errorf("SanitizeText = %q, want it unchanged", got)
	}
}
