package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/zorvok-dev/zorvok/internal/config"
)

// runConfig implements `zorvok config [get <key> | set <key> <value> | unset <key> | list]`.
//
// Like `git config`, settings are project-local (.zorvok.yml in the
// current directory) by default; pass --global to target
// ~/.config/zorvok/config.yml instead. With no subcommand, prints the
// effective (project-over-global) merged settings.
func runConfig(out, errOut io.Writer, args []string) int {
	global := false
	var positional []string
	for _, a := range args {
		if a == "--global" {
			global = true
			continue
		}
		positional = append(positional, a)
	}

	globalPath, err := config.GlobalPath()
	if err != nil {
		return exitErr(errOut, "%v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return exitErr(errOut, "determine working directory: %v", err)
	}
	projectPath := config.ProjectPath(cwd)

	targetPath := projectPath
	if global {
		targetPath = globalPath
	}

	if len(positional) == 0 {
		return configList(out, errOut, globalPath, projectPath)
	}

	switch positional[0] {
	case "get":
		if len(positional) != 2 {
			return exitErr(errOut, "usage: zorvok config get <key>")
		}
		return configGet(out, errOut, globalPath, projectPath, positional[1])
	case "set":
		if len(positional) != 3 {
			return exitErr(errOut, "usage: zorvok config set <key> <value> [--global]")
		}
		return configSet(out, errOut, targetPath, positional[1], positional[2])
	case "unset":
		if len(positional) != 2 {
			return exitErr(errOut, "usage: zorvok config unset <key> [--global]")
		}
		return configUnset(out, errOut, targetPath, positional[1])
	default:
		return exitErr(errOut, "unknown config subcommand %q (expected get, set, unset, or no argument to list)", positional[0])
	}
}

func configList(out, errOut io.Writer, globalPath, projectPath string) int {
	g, err := config.Load(globalPath)
	if err != nil {
		return exitErr(errOut, "%v", err)
	}
	p, err := config.Load(projectPath)
	if err != nil {
		return exitErr(errOut, "%v", err)
	}
	merged := config.Merged(g, p)

	if len(merged.Keys()) == 0 {
		fmt.Fprintln(out, "no configuration set. see: zorvok config set <key> <value>")
		return 0
	}
	for _, k := range merged.Keys() {
		v, _ := merged.Get(k)
		fmt.Fprintf(out, "%s = %s\n", k, v)
	}
	return 0
}

func configGet(out, errOut io.Writer, globalPath, projectPath, key string) int {
	g, err := config.Load(globalPath)
	if err != nil {
		return exitErr(errOut, "%v", err)
	}
	p, err := config.Load(projectPath)
	if err != nil {
		return exitErr(errOut, "%v", err)
	}
	merged := config.Merged(g, p)

	v, ok := merged.Get(key)
	if !ok {
		return exitErr(errOut, "no value set for %q", key)
	}
	fmt.Fprintln(out, v)
	return 0
}

func configSet(out, errOut io.Writer, path, key, value string) int {
	cfg, err := config.Load(path)
	if err != nil {
		return exitErr(errOut, "%v", err)
	}
	cfg.Set(key, value)
	if err := cfg.Save(); err != nil {
		return exitErr(errOut, "%v", err)
	}
	fmt.Fprintf(out, "%s = %s  (%s)\n", key, value, path)
	return 0
}

func configUnset(out, errOut io.Writer, path, key string) int {
	cfg, err := config.Load(path)
	if err != nil {
		return exitErr(errOut, "%v", err)
	}
	if _, ok := cfg.Get(key); !ok {
		return exitErr(errOut, "no value set for %q in %s", key, path)
	}
	cfg.Unset(key)
	if err := cfg.Save(); err != nil {
		return exitErr(errOut, "%v", err)
	}
	fmt.Fprintf(out, "unset %s  (%s)\n", key, path)
	return 0
}
