package ui

import "testing"

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
