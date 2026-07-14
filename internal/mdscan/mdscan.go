// Package mdscan turns Markdown source into scrubbed prose blocks that keep
// exact source positions. "Scrubbed" means every non-prose span — fenced and
// indented code, inline code, HTML comments, link destinations, autolinked
// URLs, structural markers — is replaced byte-for-byte with spaces, so any
// byte offset into a scrubbed segment maps straight back onto the raw line.
// Downstream packages can therefore run plain string matching over prose and
// still report precise line/column positions.
package mdscan

import (
	"regexp"
	"strings"
)

// BlockKind classifies a prose block.
type BlockKind int

const (
	// Paragraph is running prose, including list-item bodies and blockquote
	// content (their markers are blanked, the text is kept).
	Paragraph BlockKind = iota
	// Heading is ATX (`## Title`) or setext (underlined) heading text.
	Heading
	// TableRow is a single GFM table row; each row is its own block so a
	// requirements matrix yields one analyzable unit per row.
	TableRow
)

// Segment is one source line's worth of scrubbed prose. Text has exactly the
// same byte length as the raw line it came from: offset i in Text is offset i
// in Doc.Lines[Line-1].
type Segment struct {
	Line int    // 1-based source line number
	Text string // scrubbed prose, same byte length as the raw line
}

// Block is a run of segments that belong together for sentence purposes: a
// paragraph, one heading, or one table row.
type Block struct {
	Kind     BlockKind
	Segments []Segment
}

// DirectiveKind enumerates the inline-comment control directives.
type DirectiveKind int

const (
	// DirDisable suppresses rules from the directive's line onward.
	DirDisable DirectiveKind = iota
	// DirEnable re-enables rules from the directive's line onward.
	DirEnable
	// DirDisableNextLine suppresses rules on the line directly below.
	DirDisableNextLine
)

// Directive is a parsed `<!-- mustlint-… -->` control comment.
type Directive struct {
	Line  int
	Kind  DirectiveKind
	Rules []string // empty means "all rules"
}

// Doc is the scan result for one file.
type Doc struct {
	Path       string
	Lines      []string // raw source lines (CR stripped), for column math
	Blocks     []Block
	Directives []Directive
}

var (
	reFenceOpen  = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})(.*)$")
	reATX        = regexp.MustCompile(`^( {0,3}#{1,6})([ \t]|$)`)
	reATXTrail   = regexp.MustCompile(`[ \t]#+[ \t]*$`)
	reSetext     = regexp.MustCompile(`^ {0,3}(=+|-+)[ \t]*$`)
	reListMarker = regexp.MustCompile(`^([ \t]{0,3})([-*+]|\d{1,9}[.)])([ \t]+)`)
	reLinkDef    = regexp.MustCompile(`^ {0,3}\[[^\]]+\]:[ \t]`)
	reTableDelim = regexp.MustCompile(`^\|?[ \t:|-]+\|?$`)
	reQuoteMark  = regexp.MustCompile(`^( {0,3}>)+ ?`)
	reAutolink   = regexp.MustCompile(`<[a-zA-Z][a-zA-Z0-9+.-]*:[^ <>]+>`)
	reBareURL    = regexp.MustCompile(`\bhttps?://[^\s)\]>"']+`)
	reDirective  = regexp.MustCompile(`^\s*mustlint-(disable-next-line|disable|enable)(\s.*)?$`)
)

