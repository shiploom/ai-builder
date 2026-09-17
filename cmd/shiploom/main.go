// Command shiploom is the Go port entry point (stdlib only).
package main

import (
	"os"

	"github.com/shiploom/ai-builder/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:]))
}
