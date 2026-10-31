package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/zorvok-dev/zorvok/internal/registry"
)

// runSearch implements `zorvok search [query]` — a case-insensitive
// substring search against the configured registry's /v1/search API
// (registry-server/api), the same registry `zorvok login`/`trunk push`
// already talk to.
func runSearch(out, errOut io.Writer, args []string) int {
	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	url, err := registryURL()
	if err != nil {
		return exitErr(errOut, "%v", err)
	}

	client := registry.NewClient(url)
	results, err := client.Search(context.Background(), query)
	if err != nil {
		return exitErr(errOut, "%v", err)
	}

	if len(results) == 0 {
		fmt.Fprintf(out, "no packages found on %s", url)
		if query != "" {
			fmt.Fprintf(out, " matching %q", query)
		}
		fmt.Fprintln(out)
		return 0
	}

	for _, p := range results {
		versions := "no versions published"
		if len(p.Versions) > 0 {
			versions = fmt.Sprintf("%d version(s), latest %s", len(p.Versions), p.Versions[len(p.Versions)-1])
		}
		deprecated := ""
		if p.Deprecated {
			deprecated = "  [DEPRECATED]"
		}
		fmt.Fprintf(out, "%-30s %s%s\n", p.Name, versions, deprecated)
	}
	return 0
}

// runInfo implements `zorvok info <name>`.
func runInfo(out, errOut io.Writer, args []string) int {
	if len(args) == 0 {
		return exitErr(errOut, "usage: zorvok info <name>")
	}
	name := args[0]

	url, err := registryURL()
	if err != nil {
		return exitErr(errOut, "%v", err)
	}

	client := registry.NewClient(url)
	pkg, err := client.GetPackage(context.Background(), name)
	if err != nil {
		return exitErr(errOut, "%v", err)
	}

	fmt.Fprintf(out, "%s\n", pkg.Name)
	if pkg.Deprecated {
		msg := pkg.DeprecationMessage
		if msg == "" {
			msg = "(no reason given)"
		}
		fmt.Fprintf(out, "  DEPRECATED: %s\n", msg)
	}
	fmt.Fprintf(out, "  Owners:   %s\n", strings.Join(pkg.Owners, ", "))
	if len(pkg.Versions) == 0 {
		fmt.Fprintln(out, "  Versions: none published")
	} else {
		fmt.Fprintf(out, "  Versions: %s\n", strings.Join(pkg.Versions, ", "))
	}
	fmt.Fprintf(out, "  Registry: %s\n", url)
	return 0
}
