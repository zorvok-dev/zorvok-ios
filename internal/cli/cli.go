// Package cli implements Zorvok's command routing. It knows nothing
// about CocoaPods or the registry protocol directly — each command
// delegates to the relevant internal/* or engines/* package. Milestone 1
// implements version/help/init/config/doctor only; the remaining
// commands are listed in help output as not-yet-implemented so the CLI
// never silently no-ops on a real user command.
package cli

import (
	"fmt"
	"io"
	"sort"
)

// command is one top-level CLI command.
type command struct {
	name        string
	summary     string
	run         func(out, errOut io.Writer, args []string) int
	implemented bool
}

// plannedCommand documents a command from the product brief that has no
// implementation yet, so `zorvok help` can say so honestly instead of
// failing with "unknown command" or (worse) silently doing nothing.
type plannedCommand struct {
	name      string
	summary   string
	milestone string
}

// plannedCommands is currently empty — every command named in the
// original product brief has a real implementation. Kept as a live
// mechanism (not deleted) since it's exactly what the next genuinely
// unimplemented command should be listed in, rather than silently
// falling through to "unknown command".
var plannedCommands = []plannedCommand{}

func commands() []command {
	return []command{
		{"--version", "Print the Zorvok version", runVersion, true},
		{"version", "Print the Zorvok version", runVersion, true},
		{"--help", "Show this help text", nil, true}, // handled specially below
		{"help", "Show this help text", nil, true},
		{"init", "Create a .zorvok.yml project config in the current directory", runInit, true},
		{"config", "View or change Zorvok configuration", runConfig, true},
		{"doctor", "Diagnose the local Zorvok/CocoaPods environment", runDoctor, true},
		{"install", "Install dependencies from the Podfile (delegates to `pod install`; --native [--source <url>] [--no-registry] opts in to Zorvok's own installer, auto-federating a configured registry_url)", runInstall, true},
		{"update", "Update installed dependencies (delegates to `pod update`)", runUpdate, true},
		{"lib lint", "Validate a podspec (delegates to `pod lib lint`)", runLibLint, true},
		{"login", "Authenticate with the Zorvok registry", runLogin, true},
		{"trunk register", "Authenticate with the Zorvok registry (alias of `zorvok login`)", runTrunkRegister, true},
		{"trunk push", "Publish a podspec to the Zorvok registry", runTrunkPush, true},
		{"trunk me", "Show the account behind this machine's stored session", runTrunkMe, true},
		{"trunk add-owner", "Grant another account publish rights on a pod you own", runTrunkAddOwner, true},
		{"trunk remove-owner", "Revoke an account's publish rights on a pod you own", runTrunkRemoveOwner, true},
		{"trunk deprecate", "Mark a pod as deprecated, optionally pointing at a replacement", runTrunkDeprecate, true},
		{"trunk delete", "Explains why version deletion isn't supported and what to do instead", runTrunkDelete, true},
		{"trunk info", "Show details for a package on the configured registry (alias of `zorvok info`)", runTrunkInfo, true},
		{"search", "Search the configured registry for a package by name", runSearch, true},
		{"info", "Show details for a package on the configured registry", runInfo, true},
		{"registry list", "List registered CocoaPods spec sources (delegates to `pod repo list`)", runRegistryList, true},
		{"registry add", "Register a git-backed spec source (delegates to `pod repo add`)", runRegistryAdd, true},
		{"registry add-cdn", "Register a CDN-backed spec source, e.g. a Zorvok registry (delegates to `pod repo add-cdn`)", runRegistryAddCDN, true},
		{"registry remove", "Unregister a spec source (delegates to `pod repo remove`)", runRegistryRemove, true},
		{"registry update", "Update one or all registered spec sources (delegates to `pod repo update`)", runRegistryUpdate, true},
		{"registry lint", "Validate every spec in a repo (delegates to `pod repo lint`)", delegateTo("repo", "lint"), true},
		{"registry push", "Push specs to a git-backed spec repo (delegates to `pod repo push`)", delegateTo("repo", "push"), true},
		{"lib create", "Scaffold a new pod (delegates to `pod lib create`)", delegateTo("lib", "create"), true},
		{"spec", "Manage podspecs: cat/create/edit/lint/which (delegates to `pod spec`)", delegateTo("spec"), true},
		{"cache", "Manipulate the local pod download cache: clean/list (delegates to `pod cache`)", delegateTo("cache"), true},
		{"ipc", "Inter-process communication: podfile/podfile-json/spec/list/repl (delegates to `pod ipc`)", delegateTo("ipc"), true},
		{"plugins", "List or search available CocoaPods plugins (delegates to `pod plugins`)", delegateTo("plugins"), true},
		{"list", "List all available pods (delegates to `pod list`)", delegateTo("list"), true},
		{"outdated", "Show outdated dependencies in the current Podfile.lock (delegates to `pod outdated`)", delegateTo("outdated"), true},
		{"try", "Download a pod's demo project and open it (delegates to `pod try`)", delegateTo("try"), true},
		{"env", "Display the local CocoaPods/Zorvok environment (delegates to `pod env`)", delegateTo("env"), true},
		{"setup", "Set up the local CocoaPods environment (delegates to `pod setup`)", delegateTo("setup"), true},
		{"deintegrate", "Remove all traces of CocoaPods from an Xcode project (delegates to `pod deintegrate`)", delegateTo("deintegrate"), true},
	}
}

