// Package spec assembles the analyzed document model that rules run
// against: scrubbed prose split into sentences with exact source positions,
// keyword matches, requirement IDs (with section-heading inheritance), and
// the BCP 14 boilerplate declaration.
package spec

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/JaydenCJ/mustlint/internal/keyword"
	"github.com/JaydenCJ/mustlint/internal/mdscan"
)

// DefaultIDPattern matches conventional requirement IDs such as REQ-1,
// SEC-042 or REQ-AUTH-17: dash-joined uppercase tokens ending in a number.
const DefaultIDPattern = `\b[A-Z][A-Z0-9]{1,11}(?:-[A-Z0-9]{1,11})*-[0-9]{1,6}\b`

// idStoplist holds prefixes that look like requirement series but are
// citations of external documents or algorithms. It applies only to the
// default pattern; a user-supplied --id-pattern is trusted verbatim.
var idStoplist = map[string]bool{
	"RFC": true, "BCP": true, "STD": true, "FYI": true, "ISO": true,
	"IEC": true, "IEEE": true, "ECMA": true, "ANSI": true, "NIST": true,
	"FIPS": true, "CVE": true, "CWE": true, "UTF": true, "SHA": true,
	"MD": true, "AES": true, "RSA": true, "TLS": true, "SSL": true,
	"PEP": true, "DOI": true, "EN": true,
}

var defaultIDRe = regexp.MustCompile(DefaultIDPattern)

// Pos is a 1-based source position; Col counts runes on the raw line.
type Pos struct {
	Line int
	Col  int
}

// IDRef is one requirement-ID occurrence inside a sentence.
type IDRef struct {
	ID     string
	Series string // "REQ-AUTH" for REQ-AUTH-17
	Num    int    // 17 for REQ-AUTH-17 (-1 if non-numeric under a custom pattern)
	Start  int    // byte offset into Sentence.Text
}

// span maps a byte range of block text back onto a source line.
type span struct {
	start    int // byte offset into the block text
	line     int // 1-based source line
	rawStart int // byte offset into the raw line where the span begins
}

// Sentence is the atomic unit rules inspect.
type Sentence struct {
	Doc       *Document
	Text      string // scrubbed prose (may contain \n from joined lines)
	Heading   bool
	TableRow  bool
	Boiler    bool   // this sentence is the BCP 14 boilerplate
	SectionID string // requirement ID inherited from the nearest heading
	Matches   []keyword.Match
	IDs       []IDRef

	spans []span // position mapping, offsets relative to Text
}

// Boilerplate captures what the key-words paragraph declares.
type Boilerplate struct {
	Sentence *Sentence
	Has8174  bool     // cites RFC 8174 / BCP 14 "all capitals" language
	Declared []string // canonical keywords quoted in the boilerplate; nil if none
}

// Document is one analyzed Markdown file.
type Document struct {
	Path        string
	Lines       []string
	Sentences   []*Sentence
	Directives  []mdscan.Directive
	Boilerplate *Boilerplate // nil when the document declares nothing
}

// Build scans and analyzes one file. idRe selects the requirement-ID
// pattern; pass nil for the default (which applies the citation stoplist).
func Build(path, content string, idRe *regexp.Regexp) *Document {
	md := mdscan.Scan(path, content)
	doc := &Document{Path: path, Lines: md.Lines, Directives: md.Directives}

	useStoplist := idRe == nil
	if idRe == nil {
		idRe = defaultIDRe
	}

	sectionID := ""
	for _, blk := range md.Blocks {
		text, spans := joinBlock(blk)
		for _, r := range splitSentences(text) {
			s := &Sentence{
				Doc:      doc,
				Text:     text[r[0]:r[1]],
				Heading:  blk.Kind == mdscan.Heading,
				TableRow: blk.Kind == mdscan.TableRow,
				spans:    offsetSpans(spans, r[0]),
			}
			s.Matches = keyword.Find(s.Text)
			s.IDs = findIDs(s.Text, idRe, useStoplist)
			if s.Heading {
				if len(s.IDs) > 0 {
					sectionID = s.IDs[0].ID
				} else {
					sectionID = "" // a plain heading opens a new section
				}
			} else {
				s.SectionID = sectionID
			}
			doc.Sentences = append(doc.Sentences, s)
		}
	}
	doc.Boilerplate = findBoilerplate(doc)
	return doc
}

// joinBlock concatenates a block's segments with \n and records the span map.
func joinBlock(blk mdscan.Block) (string, []span) {
	var b strings.Builder
	var spans []span
	for i, seg := range blk.Segments {
		if i > 0 {
			b.WriteByte('\n')
		}
		spans = append(spans, span{start: b.Len(), line: seg.Line, rawStart: 0})
		b.WriteString(seg.Text)
	}
	return b.String(), spans
}

// offsetSpans rebases a block span map onto a sentence starting at base.
func offsetSpans(spans []span, base int) []span {
	var out []span
	for _, sp := range spans {
		if sp.start <= base {
			out = []span{{start: 0, line: sp.line, rawStart: sp.rawStart + base - sp.start}}
			continue
		}
		out = append(out, span{start: sp.start - base, line: sp.line, rawStart: sp.rawStart})
	}
	return out
}

