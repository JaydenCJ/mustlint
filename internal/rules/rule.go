// Package rules implements mustlint's twelve checks and the engine that
// runs them over a corpus of analyzed documents, honoring inline
// suppression directives and per-rule disables.
package rules

import (
	"fmt"
	"regexp"
	"sort"

	"github.com/JaydenCJ/mustlint/internal/mdscan"
	"github.com/JaydenCJ/mustlint/internal/spec"
)

// Severity orders findings; Error is the most severe.
type Severity int

const (
	Info Severity = iota
	Warning
	Error
)

// String returns the lowercase severity name used in every output format.
func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warning:
		return "warning"
	default:
		return "info"
	}
}

// ParseSeverity maps a --fail-on value; ok is false for unknown names.
func ParseSeverity(name string) (Severity, bool) {
	switch name {
	case "error":
		return Error, true
	case "warning":
		return Warning, true
	case "info":
		return Info, true
	}
	return Info, false
}

// Finding is one reported problem at an exact source position.
type Finding struct {
	File     string
	Line     int
	Col      int
	Rule     string
	Severity Severity
	Message  string
}

// Rule describes one check for the `mustlint rules` listing.
type Rule struct {
	ID         string
	Severity   Severity
	DefaultOff bool
	Summary    string
}

// All lists every rule in documentation order (boilerplate, usage, IDs,
// duplication, ambiguity).
var All = []Rule{
	{"missing-boilerplate", Warning, false,
		"RFC 2119 keywords are used but no BCP 14 key-words paragraph declares them"},
	{"outdated-boilerplate", Warning, false,
		"boilerplate cites RFC 2119 without the RFC 8174 all-capitals clause while lowercase keywords appear"},
	{"undeclared-keyword", Info, false,
		"a keyword is used that the boilerplate's quoted keyword list does not declare"},
	{"lowercase-keyword", Info, false,
		"lowercase must/shall/should is ambiguous under a plain RFC 2119 boilerplate"},
	{"mixed-case-keyword", Error, false,
		"compound keyword with inconsistent capitals, e.g. \"MUST not\""},
	{"may-not", Error, false,
		"\"MAY NOT\" is undefined and ambiguous: forbidden, or allowed to skip?"},
	{"pseudo-keyword", Warning, false,
		"all-caps word that mimics normative force but has no RFC 2119 meaning (WILL, CANNOT, …)"},
	{"missing-id", Warning, true,
		"normative statement carries no requirement ID (enable with --require-ids)"},
	{"duplicate-id", Error, false,
		"the same requirement ID is defined more than once"},
	{"id-gap", Info, false,
		"numbering gap inside a requirement-ID series"},
	{"duplicate-requirement", Warning, false,
		"two normative statements are identical after normalization"},
	{"ambiguous-term", Warning, false,
		"vague qualifier inside a normative statement (as appropriate, best effort, …)"},
}

// severityOf resolves a rule ID to its severity.
func severityOf(id string) Severity {
	for _, r := range All {
		if r.ID == id {
			return r.Severity
		}
	}
	return Warning
}

// KnownRule reports whether id names a rule.
func KnownRule(id string) bool {
	for _, r := range All {
		if r.ID == id {
			return true
		}
	}
	return false
}

// Config controls a Check run.
type Config struct {
	Disabled   map[string]bool // rule IDs switched off for the whole run
	RequireIDs bool            // enables the missing-id rule
	IDPattern  *regexp.Regexp  // nil selects spec.DefaultIDPattern
}

// Check runs every applicable rule over the corpus and returns findings
// sorted by file, line, column, then rule ID. Inline directives and
// Config.Disabled are already applied.
func Check(docs []*spec.Document, cfg Config) []Finding {
	var out []Finding
	for _, d := range docs {
		out = append(out, checkBoilerplate(d)...)
		out = append(out, checkUsage(d)...)
		out = append(out, checkAmbiguity(d)...)
		if cfg.RequireIDs {
			out = append(out, checkMissingIDs(d)...)
		}
	}
	out = append(out, checkIDs(docs)...)
	out = append(out, checkDuplicates(docs)...)

	sup := map[string]*suppressor{}
	for _, d := range docs {
		sup[d.Path] = newSuppressor(d.Directives)
	}
	kept := out[:0]
	for _, f := range out {
		if cfg.Disabled[f.Rule] {
			continue
		}
		if s := sup[f.File]; s != nil && s.suppressed(f.Line, f.Rule) {
			continue
		}
		kept = append(kept, f)
	}
	out = kept

	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Col != b.Col {
			return a.Col < b.Col
		}
		return a.Rule < b.Rule
	})
	return out
}

// finding builds a Finding anchored at a sentence-relative offset.
func finding(s *spec.Sentence, rel int, rule, format string, args ...interface{}) Finding {
	p := s.PosAt(rel)
	return Finding{
		File:     s.Doc.Path,
		Line:     p.Line,
		Col:      p.Col,
		Rule:     rule,
		Severity: severityOf(rule),
		Message:  fmt.Sprintf(format, args...),
	}
}

// suppressor answers "is (line, rule) muted by an inline directive?".
type suppressor struct {
	nextLine map[int][]string // target line → rules ([] = all)
	ranges   []supRange
}

type supRange struct {
	from, to int // inclusive line range; to == maxInt while open
	rules    []string
}

const maxInt = int(^uint(0) >> 1)

func newSuppressor(dirs []mdscan.Directive) *suppressor {
	s := &suppressor{nextLine: map[int][]string{}}
	for _, d := range dirs {
		switch d.Kind {
		case mdscan.DirDisableNextLine:
			target := d.Line + 1
			cur, exists := s.nextLine[target]
			if len(d.Rules) == 0 || (exists && len(cur) == 0) {
				s.nextLine[target] = []string{} // empty list means "all rules"
			} else {
				s.nextLine[target] = append(cur, d.Rules...)
			}
		case mdscan.DirDisable:
			s.ranges = append(s.ranges, supRange{from: d.Line, to: maxInt, rules: d.Rules})
		case mdscan.DirEnable:
			for i := range s.ranges {
				r := &s.ranges[i]
				if r.to == maxInt && matchesEnable(r.rules, d.Rules) {
					r.to = d.Line - 1
				}
			}
		}
	}
	return s
}

// matchesEnable decides whether an enable directive closes an open range:
// a bare enable closes everything; a scoped enable closes bare ranges and
// ranges that mention any of the same rules.
func matchesEnable(open, enable []string) bool {
	if len(enable) == 0 || len(open) == 0 {
		return true
	}
	for _, o := range open {
		for _, e := range enable {
			if o == e {
				return true
			}
		}
	}
	return false
}

func (s *suppressor) suppressed(line int, rule string) bool {
	if rules, ok := s.nextLine[line]; ok && ruleListed(rules, rule) {
		return true
	}
	for _, r := range s.ranges {
		if line >= r.from && line <= r.to && ruleListed(r.rules, rule) {
			return true
		}
	}
	return false
}

func ruleListed(rules []string, rule string) bool {
	if len(rules) == 0 {
		return true
	}
	for _, r := range rules {
		if r == rule {
			return true
		}
	}
	return false
}
