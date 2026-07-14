// Command mustlint lints RFC 2119 requirement language in plain-Markdown
// specification documents.
package main

import (
	"os"

	"github.com/JaydenCJ/mustlint/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
