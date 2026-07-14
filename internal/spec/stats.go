// Keyword inventory for the `mustlint stats` subcommand.
package spec

import "github.com/JaydenCJ/mustlint/internal/keyword"

// Stats is the per-file keyword inventory. Keywords counts only uppercase
// (normative) instances of the eleven BCP 14 keywords.
type Stats struct {
	File      string
	Keywords  map[string]int
	Normative int // sentences stating a requirement
	Defined   int // requirement-ID definition sites
}

// Collect inventories one document.
func Collect(d *Document) Stats {
	st := Stats{File: d.Path, Keywords: map[string]int{}}
	for _, s := range d.Sentences {
		if s.Boiler {
			continue
		}
		if !s.Heading {
			for _, m := range s.Matches {
				if m.Kind == keyword.KindStandard && m.Case == keyword.CaseUpper {
					st.Keywords[m.Canonical]++
				}
			}
		}
		if s.Normative() {
			st.Normative++
		}
		// Definition convention: first ID of a heading or normative sentence.
		if len(s.IDs) > 0 && (s.Heading || s.Normative()) {
			st.Defined++
		}
	}
	return st
}