// Scan parses one Markdown file into prose blocks and directives.
func Scan(path, content string) *Doc {
	d := &Doc{Path: path}
	lines := strings.Split(content, "\n")

	var cur *Block
	inFence := false
	var fenceCh byte
	var fenceLen int
	inComment := false
	inFrontMatter := false
	prevBlankOrCode := true // file start behaves like a blank line

	flush := func() {
		if cur != nil && len(cur.Segments) > 0 {
			d.Blocks = append(d.Blocks, *cur)
		}
		cur = nil
	}

	for i, rawLine := range lines {
		raw := strings.TrimSuffix(rawLine, "\r")
		d.Lines = append(d.Lines, raw)
		n := i + 1

		// YAML front matter: a `---` fence pair at the very top of the file.
		if inFrontMatter {
			t := strings.TrimSpace(raw)
			if t == "---" || t == "..." {
				inFrontMatter = false
			}
			continue
		}
		if n == 1 && strings.TrimSpace(raw) == "---" {
			inFrontMatter = true
			continue
		}

		work := raw

		// A multi-line HTML comment hides everything until `-->`; the hidden
		// span may contain fence markers, so comment state wins over fences.
		if inComment {
			idx := strings.Index(work, "-->")
			if idx < 0 {
				flush()
				prevBlankOrCode = true
				continue
			}
			work = strings.Repeat(" ", idx+3) + work[idx+3:]
			inComment = false
		}

		// Inside a fenced code block everything is code, including `<!--`.
		if inFence {
			body := stripQuoteMarkers(work)
			if m := reFenceOpen.FindStringSubmatch(body); m != nil &&
				m[1][0] == fenceCh && len(m[1]) >= fenceLen && strings.TrimSpace(m[2]) == "" {
				inFence = false
			}
			prevBlankOrCode = true
			continue
		}

		// Blockquote markers are structural: blank them, keep the prose.
		work = blankQuoteMarkers(work)

		// Inline code first (a backticked `<!-- … -->` is literal text, not a
		// directive), then comments, so commented-out prose never counts.
		work = scrubCodeSpans(work)
		work, inComment = d.scrubComments(work, n, inComment)

		trimmed := strings.TrimSpace(work)

		// Setext underline: promote the open paragraph to a heading.
		if cur != nil && len(cur.Segments) > 0 && reSetext.MatchString(work) {
			cur.Kind = Heading
			flush()
			prevBlankOrCode = true
			continue
		}

		if trimmed == "" {
			flush()
			prevBlankOrCode = true
			continue
		}

		if m := reFenceOpen.FindStringSubmatch(work); m != nil {
			// Backtick fences may not contain backticks in the info string.
			if m[1][0] == '~' || !strings.Contains(m[2], "`") {
				flush()
				inFence = true
				fenceCh = m[1][0]
				fenceLen = len(m[1])
				prevBlankOrCode = true
				continue
			}
		}

		// Indented code: 4+ spaces or a tab, only where a paragraph is not
		// already open (lazy continuation lines stay prose).
		if cur == nil && prevBlankOrCode &&
			(strings.HasPrefix(work, "    ") || strings.HasPrefix(work, "\t")) {
			prevBlankOrCode = true
			continue
		}

		if isThematicBreak(work) {
			flush()
			prevBlankOrCode = true
			continue
		}

		if reLinkDef.MatchString(work) {
			flush()
			prevBlankOrCode = true
			continue
		}

		if m := reATX.FindStringSubmatchIndex(work); m != nil {
			flush()
			text := strings.Repeat(" ", m[3]) + work[m[3]:]
			if t := reATXTrail.FindStringIndex(text); t != nil {
				text = text[:t[0]] + strings.Repeat(" ", t[1]-t[0])
			}
			text = scrubURLs(text)
			d.Blocks = append(d.Blocks, Block{
				Kind:     Heading,
				Segments: []Segment{{Line: n, Text: text}},
			})
			prevBlankOrCode = false
			continue
		}

		if strings.HasPrefix(trimmed, "|") {
			flush()
			if !reTableDelim.MatchString(trimmed) {
				d.Blocks = append(d.Blocks, Block{
					Kind:     TableRow,
					Segments: []Segment{{Line: n, Text: scrubURLs(work)}},
				})
			}
			prevBlankOrCode = false
			continue
		}

		if m := reListMarker.FindStringSubmatchIndex(work); m != nil {
			flush()
			work = work[:m[4]] + strings.Repeat(" ", m[5]-m[4]) + work[m[5]:]
		}

		if cur == nil {
			cur = &Block{Kind: Paragraph}
		}
		cur.Segments = append(cur.Segments, Segment{Line: n, Text: scrubURLs(work)})
		prevBlankOrCode = false
	}
	flush()
	return d
}

