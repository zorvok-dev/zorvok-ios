package registrytest

import (
	"os"
	"testing"
)

// TestSubprocessEnvExtendsRatherThanReplacesTheEnvironment proves the
// zorvok-registry subprocess inherits the test process's environment
// (PATH, HOME, ...) in addition to the two ZORVOK_REGISTRY_* vars it
// needs, rather than being launched with only those two and nothing
// else.
func TestSubprocessEnvExtendsRatherThanReplacesTheEnvironment(t *testing.T) {
	t.Setenv("ZORVOK_REGISTRYTEST_MARKER", "present")

	env := subprocessEnv("127.0.0.1:0", "http://127.0.0.1:0")

	want := map[string]bool{
		"PATH=" + os.Getenv("PATH"):                   false,
		"ZORVOK_REGISTRYTEST_MARKER=present":          false,
		"ZORVOK_REGISTRY_ADDR=127.0.0.1:0":            false,
		"ZORVOK_REGISTRY_BASE_URL=http://127.0.0.1:0": false,
	}
	for _, e := range env {
		if _, ok := want[e]; ok {
			want[e] = true
		}
	}
	for entry, found := range want {
		if !found {
			t.Fatalf("subprocess env missing %q; got %v", entry, env)
		}
	}
}
