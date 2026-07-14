// Package keyword finds BCP 14 / RFC 2119 requirement keywords — and the
// look-alikes that get authors into trouble — in plain prose. Matching is
// token-based rather than regexp-based so that word boundaries are exact
// (underscores and hyphens never glue words together) and compound keywords
// like "MUST NOT" are matched longest-first across line breaks.
package keyword

import (
	"strings"
	"unicode"
)

// CaseClass describes how a matched keyword phrase is capitalized. Under
// RFC 8174 only CaseUpper is normative; the other classes are what the
// individual rules key off.
type CaseClass int

const (
	// CaseUpper: every word fully capitalized ("MUST NOT").
	CaseUpper CaseClass = iota
	// CaseLower: every word lowercase ("must not").
	CaseLower
	// CaseTitle: sentence-style capitalization ("Must", "Should not").
	CaseTitle
	// CaseMixed: capitalization disagrees inside a compound ("MUST not").
	CaseMixed
)

// Kind separates real keywords from the two trouble classes.
type Kind int

const (
	// KindStandard is a genuine RFC 2119 keyword.
	KindStandard Kind = iota
	// KindMayNot is the infamous "MAY NOT", which RFC 2119 does not define
	// and which reads as either "is allowed not to" or "is forbidden to".
	KindMayNot
	// KindPseudo is an all-caps word that mimics normative force but has no
	// RFC 2119 definition (WILL, MIGHT, CANNOT, MANDATORY, …).
	KindPseudo
)

// Match is one keyword occurrence. Start/End are byte offsets into the text
// passed to Find.
type Match struct {
	Canonical string // normalized uppercase form, e.g. "MUST NOT"
	Actual    string // the text as written, e.g. "Must not"
	Kind      Kind
	Case      CaseClass
	Start     int
	End       int
}

// Canonical lists the eleven BCP 14 keywords in RFC 2119 order.
var Canonical = []string{
	"MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT",
	"SHOULD", "SHOULD NOT", "RECOMMENDED", "NOT RECOMMENDED",
	"MAY", "OPTIONAL",
}

// primaries can be followed by NOT to form a compound keyword.
var primaries = map[string]bool{"MUST": true, "SHALL": true, "SHOULD": true, "MAY": true}

// singles are standalone RFC 2119 keywords.
var singles = map[string]bool{"REQUIRED": true, "RECOMMENDED": true, "OPTIONAL": true}

// pseudo words read as normative but carry no RFC 2119 meaning. They are
// only reported when written in all capitals — lowercase "will" is just
// English. "CAN" is deliberately absent: it collides with the bus protocol.
var pseudo = map[string]bool{
	"WILL": true, "MIGHT": true, "CANNOT": true, "MANDATORY": true,
	"OPTIONALLY": true, "FORBIDDEN": true, "PROHIBITED": true,
}

type token struct {
	text  string
	start int
	end   int
}

// tokenize splits text into maximal letter runs. Digits, underscores and
// punctuation all break words, so "MUST-123" and "spec_must" cannot yield
// false keyword hits.
func tokenize(text string) []token {
	var toks []token
	start := -1
	for i, r := range text {
		if unicode.IsLetter(r) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			toks = append(toks, token{text: text[start:i], start: start, end: i})
			start = -1
		}
	}
	if start >= 0 {
		toks = append(toks, token{text: text[start:], start: start, end: len(text)})
	}
	return toks
}

// gapIsSpace reports whether only whitespace separates two tokens, so
// "must, not" is two words while "MUST\nNOT" is still one compound keyword.
func gapIsSpace(text string, a, b token) bool {
	return strings.TrimSpace(text[a.end:b.start]) == ""
}

func wordCase(w string) CaseClass {
	upper := strings.ToUpper(w)
	lower := strings.ToLower(w)
	switch {
	case w == upper:
		return CaseUpper
	case w == lower:
		return CaseLower
	case w == upper[:1]+lower[1:]:
		return CaseTitle
	default:
		return CaseMixed
	}
}

// phraseCase combines the word classes of a compound. Any disagreement
// between an uppercase word and a non-uppercase word is CaseMixed — that is
// exactly the "MUST not" bug class.
func phraseCase(words []string) CaseClass {
	first := wordCase(words[0])
	if len(words) == 1 {
		return first
	}
	second := wordCase(words[1])
	switch {
	case first == CaseUpper && second == CaseUpper:
		return CaseUpper
	case first == CaseLower && second == CaseLower:
		return CaseLower
	case first == CaseTitle && second == CaseLower:
		return CaseTitle
	default:
		return CaseMixed
	}
}

// Find returns every keyword occurrence in text, in order. Compounds are
// consumed greedily: "MUST NOT" yields one match, never MUST plus a stray
// NOT. Pseudo-keywords are matched only in all capitals.
func Find(text string) []Match {
	toks := tokenize(text)
	var out []Match
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		up := strings.ToUpper(t.text)

		next := func() (token, bool) {
			if i+1 < len(toks) && gapIsSpace(text, t, toks[i+1]) {
				return toks[i+1], true
			}
			return token{}, false
		}

		switch {
		case primaries[up]:
			if nt, ok := next(); ok && strings.ToUpper(nt.text) == "NOT" {
				m := compound(text, t, nt)
				if up == "MAY" {
					m.Kind = KindMayNot
				}
				out = append(out, m)
				i++
				continue
			}
			if c := phraseCase([]string{t.text}); c != CaseMixed {
				out = append(out, Match{
					Canonical: up, Actual: t.text, Kind: KindStandard,
					Case: c, Start: t.start, End: t.end,
				})
			}
		case singles[up]:
			if c := phraseCase([]string{t.text}); c != CaseMixed {
				out = append(out, Match{
					Canonical: up, Actual: t.text, Kind: KindStandard,
					Case: c, Start: t.start, End: t.end,
				})
			}
		case up == "NOT":
			if nt, ok := next(); ok && strings.ToUpper(nt.text) == "RECOMMENDED" {
				out = append(out, compound(text, t, nt))
				i++
			}
		case pseudo[up] && t.text == up:
			m := Match{
				Canonical: up, Actual: t.text, Kind: KindPseudo,
				Case: CaseUpper, Start: t.start, End: t.end,
			}
			if up == "WILL" {
				if nt, ok := next(); ok && nt.text == "NOT" {
					m.Canonical = "WILL NOT"
					m.Actual = text[t.start:nt.end]
					m.End = nt.end
					i++
				}
			}
			out = append(out, m)
		}
	}
	return out
}

// compound builds a two-word match from adjacent tokens.
func compound(text string, a, b token) Match {
	return Match{
		Canonical: strings.ToUpper(a.text) + " " + strings.ToUpper(b.text),
		Actual:    text[a.start:b.end],
		Kind:      KindStandard,
		Case:      phraseCase([]string{a.text, b.text}),
		Start:     a.start,
		End:       b.end,
	}
}
