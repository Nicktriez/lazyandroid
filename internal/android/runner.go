package android

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Runner invokes the `android` CLI.
type Runner struct {
	// Bin overrides the executable name. Empty means "android".
	Bin string
}

func (r Runner) bin() string {
	if r.Bin == "" {
		return "android"
	}
	return r.Bin
}

// Error describes a failed `android` invocation.
type Error struct {
	Args   []string
	Output string // combined stdout+stderr, possibly empty
	Err    error
}

func (e *Error) Error() string {
	msg := strings.TrimSpace(e.Output)
	if msg == "" {
		msg = e.Err.Error()
	}
	return fmt.Sprintf("android %s: %s", strings.Join(e.Args, " "), msg)
}

func (e *Error) Unwrap() error { return e.Err }

// Output runs the command to completion and returns its combined output.
// On a non-zero exit the output is still returned, alongside an *Error.
func (r Runner) Output(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, r.bin(), args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), &Error{Args: args, Output: string(out), Err: err}
	}
	return string(out), nil
}

// Stream is a running `android` invocation.
type Stream struct {
	// Lines yields combined stdout/stderr one line at a time and is closed
	// when the process exits.
	Lines <-chan string
	// Done yields the process exit error exactly once, after Lines is closed.
	Done <-chan error
}

// Stream starts the command and returns immediately. The process is killed
// when ctx is cancelled, which closes Lines and reports "signal: killed" on
// Done.
func (r Runner) Stream(ctx context.Context, args ...string) (*Stream, error) {
	cmd := exec.CommandContext(ctx, r.bin(), args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	// Interleave stderr into the same pipe: build and deploy output is
	// interesting regardless of the stream it arrives on.
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	lines := make(chan string)
	done := make(chan error, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
		done <- cmd.Wait()
	}()
	return &Stream{Lines: lines, Done: done}, nil
}
