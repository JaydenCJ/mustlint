// The check and rules subcommands.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"

	"github.com/JaydenCJ/mustlint/internal/render"
	"github.com/JaydenCJ/mustlint/internal/rules"
	"github.com/JaydenCJ/mustlint/internal/spec"
)

func runCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "text", "output format: text, json, or github")
	failOn := fs.String("fail-on", "warning", "exit 1 at this severity: error, warning, info, or never")
	requireIDs := fs.Bool("require-ids", false, "require a requirement ID on every normative statement")
	idPattern := fs.String("id-pattern", "", "requirement-ID regexp (default: "+spec.DefaultIDPattern+")")
	quiet := fs.Bool("quiet", false, "findings only, no summary line")
	var disabled multiFlag
	fs.Var(&disabled, "disable", "rule ID to switch off (repeatable)")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return ExitOK // an explicit -h/--help is not a usage error
		}
		return ExitUsage
	}

	outFormat, ok := render.ParseFormat(*format)
	if !ok {
		fmt.Fprintf(stderr, "mustlint: unknown --format %q (want text, json, or github)\n", *format)
		return ExitUsage
	}
	threshold, never, ok := parseFailOn(*failOn)
	if !ok {
		fmt.Fprintf(stderr, "mustlint: unknown --fail-on %q (want error, warning, info, or never)\n", *failOn)
		return ExitUsage
	}
	cfg := rules.Config{Disabled: map[string]bool{}, RequireIDs: *requireIDs}
	for _, r := range disabled {
		if !rules.KnownRule(r) {
			fmt.Fprintf(stderr, "mustlint: --disable %q names no rule (try `mustlint rules`)\n", r)
			return ExitUsage
		}
		cfg.Disabled[r] = true
	}
	if *idPattern != "" {
		re, err := regexp.Compile(*idPattern)
		if err != nil {
			fmt.Fprintf(stderr, "mustlint: bad --id-pattern: %v\n", err)
			return ExitUsage
		}
		cfg.IDPattern = re
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "mustlint: no files given (try `mustlint check docs/`)")
		return ExitUsage
	}

	docs, code := loadDocs(fs.Args(), cfg.IDPattern, stderr)
	if code != ExitOK {
		return code
	}

	findings := rules.Check(docs, cfg)
	switch outFormat {
	case render.JSON:
		if err := render.WriteJSON(stdout, findings, len(docs)); err != nil {
			fmt.Fprintf(stderr, "mustlint: %v\n", err)
			return ExitRuntime
		}
	case render.GitHub:
		render.WriteGitHub(stdout, findings)
	default:
		render.WriteText(stdout, findings, len(docs), *quiet)
	}

	if never {
		return ExitOK
	}
	for _, f := range findings {
		if f.Severity >= threshold {
			return ExitFail
		}
	}
	return ExitOK
}

// loadDocs reads and analyzes every requested file.
func loadDocs(paths []string, idRe *regexp.Regexp, stderr io.Writer) ([]*spec.Document, int) {
	files, err := collectFiles(paths)
	if err != nil {
		fmt.Fprintf(stderr, "mustlint: %v\n", err)
		return nil, ExitRuntime
	}
	if len(files) == 0 {
		fmt.Fprintln(stderr, "mustlint: no Markdown files found under the given paths")
		return nil, ExitRuntime
	}
	var docs []*spec.Document
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(stderr, "mustlint: %v\n", err)
			return nil, ExitRuntime
		}
		docs = append(docs, spec.Build(f, string(content), idRe))
	}
	return docs, ExitOK
}

func runRules(stdout io.Writer) int {
	fmt.Fprintf(stdout, "mustlint rules (%d):\n\n", len(rules.All))
	fmt.Fprintf(stdout, "  %-21s  %-8s  %s\n", "id", "severity", "summary")
	for _, r := range rules.All {
		fmt.Fprintf(stdout, "  %-21s  %-8s  %s\n", r.ID, r.Severity, r.Summary)
	}
	fmt.Fprintf(stdout, "\nDisable any rule with --disable RULE or an inline "+
		"<!-- mustlint-disable RULE --> comment.\n")
	return ExitOK
}
