package cli

import (
	"io"
	"os"

	"github.com/zorvok-dev/zorvok-ios/engines/cocoapods/repo"
)

// runRegistryList implements `zorvok registry list`.
func runRegistryList(out, errOut io.Writer, args []string) int {
	cwd, err := os.Getwd()
	if err != nil {
		return exitErr(errOut, "determine working directory: %v", err)
	}
	if err := repo.List(out, errOut, cwd, args); err != nil {
		return reportDelegationError(errOut, err)
	}
	return 0
}

// runRegistryAdd implements `zorvok registry add <name> <url> [branch]`.
func runRegistryAdd(out, errOut io.Writer, args []string) int {
	if len(args) < 2 {
		return exitErr(errOut, "usage: zorvok registry add <name> <url> [branch]")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return exitErr(errOut, "determine working directory: %v", err)
	}
	if err := repo.Add(out, errOut, cwd, args[0], args[1], args[2:]); err != nil {
		return reportDelegationError(errOut, err)
	}
	return 0
}

// runRegistryAddCDN implements `zorvok registry add-cdn <name> <url>`.
func runRegistryAddCDN(out, errOut io.Writer, args []string) int {
	if len(args) < 2 {
		return exitErr(errOut, "usage: zorvok registry add-cdn <name> <url>")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return exitErr(errOut, "determine working directory: %v", err)
	}
	if err := repo.AddCDN(out, errOut, cwd, args[0], args[1], args[2:]); err != nil {
		return reportDelegationError(errOut, err)
	}
	return 0
}

// runRegistryRemove implements `zorvok registry remove <name>`.
func runRegistryRemove(out, errOut io.Writer, args []string) int {
	if len(args) < 1 {
		return exitErr(errOut, "usage: zorvok registry remove <name>")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return exitErr(errOut, "determine working directory: %v", err)
	}
	if err := repo.Remove(out, errOut, cwd, args[0], args[1:]); err != nil {
		return reportDelegationError(errOut, err)
	}
	return 0
}

// runRegistryUpdate implements `zorvok registry update [name]`.
func runRegistryUpdate(out, errOut io.Writer, args []string) int {
	cwd, err := os.Getwd()
	if err != nil {
		return exitErr(errOut, "determine working directory: %v", err)
	}
	name := ""
	rest := args
	if len(args) > 0 {
		name = args[0]
		rest = args[1:]
	}
	if err := repo.Update(out, errOut, cwd, name, rest); err != nil {
		return reportDelegationError(errOut, err)
	}
	return 0
}
