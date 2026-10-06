package android

import "strings"

// HasFlag reports whether any of names is present in args, in either the
// "--flag" or "--flag=value" spelling. It never consumes a following word, so
// it is safe for valueless flags.
func HasFlag(args []string, names ...string) bool {
	for _, a := range args {
		for _, n := range names {
			if a == n || strings.HasPrefix(a, n+"=") {
				return true
			}
		}
	}
	return false
}

// WithDevice appends --device=<serial> unless args already select a device, so
// a serial the caller chose by hand is never overridden.
func WithDevice(args []string, serial string) []string {
	if serial == "" || HasFlag(args, "--device") {
		return args
	}
	return append(args, "--device="+serial)
}