// scrubComments blanks `<!-- … -->` spans that start on this line, records
// any mustlint directives found in single-line comments, and reports whether
// the line ends inside an unterminated comment.
func (d *Doc) scrubComments(line string, n int, inComment bool) (string, bool) {
	b := []byte(line)
	i := 0
	for {
		start := strings.Index(string(b[i:]), "<!--")
		if start < 0 {
			break
		}
		start += i
		rest := string(b[start+4:])
		end := strings.Index(rest, "-->")
		if end < 0 {
			for p := start; p < len(b); p++ {
				b[p] = ' '
			}
			return string(b), true
		}
		content := rest[:end]
		if m := reDirective.FindStringSubmatch(strings.TrimSpace(content)); m != nil {
			d.Directives = append(d.Directives, Directive{
				Line:  n,
				Kind:  directiveKind(m[1]),
				Rules: splitRuleList(m[2]),
			})
		}
		stop := start + 4 + end + 3
		for p := start; p < stop; p++ {
			b[p] = ' '
		}
		i = stop
	}
	return string(b), inComment
}

func directiveKind(word string) DirectiveKind {
	switch word {
	case "disable":
		return DirDisable
	case "enable":
		return DirEnable
	default:
		return DirDisableNextLine
	}
}

func splitRuleList(s string) []string {
	var rules []string
	for _, f := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t'
	}) {
		if f != "" {
			rules = append(rules, f)
		}
	}
	return rules
}

// isThematicBreak reports a CommonMark thematic break: three or more of the
// same `-`, `*`, or `_` with nothing but spaces between (RE2 has no
// backreferences, so this is a hand-rolled check).
func isThematicBreak(s string) bool {
	t := strings.TrimLeft(s, " ")
	if len(s)-len(t) > 3 {
		return false
	}
	var ch byte
	count := 0
	for i := 0; i < len(t); i++ {
		c := t[i]
		if c == ' ' || c == '\t' {
			continue
		}
		if c != '*' && c != '-' && c != '_' {
			return false
		}
		if ch == 0 {
			ch = c
		} else if c != ch {
			return false
		}
		count++
	}
	return count >= 3
}

// blankQuoteMarkers replaces leading `>` markers (possibly nested) with
// spaces so blockquoted prose keeps its exact columns.
func blankQuoteMarkers(s string) string {
	loc := reQuoteMark.FindStringIndex(s)
	if loc == nil {
		return s
	}
	return strings.Repeat(" ", loc[1]) + s[loc[1]:]
}

// stripQuoteMarkers removes leading `>` markers for structural matching
// (fence close detection inside blockquotes); it does not preserve length.
func stripQuoteMarkers(s string) string {
	loc := reQuoteMark.FindStringIndex(s)
	if loc == nil {
		return s
	}
	return s[loc[1]:]
}

// scrubCodeSpans blanks inline code spans, delimiters included. A backtick
// run with no matching closer of the same length is left as literal text,
// mirroring CommonMark. Spans are matched per line only.
func scrubCodeSpans(s string) string {
	b := []byte(s)
	i := 0
	for i < len(b) {
		if b[i] != '`' {
			i++
			continue
		}
		j := i
		for j < len(b) && b[j] == '`' {
			j++
		}
		n := j - i
		closeEnd := -1
		k := j
		for k < len(b) {
			if b[k] != '`' {
				k++
				continue
			}
			m := k
			for m < len(b) && b[m] == '`' {
				m++
			}
			if m-k == n {
				closeEnd = m
				break
			}
			k = m
		}
		if closeEnd < 0 {
			i = j
			continue
		}
		for p := i; p < closeEnd; p++ {
			b[p] = ' '
		}
		i = closeEnd
	}
	return string(b)
}

// scrubURLs blanks link destinations `](…)`, autolinks `<scheme:…>`, and
// bare http(s) URLs — paths like /docs/must-read would otherwise leak
// keyword-shaped tokens into prose.
func scrubURLs(s string) string {
	s = reAutolink.ReplaceAllStringFunc(s, blank)
	s = reBareURL.ReplaceAllStringFunc(s, blank)
	b := []byte(s)
	for i := 0; i+1 < len(b); i++ {
		if b[i] != ']' || b[i+1] != '(' {
			continue
		}
		depth := 0
		j := i + 1
		for ; j < len(b); j++ {
			if b[j] == '(' {
				depth++
			} else if b[j] == ')' {
				depth--
				if depth == 0 {
					break
				}
			}
		}
		if j < len(b) {
			for p := i + 1; p <= j; p++ {
				b[p] = ' '
			}
			i = j
		}
	}
	return string(b)
}

func blank(m string) string { return strings.Repeat(" ", len(m)) }
