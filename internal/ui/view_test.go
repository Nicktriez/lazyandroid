package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/Nicktriez/lazyandroid/internal/android"
)

func TestVisibleRange(t *testing.T) {
	tests := []struct {
		name           string
		n, cursor, h   int
		wantStart, end int
	}{
		{"empty", 0, 0, 5, 0, 0},
		{"zero height", 10, 0, 0, 0, 0},
		{"fits", 3, 2, 5, 0, 3},
		{"exact fit", 5, 0, 5, 0, 5},
		{"scrolled to bottom", 20, 19, 5, 15, 20},
		{"scrolled to top", 20, 0, 5, 0, 5},
		{"middle keeps cursor visible", 20, 10, 5, 8, 13},
		{"last cursor", 7, 6, 3, 4, 7},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start, end := visibleRange(tc.n, tc.cursor, tc.h)
			if start != tc.wantStart || end != tc.end {
				t.Errorf("visibleRange(%d, %d, %d) = (%d, %d), want (%d, %d)",
					tc.n, tc.cursor, tc.h, start, end, tc.wantStart, tc.end)
			}
			if end-start > tc.h {
				t.Errorf("window is %d rows tall, exceeds budget %d", end-start, tc.h)
			}
			if tc.n > 0 && tc.h > 0 && (tc.cursor < start || tc.cursor >= end) {
				t.Errorf("cursor %d is outside window [%d, %d)", tc.cursor, start, end)
			}
		})
	}
}

// stateModel is a sized model with one row per state a list can report. The
// cursor is on the first row of each list, so the tests see the selected
// rendering as well as the plain one.
func stateModel() Model {
	m := Model{width: 80}
	m.devices = []android.Device{
		{Serial: "34131JEGR08885", State: "device", Model: "Pixel_6a"},
		{Serial: "R58M12345", State: "unauthorized"},
		{Serial: "emulator-5554", State: "offline"},
		{Serial: "R58M99999", State: "no permissions"},
	}
	m.emulators = []android.Emulator{
		{ID: "medium_phone", APILevel: "android-36", Serial: "emulator-5554"},
		{ID: "small_phone", APILevel: "android-36.1"},
	}
	return m
}

// sgrParams returns the parameters of the SGR sequence that colours text inside
// a rendered string, so a test can assert the foreground and the background a
// token ended up with rather than the exact byte order lipgloss emits.
func sgrParams(rendered, text string) []string {
	at := strings.LastIndex(rendered, text)
	if at < 0 {
		return nil
	}
	start := strings.LastIndex(rendered[:at], "\x1b[")
	if start < 0 {
		return nil
	}
	seq := strings.TrimSuffix(strings.TrimPrefix(rendered[start:at], "\x1b["), "m")
	return strings.Split(seq, ";")
}

func hasParams(got, want []string) bool {
	for _, w := range want {
		if !slices.Contains(got, w) {
			return false
		}
	}
	return true
}

// The state column is the cue the user acts on: green when the device can be
// targeted, yellow while it is seen but not usable yet, red when the host has
// to fix something.
func TestRowStatesAreColoured(t *testing.T) {
	m := stateModel()
	for _, tc := range []struct {
		name  string
		state string
		want  lipgloss.Style
		row   string
	}{
		{"ready device", "ready", styleGood, m.deviceRow(0, 80)},
		{"unauthorised device", "unauthorized", styleBad, m.deviceRow(1, 80)},
		{"offline device", "offline", styleWarn, m.deviceRow(2, 80)},
		{"device with no permissions", "no permissions", styleBad, m.deviceRow(3, 80)},
		{"running AVD", "running", styleGood, m.emulatorRow(0, 80)},
		{"offline AVD", "offline", styleWarn, m.emulatorRow(1, 80)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := sgrParams(tc.row, tc.state)
			if want := sgrParams(tc.want.Render(tc.state), tc.state); !hasParams(got, want) {
				t.Errorf("row = %q, state %q coloured %v, want %v", tc.row, tc.state, got, want)
			}
		})
	}
}

// The selected row must keep its highlight and still show the state colour:
// rendering the whole line with the selection style would drop the colour.
func TestSelectedRowKeepsItsHighlightAndStateColour(t *testing.T) {
	m := stateModel()
	for _, tc := range []struct{ name, state, row string }{
		{"device", "ready", m.deviceRow(0, 80)},
		{"AVD", "offline", m.emulatorRow(1, 80)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := sgrParams(tc.row, tc.state)
			if !hasParams(params, sgrParams(styleSel.Render("x"), "x")) {
				t.Errorf("row = %q, lost the selection highlight: %v", tc.row, params)
			}
			if !hasParams(params, sgrParams(stateStyle(tc.state).Render("x"), "x")) {
				t.Errorf("row = %q, lost the %s state colour: %v", tc.row, tc.state, params)
			}
		})
	}
}