// PosAt converts a byte offset into this sentence's text to a source
// position. Columns are rune counts over the raw line, 1-based, so they are
// what editors and CI annotations expect.
func (s *Sentence) PosAt(rel int) Pos {
	sp := s.spans[0]
	for _, cand := range s.spans[1:] {
		if cand.start > rel {
			break
		}
		sp = cand
	}
	byteCol := sp.rawStart + (rel - sp.start)
	raw := ""
	if sp.line-1 < len(s.Doc.Lines) {
		raw = s.Doc.Lines[sp.line-1]
	}
	if byteCol > len(raw) {
		byteCol = len(raw)
	}
	return Pos{Line: sp.line, Col: utf8.RuneCountInString(raw[:byteCol]) + 1}
}

// Pos is the position of the sentence's first byte.
func (s *Sentence) Pos() Pos { return s.PosAt(0) }

// Normative reports whether this sentence states a requirement: it carries
// at least one real (or MAY NOT) keyword written normatively — all capitals,
// or mixed capitals, which is a bug but plainly normative intent. Headings
// and the boilerplate itself never count.
func (s *Sentence) Normative() bool {
	if s.Heading || s.Boiler {
		return false
	}
	for _, m := range s.Matches {
		if m.Kind == keyword.KindPseudo {
			continue
		}
		if m.Case == keyword.CaseUpper || m.Case == keyword.CaseMixed {
			return true
		}
	}
	return false
}

// Normalized returns a canonical form of the sentence for duplicate
// detection: IDs stripped, case folded, punctuation removed, whitespace
// collapsed.
func (s *Sentence) Normalized() string {
	text := s.Text
	// Blank the ID tokens so "REQ-1: X MUST Y" duplicates "REQ-2: X MUST Y".
	b := []byte(text)
	for _, id := range s.IDs {
		for p := id.Start; p < id.Start+len(id.ID) && p < len(b); p++ {
			b[p] = ' '
		}
	}
	var words []string
	cur := strings.Builder{}
	for _, r := range strings.ToLower(string(b)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			cur.WriteRune(r)
			continue
		}
		if cur.Len() > 0 {
			words = append(words, cur.String())
			cur.Reset()
		}
	}
	if cur.Len() > 0 {
		words = append(words, cur.String())
	}
	return strings.Join(words, " ")
}

// findIDs extracts requirement IDs from sentence text.
func findIDs(text string, re *regexp.Regexp, useStoplist bool) []IDRef {
	var out []IDRef
	for _, loc := range re.FindAllStringIndex(text, -1) {
		id := text[loc[0]:loc[1]]
		series, num := SplitID(id)
		if useStoplist && idStoplist[series] {
			continue
		}
		out = append(out, IDRef{ID: id, Series: series, Num: num, Start: loc[0]})
	}
	return out
}

// SplitID separates a requirement ID into its series prefix and number.
// REQ-AUTH-17 → ("REQ-AUTH", 17). IDs without a trailing number (possible
// under a custom pattern) return the whole ID as series and -1.
func SplitID(id string) (string, int) {
	idx := strings.LastIndex(id, "-")
	if idx < 0 {
		return id, -1
	}
	n, err := strconv.Atoi(id[idx+1:])
	if err != nil {
		return id, -1
	}
	return id[:idx], n
}

// findBoilerplate locates the key-words conformance sentence and parses
// what it declares.
func findBoilerplate(doc *Document) *Boilerplate {
	var found *Sentence
	for _, s := range doc.Sentences {
		if s.Heading {
			continue
		}
		low := strings.ToLower(s.Text)
		if !strings.Contains(low, "key words") && !strings.Contains(low, "keywords") {
			continue
		}
		if !strings.Contains(low, "2119") && !strings.Contains(low, "bcp 14") &&
			!strings.Contains(low, "bcp14") {
			continue
		}
		if !strings.Contains(low, "interpret") {
			continue
		}
		found = s
		break
	}
	if found == nil {
		return nil
	}
	found.Boiler = true
	bp := &Boilerplate{Sentence: found}

	low := strings.ToLower(found.Text)
	if strings.Contains(low, "8174") || strings.Contains(low, "all capitals") {
		bp.Has8174 = true
	} else {
		// The 8174 citation may sit in an adjacent sentence of the same
		// paragraph ("… when, and only when, they appear in all capitals.").
		for _, s := range doc.Sentences {
			l := strings.ToLower(s.Text)
			if strings.Contains(l, "8174") || strings.Contains(l, "all capitals") {
				bp.Has8174 = true
				break
			}
		}
	}
	bp.Declared = declaredKeywords(found.Text)
	return bp
}

var reQuoted = regexp.MustCompile(`["“]([A-Z]+(?: [A-Z]+)?)["”]`)

// declaredKeywords parses the quoted keyword list out of the boilerplate,
// accepting straight or curly quotes. nil means the boilerplate does not
// enumerate keywords, so nothing can be checked against it.
func declaredKeywords(text string) []string {
	canon := map[string]bool{}
	for _, k := range keyword.Canonical {
		canon[k] = true
	}
	var out []string
	seen := map[string]bool{}
	for _, m := range reQuoted.FindAllStringSubmatch(text, -1) {
		k := m[1]
		if canon[k] && !seen[k] {
			out = append(out, k)
			seen[k] = true
		}
	}
	return out
}
