// Command zorvok is the Zorvok CLI entry point.
package main

import (
	"os"

	"github.com/zorvok-dev/zorvok/internal/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:], os.Stdout, os.Stderr))
}
