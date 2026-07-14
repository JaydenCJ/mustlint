// Keyword-usage rules: per-instance checks on how BCP 14 keywords (and
// their look-alikes) are written.
package rules

import (
	"strconv"

	"github.com/JaydenCJ/mustlint/internal/keyword"
	"github.com/JaydenCJ/mustlint/internal/spec"
)

// lowercaseWatched are the keywords whose lowercase use is flagged under a
// plain RFC 2119 boilerplate. "may", "required", "optional" and
// "recommended" are everyday English and would drown the report in noise.
var lowercaseWatched = map[string]bool{
	"MUST": true, "MUST NOT": true,
	"SHALL": true, "SHALL NOT": true,
	"SHOULD": true, "SHOULD NOT": true,
}

// checkUsage emits mixed-case-keyword, may-not, pseudo-keyword and
// lowercase-keyword findings.
func checkUsage(d *spec.Document) []Finding {
	var out []Finding
	plain2119 := d.Boilerplate != nil && !d.Boilerplate.Has8174

	for _, s := range d.Sentences {
		if s.Heading || s.Boiler {
			continue
		}
		for _, m := range s.Matches {
			switch m.Kind {
			case keyword.KindMayNot:
				// Only capitalized "MAY NOT" signals normative intent;
				// lowercase "may not" is ordinary prose.
				if m.Case == keyword.CaseUpper || m.Case == keyword.CaseMixed {
					out = append(out, finding(s, m.Start, "may-not",
						"%q is not an RFC 2119 keyword and is ambiguous "+
							"(forbidden, or allowed to skip?): use \"MUST NOT\" to forbid, "+
							"or rephrase as \"MAY omit\"", m.Actual))
				}
			case keyword.KindPseudo:
				out = append(out, finding(s, m.Start, "pseudo-keyword",
					"%q reads as normative but has no RFC 2119 meaning: use %s, "+
						"or write it in lowercase for plain prose",
					m.Actual, pseudoReplacement(m.Canonical)))
			case keyword.KindStandard:
				switch m.Case {
				case keyword.CaseMixed:
					out = append(out, finding(s, m.Start, "mixed-case-keyword",
						"mixed-case %q: write %q with both words in capitals so the "+
							"compound keyword is unambiguous", m.Actual, m.Canonical))
				case keyword.CaseLower, keyword.CaseTitle:
					if plain2119 && lowercaseWatched[m.Canonical] {
						out = append(out, finding(s, m.Start, "lowercase-keyword",
							"lowercase %q is ambiguous under a plain RFC 2119 boilerplate: "+
								"capitalize it if it states a requirement, or reword it "+
								"(e.g. \"needs to\") if it does not", m.Actual))
					}
				}
			}
		}
	}
	return out
}

// pseudoReplacement suggests the keyword an author probably meant.
func pseudoReplacement(canonical string) string {
	switch canonical {
	case "WILL":
		return "\"MUST\" (or \"SHALL\")"
	case "WILL NOT", "CANNOT", "FORBIDDEN", "PROHIBITED":
		return "\"MUST NOT\""
	case "MIGHT", "OPTIONALLY":
		return "\"MAY\""
	case "MANDATORY":
		return "\"REQUIRED\" (or \"MUST\")"
	}
	return "\"MUST\""
}

// checkBoilerplate emits the three document-level declaration rules.
func checkBoilerplate(d *spec.Document) []Finding {
	var out []Finding

	// First normative-intent keyword instance anchors missing-boilerplate.
	var firstSent *spec.Sentence
	var firstMatch keyword.Match
	usedUpper := map[string]*struct {
		s *spec.Sentence
		m keyword.Match
	}{}
	lowercaseCount := 0
	var firstLower *spec.Sentence
	var firstLowerMatch keyword.Match

	for _, s := range d.Sentences {
		if s.Heading || s.Boiler {
			continue
		}
		for _, m := range s.Matches {
			if m.Kind != keyword.KindStandard {
				continue
			}
			switch m.Case {
			case keyword.CaseUpper:
				if firstSent == nil {
					firstSent, firstMatch = s, m
				}
				if usedUpper[m.Canonical] == nil {
					usedUpper[m.Canonical] = &struct {
						s *spec.Sentence
						m keyword.Match
					}{s, m}
				}
			case keyword.CaseLower, keyword.CaseTitle:
				if lowercaseWatched[m.Canonical] {
					lowercaseCount++
					if firstLower == nil {
						firstLower, firstLowerMatch = s, m
					}
				}
			}
		}
	}

	if d.Boilerplate == nil {
		if firstSent != nil {
			out = append(out, finding(firstSent, firstMatch.Start, "missing-boilerplate",
				"document uses RFC 2119 keywords (first: %q here) but never declares them: "+
					"add the BCP 14 key-words paragraph citing RFC 2119 and RFC 8174",
				firstMatch.Actual))
		}
		return out
	}

	if !d.Boilerplate.Has8174 && lowercaseCount > 0 {
		noun := "instance exists"
		if lowercaseCount > 1 {
			noun = "instances exist"
		}
		out = append(out, finding(d.Boilerplate.Sentence, 0, "outdated-boilerplate",
			"boilerplate cites RFC 2119 without the RFC 8174 \"all capitals\" clause, "+
				"yet %d lowercase keyword %s (first: %s): adopt the BCP 14 "+
				"boilerplate so only capitalized keywords are normative",
			lowercaseCount, noun, atPos(firstLower, firstLowerMatch.Start)))
	}

	if len(d.Boilerplate.Declared) > 0 {
		declared := map[string]bool{}
		for _, k := range d.Boilerplate.Declared {
			declared[k] = true
		}
		// Report in canonical order for deterministic output.
		for _, k := range keyword.Canonical {
			use := usedUpper[k]
			if use == nil || declared[k] {
				continue
			}
			out = append(out, finding(use.s, use.m.Start, "undeclared-keyword",
				"%q is used but the key-words boilerplate does not declare it: "+
					"add it to the quoted list (or use a declared keyword)", k))
		}
	}
	return out
}

// atPos renders a stable file:line:col reference for cross-finding messages.
func atPos(s *spec.Sentence, rel int) string {
	p := s.PosAt(rel)
	return s.Doc.Path + ":" + strconv.Itoa(p.Line) + ":" + strconv.Itoa(p.Col)
}
