package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/zorvok-dev/zorvok-ios/internal/config"
)

// runInit creates a project-local .zorvok.yml. It never touches Podfile,
// Podfile.lock, or any .podspec — Zorvok's whole premise is that those
// stay exactly as CocoaPods left them.
func runInit(out, errOut io.Writer, _ []string) int {
	cwd, err := os.Getwd()
	if err != nil {
		return exitErr(errOut, "determine working directory: %v", err)
	}

	path := config.ProjectPath(cwd)
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(out, "already initialized: %s exists\n", path)
		return 0
	}

	cfg, err := config.Load(path)
	if err != nil {
		return exitErr(errOut, "%v", err)
	}
	cfg.Set("version", "1")
	if err := cfg.Save(); err != nil {
		return exitErr(errOut, "%v", err)
	}

	fmt.Fprintf(out, "created %s\n", path)

	if _, err := os.Stat("Podfile"); err == nil {
		fmt.Fprintln(out, "detected an existing Podfile — it was not modified.")
		fmt.Fprintln(out, "note: `zorvok install` is not implemented yet (planned for Milestone 2).")
	} else {
		fmt.Fprintln(out, "no Podfile found in this directory. Zorvok manages existing CocoaPods")
		fmt.Fprintln(out, "projects; it does not currently scaffold a new Podfile.")
	}

	return 0
}
