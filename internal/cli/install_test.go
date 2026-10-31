package cli

import (
	"reflect"
	"testing"
)

func TestParseInstallArgsDefault(t *testing.T) {
	native, sources, noRegistry, rest, err := parseInstallArgs([]string{"--repo-update"})
	if err != nil {
		t.Fatal(err)
	}
	if native || noRegistry || len(sources) != 0 || !reflect.DeepEqual(rest, []string{"--repo-update"}) {
		t.Fatalf("got native=%v sources=%v noRegistry=%v rest=%v", native, sources, noRegistry, rest)
	}
}

func TestParseInstallArgsNative(t *testing.T) {
	native, sources, noRegistry, rest, err := parseInstallArgs([]string{"--native", "--source", "https://a.example/cdn", "--source", "https://b.example/cdn"})
	if err != nil {
		t.Fatal(err)
	}
	if !native {
		t.Fatal("expected native=true")
	}
	if noRegistry {
		t.Fatal("expected noRegistry=false")
	}
	if !reflect.DeepEqual(sources, []string{"https://a.example/cdn", "https://b.example/cdn"}) {
		t.Fatalf("unexpected sources: %v", sources)
	}
	if len(rest) != 0 {
		t.Fatalf("unexpected rest: %v", rest)
	}
}

func TestParseInstallArgsNoRegistry(t *testing.T) {
	native, _, noRegistry, _, err := parseInstallArgs([]string{"--native", "--no-registry"})
	if err != nil {
		t.Fatal(err)
	}
	if !native || !noRegistry {
		t.Fatalf("got native=%v noRegistry=%v", native, noRegistry)
	}
}

func TestParseInstallArgsSourceWithoutNativeIsRejected(t *testing.T) {
	_, _, _, _, err := parseInstallArgs([]string{"--source", "https://a.example/cdn"})
	if err == nil {
		t.Fatal("expected an error: --source without --native has no effect and must not be silently accepted")
	}
}

func TestParseInstallArgsNoRegistryWithoutNativeIsRejected(t *testing.T) {
	_, _, _, _, err := parseInstallArgs([]string{"--no-registry"})
	if err == nil {
		t.Fatal("expected an error: --no-registry without --native has no effect and must not be silently accepted")
	}
}

func TestParseInstallArgsSourceMissingValue(t *testing.T) {
	_, _, _, _, err := parseInstallArgs([]string{"--native", "--source"})
	if err == nil {
		t.Fatal("expected an error for --source with no URL")
	}
}
