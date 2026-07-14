// Ambiguity rule: vague qualifiers have no place inside a normative
// statement — "the server MUST respond in a timely manner" is a requirement
// nobody can test. The term list mirrors what RFC editors and protocol
// reviewers flag by hand.
package rules

import (
	"regexp"
	"sort"
	"strings"

	"github.com/JaydenCJ/mustlint/internal/spec"
)

// ambiguousTerms maps each vague phrase to a tailored hint. An empty hint
// falls back to the generic message.
var ambiguousTerms = map[string]string{
	"as appropriate":      "state the criteria that make it appropriate",
	"if appropriate":      "state the criteria that make it appropriate",
	"where appropriate":   "state the criteria that make it appropriate",
	"as needed":           "state when it is needed",
	"if needed":           "state when it is needed",
	"as necessary":        "state when it is necessary",
	"if necessary":        "state when it is necessary",
	"where applicable":    "enumerate the cases where it applies",
	"as applicable":       "enumerate the cases where it applies",
	"if possible":         "make it SHOULD with an explicit exception, or drop it",
	"where possible":      "make it SHOULD with an explicit exception, or drop it",
	"as soon as possible": "give a concrete deadline or timeout",
	"in a timely manner":  "give a concrete deadline or timeout",
	"timely":              "give a concrete deadline or timeout",
	"best effort":         "define what effort is required and what may be dropped",
	"best-effort":         "define what effort is required and what may be dropped",
	"reasonable":          "give the concrete bound you mean",
	"reasonably":          "give the concrete bound you mean",
	"sufficient":          "quantify how much is sufficient",
	"sufficiently":        "quantify how much is sufficient",
	"adequate":            "quantify how much is adequate",
	"adequately":          "quantify how much is adequate",
	"appropriately":       "describe the exact required behavior",
	"properly":            "describe the exact required behavior",
	"gracefully":          "describe the exact required behavior",
	"generally":           "requirements hold always or under stated conditions",
	"usually":             "requirements hold always or under stated conditions",
	"normally":            "requirements hold always or under stated conditions",
	"typically":           "requirements hold always or under stated conditions",
	"and/or":              "write \"A or B, or both\"",
	"etc.":                "enumerate the full list or define an extension point",
	"etc":                 "enumerate the full list or define an extension point",
}

var reAmbiguous = buildAmbiguousRe()

// buildAmbiguousRe compiles one alternation, longest phrase first so
// "as soon as possible" wins over any shorter overlap.
func buildAmbiguousRe() *regexp.Regexp {
	terms := make([]string, 0, len(ambiguousTerms))
	for t := range ambiguousTerms {
		terms = append(terms, t)
	}
	sort.Slice(terms, func(i, j int) bool {
		if len(terms[i]) != len(terms[j]) {
			return len(terms[i]) > len(terms[j])
		}
		return terms[i] < terms[j]
	})
	quoted := make([]string, len(terms))
	for i, t := range terms {
		quoted[i] = regexp.QuoteMeta(t)
	}
	// \b works on the leading letter of every term; the trailing boundary
	// needs care because "etc." ends in punctuation.
	return regexp.MustCompile(`(?i)\b(` + strings.Join(quoted, "|") + `)([^a-zA-Z-]|$)`)
}

// checkAmbiguity flags vague qualifiers, but only inside normative
// sentences — descriptive prose is allowed to hedge.
func checkAmbiguity(d *spec.Document) []Finding {
	var out []Finding
	for _, s := range d.Sentences {
		if !s.Normative() {
			continue
		}
		text := s.Text
		off := 0
		for {
			loc := reAmbiguous.FindStringSubmatchIndex(text[off:])
			if loc == nil {
				break
			}
			start, end := off+loc[2], off+loc[3]
			term := strings.ToLower(text[start:end])
			hint := ambiguousTerms[term]
			if hint == "" {
				hint = "state the exact behavior or criteria"
			}
			out = append(out, finding(s, start, "ambiguous-term",
				"%q leaves this requirement open to interpretation: %s",
				text[start:end], hint))
			off = end
		}
	}
	return out
}
