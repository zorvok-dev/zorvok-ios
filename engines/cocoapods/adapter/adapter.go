// Package adapter is the sole boundary between Zorvok's Go code and the
// CocoaPods Ruby toolchain. Per ADR 0002, Milestone 2 delegates
// installation, resolution, and Xcode integration entirely to the real
// `pod` executable; nothing outside this package shells out to it.
package adapter

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// ErrNotFound is returned when the `pod` executable isn't on PATH.
var ErrNotFound = errors.New("`pod` not found on PATH")

// ErrCommandFailed wraps a non-zero exit from `pod` itself. `pod` has
// already streamed its own diagnostics to errOut by the time this is
// returned, so callers should propagate the exit code without printing
// this error's text again.
var ErrCommandFailed = errors.New("pod command failed")

// Find locates the `pod` executable, or returns ErrNotFound with guidance.
func Find() (string, error) {
	path, err := exec.LookPath("pod")
	if err != nil {
		return "", fmt.Errorf("%w — install CocoaPods with `gem install cocoapods`, or run `zorvok doctor`", ErrNotFound)
	}
	return path, nil
}

// Run executes `pod <args...>` in dir, streaming its stdout/stderr live
// (CocoaPods prints real-time progress during install/update, which users
// expect to see as it happens, not buffered until completion).
func Run(out, errOut io.Writer, dir string, args ...string) error {
	path, err := Find()
	if err != nil {
		return err
	}

	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = errOut
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("%w (exit %d)", ErrCommandFailed, exitErr.ExitCode())
		}
		return fmt.Errorf("running pod %s: %w", args[0], err)
	}
	return nil
}

// Version returns `pod --version`'s output.
func Version() (string, error) {
	path, err := Find()
	if err != nil {
		return "", err
	}
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("pod --version: %w", err)
	}
	return string(out), nil
}
