package cli

import (
	"context"
	"io"

	zauth "github.com/zorvok-dev/zorvok/internal/auth"
	"github.com/zorvok-dev/zorvok/internal/registry"
)

// runLogin implements `zorvok login <email> [name]`, mirroring CocoaPods
// Trunk's registration UX (ARCHITECTURE.md §2.4): it issues a session
// token immediately, stored locally for this machine, but the account
// stays unverified — and therefore can't publish — until the emailed
// verification link is visited. There is no real mail transport yet
// (registry-server/auth.LogSender), so in local dev the link is printed
// to the registry server's own log, not actually emailed.
func runLogin(out, errOut io.Writer, args []string) int {
	if len(args) == 0 {
		return exitErr(errOut, "usage: zorvok login <email> [display name]")
	}
	email := args[0]
	name := ""
	if len(args) > 1 {
		name = args[1]
	}

	url, err := registryURL()
	if err != nil {
		return exitErr(errOut, "%v", err)
	}

	client := registry.NewClient(url)
	token, err := client.Register(context.Background(), email, name)
	if err != nil {
		return exitErr(errOut, "registration failed: %v", err)
	}

	store, err := zauth.LoadStore()
	if err != nil {
		return exitErr(errOut, "%v", err)
	}
	store.Set(url, zauth.Credentials{Email: email, Token: token})
	if err := store.Save(); err != nil {
		return exitErr(errOut, "%v", err)
	}

	io.WriteString(out, "Registered with "+url+" as "+email+".\n")
	io.WriteString(out, "A verification link was sent to your email — you must verify before you can publish.\n")
	io.WriteString(out, "(Running a local dev registry? The link is printed in that server's own log, not emailed.)\n")
	return 0
}
