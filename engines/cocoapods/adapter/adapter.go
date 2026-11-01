// Package adapter is the only code that shells out to the CocoaPods
// Ruby toolchain. Nothing else in this project talks to `pod` directly.
package adapter

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
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

// Capture executes `pod <args...>` in dir like Run, but returns stdout as
// a string instead of streaming it — for commands like `pod ipc spec`
// whose output is data Zorvok needs to parse, not progress to show the
// user. stderr still streams live to errOut, since `pod` may print
// warnings there even on success.
func Capture(errOut io.Writer, dir string, args ...string) (string, error) {
	path, err := Find()
	if err != nil {
		return "", err
	}

	var stdout bytes.Buffer
	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = errOut
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("%w (exit %d)", ErrCommandFailed, exitErr.ExitCode())
		}
		return "", fmt.Errorf("running pod %s: %w", args[0], err)
	}
	return stdout.String(), nil
}

// RunRubyScript runs a Ruby script against the same CocoaPods gem
// environment `pod` itself uses (see ResolveRubyEnv), feeding the
// script over stdin (`ruby -`) rather than writing it to a temp file —
// script is a compile-time-embedded string (installer/zorvok_install.rb),
// not something that needs a path on disk. args become the script's
// ARGV. Used only by the native installer path; everything else in
// this package shells to `pod` itself.
func RunRubyScript(out, errOut io.Writer, dir, script string, args ...string) error {
	env, err := ResolveRubyEnv()
	if err != nil {
		return err
	}

	// Two separate things need forcing to UTF-8, independently, on a
	// machine with no locale configured (LANG/LC_ALL unset — common in
	// minimal containers and CI, where Ruby otherwise assumes US-ASCII):
	// the `-E UTF-8:UTF-8` flag sets the default external/internal
	// encoding IO uses, but has no effect on how `ruby -` parses the
	// script's own source read from stdin — that's controlled only by a
	// `# encoding: UTF-8` magic comment. Without the flag, writing a
	// string with so much as an em dash to out/errOut raises
	// Encoding::InvalidByteSequenceError; without the comment, the
	// script fails to parse at all the moment one appears in its source.
	cmdArgs := append([]string{"-E", "UTF-8:UTF-8", "-"}, args...)
	cmd := exec.Command(env.RubyPath, cmdArgs...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader("# encoding: UTF-8\n" + script)
	cmd.Stdout = out
	cmd.Stderr = errOut
	cmd.Env = rubyScriptEnv(env)

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("%w (exit %d)", ErrCommandFailed, exitErr.ExitCode())
		}
		return fmt.Errorf("running ruby script: %w", err)
	}
	return nil
}

// rubyScriptEnv builds the environment for the embedded Ruby shim. It
// starts from the inherited process environment but strips RUBYOPT and
// any BUNDLE_* variable, which would otherwise let whatever Gemfile or
// bundler context invoked `zorvok` leak into the shim and silently
// change which CocoaPods gems it loads — defeating the whole point of
// resolving env.GemHome from the real `pod` executable in the first
// place. GEM_HOME and GEM_PATH are stripped too and, when
// ResolveRubyEnv found a GEM_HOME (the Homebrew-wrapper shape), set to
// exactly what the real `pod` wrapper script itself sets: GEM_HOME
// alone, with GEM_PATH left unset so RubyGems computes it fresh from
// GEM_HOME plus Ruby's own default gem path — explicitly pinning
// GEM_PATH to GemHome looks equivalent but isn't: it excludes the
// default path entirely, which is where Ruby's own bundled default
// gems (minitest, and others `cocoapods-core` depends on) actually
// live, breaking `require 'cocoapods'` outright.
func rubyScriptEnv(env RubyEnv) []string {
	inherited := os.Environ()
	out := make([]string, 0, len(inherited)+1)
	for _, kv := range inherited {
		key, _, _ := strings.Cut(kv, "=")
		switch {
		case key == "RUBYOPT", strings.HasPrefix(key, "BUNDLE_"), key == "GEM_PATH", key == "GEM_HOME":
			continue
		default:
			out = append(out, kv)
		}
	}
	if env.GemHome != "" {
		out = append(out, "GEM_HOME="+env.GemHome)
	}
	return out
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
