package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/zorvok-dev/zorvok-ios/internal/registry"
)

// runSearch implements `zorvok search [query]` — a case-insensitive
// substring search against the configured registry's own search API,
// the same registry `zorvok login`/`trunk push` already talk to.
func runSearch(out, errOut io.Writer, args []string) int {
	if len(args) > 1 {
		return exitErr(errOut, "usage: zorvok search [query] — got extra argument %q", args[1])
	}
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
			versions = fmt.Sprintf("%d version(s), latest %s", len(p.Versions), latestVersion(p.Versions))
		}
		deprecated := ""
		if p.Deprecated {
			deprecated = "  [DEPRECATED]"
		}
		fmt.Fprintf(out, "%-30s %s%s\n", p.Name, versions, deprecated)
	}
	return 0
}

// latestVersion returns the greatest of vs by numeric dot-separated
// component — the registry's response is a JSON array with no sort
// guarantee of its own, so picking vs[len(vs)-1] would report
// whichever version happened to be stored or returned last, not
// necessarily the newest (e.g. "9.0.0" sorting after "10.0.0").
// Non-numeric components (pre-release suffixes like "-beta") fall back
// to a plain string comparison for that component.
func latestVersion(vs []string) string {
	latest := vs[0]
	for _, v := range vs[1:] {
		if compareVersions(v, latest) > 0 {
			latest = v
		}
	}
	return latest
}

func compareVersions(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var av, bv string
		if i < len(as) {
			av = as[i]
		}
		if i < len(bs) {
			bv = bs[i]
		}
		an, aErr := strconv.Atoi(av)
		bn, bErr := strconv.Atoi(bv)
		if aErr == nil && bErr == nil {
			if an != bn {
				return an - bn
			}
			continue
		}
		if c := strings.Compare(av, bv); c != 0 {
			return c
		}
	}
	return 0
}

// runInfo implements `zorvok info <name>`.
func runInfo(out, errOut io.Writer, args []string) int {
	if len(args) == 0 {
		return exitErr(errOut, "usage: zorvok info <name>")
	}
	if len(args) > 1 {
		return exitErr(errOut, "usage: zorvok info <name> — got extra argument %q", args[1])
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
	if len(pkg.Owners) == 0 {
		fmt.Fprintln(out, "  Owners:   none")
	} else {
		fmt.Fprintf(out, "  Owners:   %s\n", strings.Join(pkg.Owners, ", "))
	}
	if len(pkg.Versions) == 0 {
		fmt.Fprintln(out, "  Versions: none published")
	} else {
		fmt.Fprintf(out, "  Versions: %s\n", strings.Join(pkg.Versions, ", "))
	}
	fmt.Fprintf(out, "  Registry: %s\n", url)
	return 0
}