// Execute runs the CLI and returns a process exit code.
func Execute(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		printHelp(errOut)
		return 1
	}

	switch args[0] {
	case "--help", "-h", "help":
		printHelp(out)
		return 0
	case "--version", "-v", "version":
		return runVersion(out, errOut, args[1:])
	}

	// Two-word commands (`lib lint`, `trunk push`, `registry add`) are
	// matched before single-word ones so `zorvok lib lint` doesn't get
	// swallowed by a bare `lib` lookup.
	if len(args) >= 2 {
		twoWord := args[0] + " " + args[1]
		rest := args[2:]
		for _, c := range commands() {
			if c.name == twoWord && c.run != nil {
				return c.run(out, errOut, rest)
			}
		}
		for _, p := range plannedCommands {
			if p.name == twoWord {
				return reportPlanned(errOut, twoWord, p.milestone)
			}
		}
	}

	name := args[0]
	rest := args[1:]

	for _, c := range commands() {
		if c.name == name && c.run != nil {
			return c.run(out, errOut, rest)
		}
	}

	for _, p := range plannedCommands {
		if p.name == name {
			return reportPlanned(errOut, name, p.milestone)
		}
	}

	fmt.Fprintf(errOut, "zorvok: unknown command %q\n\n", name)
	printHelp(errOut)
	return 1
}

func reportPlanned(errOut io.Writer, name, milestone string) int {
	fmt.Fprintf(errOut, "zorvok %s: not implemented yet (planned for %s)\n", name, milestone)
	fmt.Fprintf(errOut, "see ROADMAP.md for status: https://github.com/zorvok-dev/zorvok-ios/blob/main/ROADMAP.md\n")
	return 1
}

func printHelp(out io.Writer) {
	fmt.Fprintln(out, "zorvok — CocoaPods-compatible iOS dependency manager")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "USAGE:")
	fmt.Fprintln(out, "    zorvok <command> [arguments]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "COMMANDS:")

	type row struct{ name, summary, note string }
	var rows []row
	seen := map[string]bool{}
	for _, c := range commands() {
		if c.name == "--version" || c.name == "--help" || seen[c.name] {
			continue
		}
		seen[c.name] = true
		rows = append(rows, row{c.name, c.summary, ""})
	}
	for _, p := range plannedCommands {
		rows = append(rows, row{p.name, p.summary, "planned, " + p.milestone})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })

	for _, r := range rows {
		note := ""
		if r.note != "" {
			note = "  (not yet implemented — " + r.note + ")"
		}
		fmt.Fprintf(out, "    %-14s %s%s\n", r.name, r.summary, note)
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "    --version        Print the Zorvok version")
	fmt.Fprintln(out, "    --help           Show this help text")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Run 'zorvok doctor' to check your local environment.")
}

// exitErr writes a formatted error to errOut and returns exit code 1.
func exitErr(errOut io.Writer, format string, args ...any) int {
	fmt.Fprintf(errOut, "zorvok: "+format+"\n", args...)
	return 1
}
