// Package cli implements the mustlint command-line interface. Run takes
// argv and two writers and returns an exit code, so the entire surface is
// testable in-process without building a binary.
package cli

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/JaydenCJ/mustlint/internal/rules"
	"github.com/JaydenCJ/mustlint/internal/version"
)

// Exit codes, documented in the README. Scripts branch on them.
const (
	ExitOK      = 0 // clean, or findings below the --fail-on threshold
	ExitFail    = 1 // findings at or above the --fail-on threshold
	ExitUsage   = 2 // bad flags or arguments
	ExitRuntime = 3 // unreadable file, bad pattern at runtime, …
)

// Run dispatches argv and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return ExitUsage
	}
	switch args[0] {
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "stats":
		return runStats(args[1:], stdout, stderr)
	case "rules":
		return runRules(stdout)
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "mustlint %s\n", version.Version)
		return ExitOK
	case "help", "--help", "-h":
		usage(stdout)
		return ExitOK
	default:
		if strings.HasPrefix(args[0], "-") {
			fmt.Fprintf(stderr, "mustlint: unknown flag %q before a subcommand\n\n", args[0])
			usage(stderr)
			return ExitUsage
		}
		// Bare paths: treat as `check <paths>`.
		return runCheck(args, stdout, stderr)
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `mustlint — lint RFC 2119 requirement language in Markdown specs

Usage:
  mustlint check [flags] <file|dir>...   lint specs (default subcommand)
  mustlint stats [flags] <file|dir>...   keyword usage inventory
  mustlint rules                         list every rule with its severity
  mustlint version                       print the version

Check flags:
  --format text|json|github   output format               (default text)
  --fail-on error|warning|info|never
                              exit 1 at this severity     (default warning)
  --disable RULE              switch a rule off           (repeatable)
  --require-ids               every requirement needs an ID (enables missing-id)
  --id-pattern REGEXP         requirement-ID pattern      (default REQ-1 style)
  --quiet                     findings only, no summary line

Stats flags:
  --format text|json          output format               (default text)

Exit codes: 0 ok, 1 findings at/above --fail-on, 2 usage error, 3 runtime error.
`)
}

// multiFlag is a repeatable string flag.
type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

// collectFiles expands the path arguments: files are taken as-is, and
// directories are walked recursively for .md / .markdown files, skipping
// hidden directories. The result is sorted for deterministic runs.
func collectFiles(paths []string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			add(p)
			continue
		}
		err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := d.Name()
			if d.IsDir() {
				if path != p && strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
				return nil
			}
			ext := strings.ToLower(filepath.Ext(name))
			if ext == ".md" || ext == ".markdown" {
				add(path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(out)
	return out, nil
}

// parseFailOn maps the --fail-on flag onto a severity threshold; the bool
// pair is (never, ok).
func parseFailOn(v string) (rules.Severity, bool, bool) {
	if v == "never" {
		return rules.Info, true, true
	}
	sev, ok := rules.ParseSeverity(v)
	return sev, false, ok
}
