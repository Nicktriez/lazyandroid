package android

import "context"

// Emulators lists the configured AVDs via `android emulator list --long`.
func (r Runner) Emulators(ctx context.Context) ([]Emulator, error) {
	out, err := r.Output(ctx, "emulator", "list", "--long")
	if err != nil {
		return nil, err
	}
	return ParseEmulatorList(out)
}

// Info returns the environment key/value pairs from `android info`.
func (r Runner) Info(ctx context.Context) ([]KeyValue, error) {
	out, err := r.Output(ctx, "info")
	if err != nil {
		return nil, err
	}
	return ParseInfo(out), nil
}

// SDKPackages returns the pre-aligned table from `android sdk list`. The text
// is passed through verbatim: the CLI aligns it for terminal display and the
// columns are not worth re-deriving.
func (r Runner) SDKPackages(ctx context.Context, all bool) (string, error) {
	args := []string{"sdk", "list"}
	if all {
		args = append(args, "--all")
	}
	return r.Output(ctx, args...)
}
