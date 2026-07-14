// The stats subcommand: a keyword-usage inventory across the corpus.
package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/JaydenCJ/mustlint/internal/render"
	"github.com/JaydenCJ/mustlint/internal/spec"
)

func runStats(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return ExitOK // an explicit -h/--help is not a usage error
		}
		return ExitUsage
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "mustlint: unknown --format %q (want text or json)\n", *format)
		return ExitUsage
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "mustlint: no files given (try `mustlint stats docs/`)")
		return ExitUsage
	}

	docs, code := loadDocs(fs.Args(), nil, stderr)
	if code != ExitOK {
		return code
	}
	all := make([]spec.Stats, 0, len(docs))
	for _, d := range docs {
		all = append(all, spec.Collect(d))
	}
	if *format == "json" {
		if err := render.WriteStatsJSON(stdout, all); err != nil {
			fmt.Fprintf(stderr, "mustlint: %v\n", err)
			return ExitRuntime
		}
		return ExitOK
	}
	render.WriteStatsText(stdout, all)
	return ExitOK
}
