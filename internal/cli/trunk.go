package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	zauth "github.com/zorvok-dev/zorvok-ios/internal/auth"
	"github.com/zorvok-dev/zorvok-ios/internal/registry"
)

// runTrunkRegister is `zorvok trunk register`, an alias for `zorvok login`.
func runTrunkRegister(out, errOut io.Writer, args []string) int {
	return runLogin(out, errOut, args)
}

// runTrunkInfo is `zorvok trunk info`, an alias for `zorvok info`.
func runTrunkInfo(out, errOut io.Writer, args []string) int {
	return runInfo(out, errOut, args)
}

// loggedInClient resolves the configured registry and this machine's
// stored session for it, the shared precondition for every
// authenticated trunk command. It reports failure through its error
// return rather than writing to errOut itself, matching every other
// helper in this package — the caller decides how to present it.
func loggedInClient() (client *registry.Client, url, token string, err error) {
	url, err = registryURL()
	if err != nil {
		return nil, "", "", err
	}
	store, err := zauth.LoadStore()
	if err != nil {
		return nil, "", "", err
	}
	creds, found := store.Get(url)
	if !found {
		return nil, "", "", fmt.Errorf("not logged in to %s — run `zorvok login <email>` first", url)
	}
	return registry.NewClient(url), url, creds.Token, nil
}

// runTrunkMe implements `zorvok trunk me`, mirroring `pod trunk me`:
// shows the account behind this machine's stored session.
func runTrunkMe(out, errOut io.Writer, _ []string) int {
	client, url, token, err := loggedInClient()
	if err != nil {
		return exitErr(errOut, "%v", err)
	}
	account, err := client.Me(context.Background(), token)
	if err != nil {
		return exitErr(errOut, "%v", err)
	}
	fmt.Fprintf(out, "Email:     %s\n", account.Email)
	if account.Name != "" {
		fmt.Fprintf(out, "Name:      %s\n", account.Name)
	}
	fmt.Fprintf(out, "Verified:  %v\n", account.Verified)
	fmt.Fprintf(out, "Registry:  %s\n", url)
	return 0
}

// runTrunkAddOwner implements `zorvok trunk add-owner POD OWNER-EMAIL`,
// mirroring `pod trunk add-owner`: grants another account publish
// rights on a pod you already own.
func runTrunkAddOwner(out, errOut io.Writer, args []string) int {
	if len(args) != 2 {
		return exitErr(errOut, "usage: zorvok trunk add-owner POD OWNER-EMAIL")
	}
	pod, owner := args[0], args[1]

	client, _, token, err := loggedInClient()
	if err != nil {
		return exitErr(errOut, "%v", err)
	}
	if err := client.AddOwner(context.Background(), token, pod, owner); err != nil {
		return exitErr(errOut, "%v", err)
	}
	fmt.Fprintf(out, "Added %s as an owner of %s.\n", owner, pod)
	return 0
}

// runTrunkRemoveOwner implements `zorvok trunk remove-owner POD
// OWNER-EMAIL`, mirroring `pod trunk remove-owner`: revokes publish
// rights. The registry refuses to remove a pod's last remaining owner
// (it has no reserved-name or re-claim mechanism — that would orphan
// the pod for every future publisher).
func runTrunkRemoveOwner(out, errOut io.Writer, args []string) int {
	if len(args) != 2 {
		return exitErr(errOut, "usage: zorvok trunk remove-owner POD OWNER-EMAIL")
	}
	pod, owner := args[0], args[1]

	client, _, token, err := loggedInClient()
	if err != nil {
		return exitErr(errOut, "%v", err)
	}
	if err := client.RemoveOwner(context.Background(), token, pod, owner); err != nil {
		return exitErr(errOut, "%v", err)
	}
	fmt.Fprintf(out, "Removed %s as an owner of %s.\n", owner, pod)
	return 0
}

// runTrunkDeprecate implements `zorvok trunk deprecate NAME
// [--in-favor-of=OTHER]`, mirroring `pod trunk deprecate`.
func runTrunkDeprecate(out, errOut io.Writer, args []string) int {
	if len(args) == 0 {
		return exitErr(errOut, "usage: zorvok trunk deprecate NAME [--in-favor-of=OTHER]")
	}
	name := args[0]
	message := ""
	for _, a := range args[1:] {
		rest, ok := strings.CutPrefix(a, "--in-favor-of=")
		if !ok || rest == "" {
			return exitErr(errOut, "unrecognized argument %q — usage: zorvok trunk deprecate NAME [--in-favor-of=OTHER]", a)
		}
		message = "use " + rest + " instead"
	}

	client, _, token, err := loggedInClient()
	if err != nil {
		return exitErr(errOut, "%v", err)
	}
	if err := client.Deprecate(context.Background(), token, name, message); err != nil {
		return exitErr(errOut, "%v", err)
	}
	fmt.Fprintf(out, "Deprecated %s.\n", name)
	return 0
}

// runTrunkDelete implements `zorvok trunk delete NAME VERSION`. Real
// CocoaPods warns its own users away from this ("Please consider using
// the deprecate command instead... Once deleted, this version can never
// be pushed again") because it breaks everyone still depending on that
// version. Zorvok's registry additionally treats a published version as
// immutable by design (ADR 0003) — deletion would violate that
// invariant for every other installer caching or resolving against it —
// so this command intentionally does not delete anything; it directs
// users to the safe alternative instead of silently no-oping.
func runTrunkDelete(_ io.Writer, errOut io.Writer, args []string) int {
	if len(args) != 2 {
		return exitErr(errOut, "usage: zorvok trunk delete NAME VERSION")
	}
	fmt.Fprintf(errOut, "zorvok trunk delete: not supported — published versions are immutable.\n")
	fmt.Fprintf(errOut, "Use `zorvok trunk deprecate %s` to warn consumers off a version instead.\n", args[0])
	return 1
}
