package android

import (
	"fmt"
	"strings"
)

// ParseInfo parses the `key: value` lines emitted by `android info`. Lines
// without a colon are skipped; CLI order is preserved.
func ParseInfo(out string) []KeyValue {
	var pairs []KeyValue
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		pairs = append(pairs, KeyValue{
			Key:   strings.TrimSpace(key),
			Value: strings.TrimSpace(value),
		})
	}
	return pairs
}

// emulatorListColumns are the header cells of the fixed-width table printed by
// `android emulator list --long`, in order.
var emulatorListColumns = []string{"AVD ID", "AVD Name", "API Level", "Status", "Serial"}

// ParseEmulatorList parses the fixed-width table emitted by
// `android emulator list --long`. Column positions are derived from the header
// row, so the parser follows the CLI's own alignment instead of hard-coding
// widths. Empty output yields no emulators and no error.
func ParseEmulatorList(out string) ([]Emulator, error) {
	lines := strings.Split(out, "\n")

	headerIdx := -1
	for i, line := range lines {
		if strings.Contains(line, "AVD ID") && strings.Contains(line, "AVD Name") {
			headerIdx = i
			break
		}
	}
	if headerIdx == -1 {
		if strings.TrimSpace(out) == "" {
			return nil, nil
		}
		return nil, fmt.Errorf("emulator list: header row not found in CLI output")
	}

	header := lines[headerIdx]
	offsets := make([]int, len(emulatorListColumns))
	searchFrom := 0
	for i, col := range emulatorListColumns {
		at := strings.Index(header[searchFrom:], col)
		if at < 0 {
			return nil, fmt.Errorf("emulator list: header column %q not found", col)
		}
		offsets[i] = searchFrom + at
		searchFrom = offsets[i] + len(col)
	}

	var emus []Emulator
	for _, line := range lines[headerIdx+1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := make([]string, len(offsets))
		for i := range offsets {
			end := len(line)
			if i+1 < len(offsets) {
				end = offsets[i+1]
			}
			fields[i] = cutField(line, offsets[i], end)
		}
		if fields[0] == "" {
			continue
		}
		emus = append(emus, Emulator{
			ID:       fields[0],
			Name:     fields[1],
			APILevel: fields[2],
			Status:   fields[3],
			Serial:   fields[4],
		})
	}
	return emus, nil
}

// cutField returns the trimmed slice s[start:end], tolerating rows that are
// shorter than the header row.
func cutField(s string, start, end int) string {
	if start >= len(s) {
		return ""
	}
	if end > len(s) {
		end = len(s)
	}
	if end < start {
		end = start
	}
	return strings.TrimSpace(s[start:end])
}
