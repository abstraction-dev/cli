// Command abstr is the Abstraction CLI: an interface to Astrid / Agent chat.
package main

import (
	"os"

	"github.com/abstraction-dev/cli/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:]))
}
