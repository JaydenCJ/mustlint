// Requirement-ID rules: uniqueness, coverage, and series continuity.
//
// Definition convention (documented in docs/rules.md): the FIRST ID inside
// a heading or a normative sentence defines that requirement; any further
// IDs in the same sentence are cross-references ("REQ-9 supersedes REQ-2").
// IDs in plain prose are always references.
package rules

import (
	"fmt"
	"sort"

	"github.com/JaydenCJ/mustlint/internal/spec"
)

// defSite is one place a requirement ID is defined.
type defSite struct {
	sentence *spec.Sentence
	ref      spec.IDRef
}

// definitions walks the corpus in document order and returns every ID
// definition site, preserving order for deterministic reporting.
func definitions(docs []*spec.Document) []defSite {
	var out []defSite
	for _, d := range docs {
		for _, s := range d.Sentences {
			if s.Boiler || len(s.IDs) == 0 {
				continue
			}
			if s.Heading || s.Normative() {
				out = append(out, defSite{sentence: s, ref: s.IDs[0]})
			}
		}
	}
	return out
}

// checkMissingIDs flags normative sentences with neither an inline ID nor
// an ID inherited from their section heading. Only run under --require-ids.
func checkMissingIDs(d *spec.Document) []Finding {
	var out []Finding
	for _, s := range d.Sentences {
		if !s.Normative() || len(s.IDs) > 0 || s.SectionID != "" {
			continue
		}
		out = append(out, finding(s, 0, "missing-id",
			"normative statement has no requirement ID (inline like \"REQ-1:\" "+
				"or inherited from a section heading): stable IDs keep reviews, "+
				"tests, and cross-references honest"))
	}
	return out
}

// checkIDs emits duplicate-id and id-gap findings across the whole corpus,
// so collisions between files are caught too.
func checkIDs(docs []*spec.Document) []Finding {
	var out []Finding
	defs := definitions(docs)

	first := map[string]defSite{}
	for _, def := range defs {
		prev, seen := first[def.ref.ID]
		if !seen {
			first[def.ref.ID] = def
			continue
		}
		out = append(out, finding(def.sentence, def.ref.Start, "duplicate-id",
			"requirement ID %s is already defined at %s: give each requirement "+
				"a unique ID", def.ref.ID, atPos(prev.sentence, prev.ref.Start)))
	}

	out = append(out, checkGaps(defs)...)
	return out
}

// checkGaps reports missing numbers inside each ID series. Renumbering to
// close a gap breaks external references, so the finding suggests retiring
// IDs explicitly instead; it is informational by design.
func checkGaps(defs []defSite) []Finding {
	type numSite struct {
		num  int
		site defSite
	}
	series := map[string][]numSite{}
	var order []string
	for _, def := range defs {
		if def.ref.Num < 0 {
			continue // non-numeric ID under a custom pattern
		}
		if _, ok := series[def.ref.Series]; !ok {
			order = append(order, def.ref.Series)
		}
		series[def.ref.Series] = append(series[def.ref.Series], numSite{def.ref.Num, def})
	}
	sort.Strings(order)

	var out []Finding
	for _, name := range order {
		sites := series[name]
		if len(sites) < 2 {
			continue
		}
		// Deduplicate numbers (duplicates are reported separately) and sort.
		byNum := map[int]defSite{}
		var nums []int
		for _, ns := range sites {
			if _, ok := byNum[ns.num]; !ok {
				byNum[ns.num] = ns.site
				nums = append(nums, ns.num)
			}
		}
		sort.Ints(nums)
		for i := 1; i < len(nums); i++ {
			lo, hi := nums[i-1], nums[i]
			if hi-lo <= 1 {
				continue
			}
			site := byNum[hi]
			out = append(out, finding(site.sentence, site.ref.Start, "id-gap",
				"series %s jumps from %s-%d to %s-%d (%s): if requirements were "+
					"removed, retire their IDs explicitly rather than leaving "+
					"silent holes", name, name, lo, name, hi, missingList(name, lo, hi)))
		}
	}
	return out
}

// missingList names the absent IDs, abbreviating long runs.
func missingList(series string, lo, hi int) string {
	n := hi - lo - 1
	if n == 1 {
		return fmt.Sprintf("%s-%d missing", series, lo+1)
	}
	if n <= 3 {
		s := ""
		for k := lo + 1; k < hi; k++ {
			if s != "" {
				s += ", "
			}
			s += fmt.Sprintf("%s-%d", series, k)
		}
		return s + " missing"
	}
	return fmt.Sprintf("%s-%d through %s-%d missing, %d IDs", series, lo+1, series, hi-1, n)
}

// checkDuplicates flags normative sentences that are identical after
// normalization (IDs stripped, case folded, punctuation removed). Six-word
// minimum: shorter statements ("Servers MUST support TLS") repeat
// legitimately in overview tables.
func checkDuplicates(docs []*spec.Document) []Finding {
	const minWords = 6
	type site struct {
		s *spec.Sentence
	}
	first := map[string]site{}
	var out []Finding
	for _, d := range docs {
		for _, s := range d.Sentences {
			if !s.Normative() {
				continue
			}
			norm := s.Normalized()
			if countWords(norm) < minWords {
				continue
			}
			prev, seen := first[norm]
			if !seen {
				first[norm] = site{s}
				continue
			}
			out = append(out, finding(s, 0, "duplicate-requirement",
				"normative statement duplicates %s (identical after normalization): "+
					"state each requirement once and cross-reference it elsewhere",
				atPos(prev.s, 0)))
		}
	}
	return out
}

func countWords(s string) int {
	n := 0
	inWord := false
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			inWord = false
			continue
		}
		if !inWord {
			n++
			inWord = true
		}
	}
	return n
}
