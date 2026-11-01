package podspec

import (
	"bytes"
	"errors"
	"testing"

	"github.com/zorvok-dev/zorvok-ios/engines/cocoapods/adapter"
	"github.com/zorvok-dev/zorvok-ios/internal/testutil"
)

var stubPod = testutil.StubPod

func TestLintNotOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	var out, errOut bytes.Buffer
	if err := Lint(&out, &errOut, t.TempDir(), nil); !errors.Is(err, adapter.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestLintForwardsExtraArgsAfterLibLint(t *testing.T) {
	stubPod(t, `echo "$@"`)

	var out, errOut bytes.Buffer
	if err := Lint(&out, &errOut, t.TempDir(), []string{"--allow-warnings", "Foo.podspec"}); err != nil {
		t.Fatalf("Lint: %v", err)
	}
	if want := "lib lint --allow-warnings Foo.podspec\n"; out.String() != want {
		t.Fatalf("got argv %q, want %q", out.String(), want)
	}
}

func TestLintPropagatesPodFailure(t *testing.T) {
	stubPod(t, `echo "boom" >&2; exit 1`)

	var out, errOut bytes.Buffer
	err := Lint(&out, &errOut, t.TempDir(), nil)
	if !errors.Is(err, adapter.ErrCommandFailed) {
		t.Fatalf("got %v, want ErrCommandFailed", err)
	}
	if errOut.String() != "boom\n" {
		t.Fatalf("expected pod's own stderr to pass through, got %q", errOut.String())
	}
}

func TestJSONNotOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	var errOut bytes.Buffer
	if _, err := JSON(&errOut, t.TempDir(), "Foo.podspec"); !errors.Is(err, adapter.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestJSONReturnsCapturedStdout(t *testing.T) {
	stubPod(t, `echo '{"name":"Foo"}'`)

	var errOut bytes.Buffer
	got, err := JSON(&errOut, t.TempDir(), "Foo.podspec")
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if want := "{\"name\":\"Foo\"}\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestJSONForwardsPodspecPathAndDir(t *testing.T) {
	stubPod(t, `echo "$@"`)

	var errOut bytes.Buffer
	got, err := JSON(&errOut, t.TempDir(), "Sub/Foo.podspec")
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if want := "ipc spec Sub/Foo.podspec\n"; got != want {
		t.Fatalf("got argv %q, want %q", got, want)
	}
}

func TestJSONPropagatesPodFailure(t *testing.T) {
	stubPod(t, `echo "invalid podspec" >&2; exit 1`)

	var errOut bytes.Buffer
	_, err := JSON(&errOut, t.TempDir(), "Foo.podspec")
	if !errors.Is(err, adapter.ErrCommandFailed) {
		t.Fatalf("got %v, want ErrCommandFailed", err)
	}
	if errOut.String() != "invalid podspec\n" {
		t.Fatalf("expected pod's own stderr to pass through, got %q", errOut.String())
	}
}
