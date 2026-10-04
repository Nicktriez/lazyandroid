// Package android wraps the `android` CLI: it runs commands, streams their
// output, and parses the human-readable formats they emit into values.
package android

// Emulator describes an Android Virtual Device as reported by
// `android emulator list --long`.
type Emulator struct {
	ID       string // AVD ID, e.g. "medium_phone"
	Name     string // display name, e.g. "Medium Phone"
	APILevel string // e.g. "android-36"
	Status   string // e.g. "Offline"
	Serial   string // device serial when booted, empty otherwise
}

// Running reports whether the AVD currently has a device serial attached.
// `android emulator list --long` leaves the Serial column empty for AVDs that
// are not booted.
func (e Emulator) Running() bool { return e.Serial != "" }

// KeyValue is a single "key: value" pair from `android info`.
type KeyValue struct {
	Key   string
	Value string
}
