// Tests for the Markdown scanner: what counts as prose, what gets blanked,
// and whether byte positions survive scrubbing. Every case here guards a
// real false-positive class (keywords inside code, comments, URLs, …).
package mdscan

import (
	"strings"
	"testing"
)

// proseText joins every prose segment of a scan for containment checks.
func proseText(d *Doc) string {
	var b strings.Builder
	for _, blk := range d.Blocks {
		for _, seg := range blk.Segments {
			b.WriteString(seg.Text)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func TestParagraphAndBlankLineBlocks(t *testing.T) {
	d := Scan("t.md", "The server MUST reply.\nIt SHOULD log the request.\n")
	if len(d.Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1 (wrapped lines join)", len(d.Blocks))
	}
	if got := len(d.Blocks[0].Segments); got != 2 {
		t.Fatalf("segments = %d, want 2", got)
	}
	if d.Blocks[0].Segments[1].Line != 2 {
		t.Errorf("second segment line = %d, want 2", d.Blocks[0].Segments[1].Line)
	}
	d = Scan("t.md", "First paragraph.\n\nSecond paragraph.\n")
	if len(d.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2 (blank line splits)", len(d.Blocks))
	}
}

func TestFencedCodeIsSkipped(t *testing.T) {
	d := Scan("t.md", "Before.\n\n```\nThe client MUST NOT appear here.\n```\n\nAfter.\n")
	if got := proseText(d); strings.Contains(got, "MUST") {
		t.Errorf("backtick fence leaked into prose: %q", got)
	}
	if len(d.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2 (Before/After)", len(d.Blocks))
	}
	d = Scan("t.md", "~~~text\nMUST\n~~~\nProse.\n")
	got := proseText(d)
	if strings.Contains(got, "MUST") {
		t.Errorf("tilde fence leaked: %q", got)
	}
	if !strings.Contains(got, "Prose.") {
		t.Errorf("prose after tilde fence lost: %q", got)
	}
}

func TestFenceCloseRulesAndUnclosedFence(t *testing.T) {
	// A ``` close cannot terminate a ```` fence.
	d := Scan("t.md", "````\ncode\n```\nstill MUST be code\n````\nProse.\n")
	got := proseText(d)
	if strings.Contains(got, "MUST") {
		t.Errorf("short close terminated a longer fence: %q", got)
	}
	if !strings.Contains(got, "Prose.") {
		t.Errorf("prose after real close lost: %q", got)
	}
	// An unclosed fence swallows the rest of the file.
	d = Scan("t.md", "```\nMUST\nnever closes\n")
	if got := proseText(d); strings.Contains(got, "MUST") {
		t.Errorf("unclosed fence leaked: %q", got)
	}
}

func TestIndentedCodeVsLazyContinuation(t *testing.T) {
	// Four-space indent after a blank line is code …
	d := Scan("t.md", "Prose.\n\n    client.send(\"MUST\")\n\nMore prose.\n")
	if got := proseText(d); strings.Contains(got, "client.send") {
		t.Errorf("indented code leaked: %q", got)
	}
	// … but a 4-space continuation of an open paragraph is lazy prose.
	d = Scan("t.md", "The server MUST\n    reply promptly.\n")
	if got := proseText(d); !strings.Contains(got, "reply promptly.") {
		t.Errorf("lazy continuation dropped: %q", got)
	}
}

func TestInlineCodeSpansBlanked(t *testing.T) {
	raw := "Use `MUST` carefully."
	d := Scan("t.md", raw+"\n")
	seg := d.Blocks[0].Segments[0]
	if len(seg.Text) != len(raw) {
		t.Fatalf("scrubbed length %d != raw length %d", len(seg.Text), len(raw))
	}
	if strings.Contains(seg.Text, "MUST") {
		t.Errorf("inline code leaked: %q", seg.Text)
	}
	if !strings.Contains(seg.Text, "carefully.") {
		t.Errorf("prose around code lost: %q", seg.Text)
	}
	// Double-backtick spans blank literal inner backticks too.
	d = Scan("t.md", "The token `` `MUST` `` is literal.\n")
	if got := proseText(d); strings.Contains(got, "MUST") {
		t.Errorf("double-backtick span leaked: %q", got)
	}
}

func TestUnclosedBacktickIsLiteralText(t *testing.T) {
	d := Scan("t.md", "A stray ` then the server MUST reply.\n")
	if got := proseText(d); !strings.Contains(got, "MUST") {
		t.Errorf("unclosed backtick swallowed prose: %q", got)
	}
}

func TestHTMLCommentsBlanked(t *testing.T) {
	d := Scan("t.md", "Visible <!-- hidden MUST --> text.\n")
	got := proseText(d)
	if strings.Contains(got, "MUST") {
		t.Errorf("comment content leaked: %q", got)
	}
	if !strings.Contains(got, "Visible") || !strings.Contains(got, "text.") {
		t.Errorf("prose around comment lost: %q", got)
	}
	// Comments spanning lines hide everything until the closer.
	d = Scan("t.md", "Before.\n<!-- line one MUST\nline two SHALL -->\nAfter.\n")
	got = proseText(d)
	if strings.Contains(got, "MUST") || strings.Contains(got, "SHALL") {
		t.Errorf("multi-line comment leaked: %q", got)
	}
	if !strings.Contains(got, "After.") {
		t.Errorf("prose after comment lost: %q", got)
	}
}

func TestDirectiveInsideInlineCodeIsIgnored(t *testing.T) {
	// Documentation that *shows* a directive must not trigger it.
	d := Scan("t.md", "Write `<!-- mustlint-disable may-not -->` above the line.\n")
	if len(d.Directives) != 0 {
		t.Fatalf("directives = %v, want none", d.Directives)
	}
}

func TestDirectiveParsing(t *testing.T) {
	src := "<!-- mustlint-disable may-not, id-gap -->\n" +
		"text\n" +
		"<!-- mustlint-enable -->\n" +
		"<!-- mustlint-disable-next-line ambiguous-term -->\n"
	d := Scan("t.md", src)
	if len(d.Directives) != 3 {
		t.Fatalf("directives = %d, want 3", len(d.Directives))
	}
	first := d.Directives[0]
	if first.Kind != DirDisable || first.Line != 1 {
		t.Errorf("first = %+v, want disable at line 1", first)
	}
	if len(first.Rules) != 2 || first.Rules[0] != "may-not" || first.Rules[1] != "id-gap" {
		t.Errorf("first rules = %v, want [may-not id-gap]", first.Rules)
	}
	if d.Directives[1].Kind != DirEnable || len(d.Directives[1].Rules) != 0 {
		t.Errorf("second = %+v, want bare enable", d.Directives[1])
	}
	if d.Directives[2].Kind != DirDisableNextLine {
		t.Errorf("third = %+v, want disable-next-line", d.Directives[2])
	}
}

func TestHeadingsATXAndSetext(t *testing.T) {
	d := Scan("t.md", "## Requirements MUST ##\n")
	if len(d.Blocks) != 1 || d.Blocks[0].Kind != Heading {
		t.Fatalf("blocks = %+v, want one Heading", d.Blocks)
	}
	seg := d.Blocks[0].Segments[0]
	if strings.Contains(seg.Text, "#") {
		t.Errorf("hash markers not blanked: %q", seg.Text)
	}
	if !strings.Contains(seg.Text, "Requirements MUST") {
		t.Errorf("heading text lost: %q", seg.Text)
	}
	d = Scan("t.md", "Protocol Overview\n=================\n\nProse.\n")
	if len(d.Blocks) != 2 || d.Blocks[0].Kind != Heading {
		t.Fatalf("setext paragraph not promoted to heading: %+v", d.Blocks)
	}
	if d.Blocks[1].Kind != Paragraph {
		t.Errorf("following prose misclassified: %+v", d.Blocks[1])
	}
}

func TestBlockquoteMarkersBlankedInPlace(t *testing.T) {
	raw := "> The client MUST retry."
	d := Scan("t.md", raw+"\n")
	seg := d.Blocks[0].Segments[0]
	if len(seg.Text) != len(raw) {
		t.Fatalf("length changed: %d != %d", len(seg.Text), len(raw))
	}
	if strings.Contains(seg.Text, ">") {
		t.Errorf("quote marker kept: %q", seg.Text)
	}
	if !strings.Contains(seg.Text, "MUST retry.") {
		t.Errorf("quoted prose lost: %q", seg.Text)
	}
}

func TestListItemsAreSeparateBlocks(t *testing.T) {
	d := Scan("t.md", "- first item MUST hold\n- second item MAY hold\n")
	if len(d.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2 (one per item)", len(d.Blocks))
	}
	if strings.Contains(d.Blocks[0].Segments[0].Text, "-") {
		t.Errorf("list marker kept: %q", d.Blocks[0].Segments[0].Text)
	}
}

func TestTableRowsAreSeparateBlocksAndDelimiterSkipped(t *testing.T) {
	src := "| ID | Rule |\n|---|---|\n| REQ-1 | MUST reply |\n"
	d := Scan("t.md", src)
	if len(d.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2 (header + one data row)", len(d.Blocks))
	}
	for _, blk := range d.Blocks {
		if blk.Kind != TableRow {
			t.Errorf("block kind = %v, want TableRow", blk.Kind)
		}
	}
}

func TestURLsAndLinkTargetsBlanked(t *testing.T) {
	d := Scan("t.md", "See [the spec](https://example.test/must-not.html) for details.\n")
	got := proseText(d)
	if strings.Contains(got, "must-not") {
		t.Errorf("link destination leaked: %q", got)
	}
	if !strings.Contains(got, "the spec") {
		t.Errorf("link text lost: %q", got)
	}
	d = Scan("t.md", "Docs at https://example.test/MUST/read and <https://example.test/shall>.\n")
	got = proseText(d)
	if strings.Contains(got, "MUST") || strings.Contains(got, "shall") {
		t.Errorf("URL path leaked keyword-shaped tokens: %q", got)
	}
	d = Scan("t.md", "[spec]: https://example.test/must.html\nProse.\n")
	if got := proseText(d); strings.Contains(got, "must.html") {
		t.Errorf("link reference definition leaked: %q", got)
	}
}

func TestFrontMatterSkipped(t *testing.T) {
	d := Scan("t.md", "---\ntitle: The MUST guide\n---\nReal prose.\n")
	got := proseText(d)
	if strings.Contains(got, "MUST") {
		t.Errorf("front matter leaked: %q", got)
	}
	if !strings.Contains(got, "Real prose.") {
		t.Errorf("prose after front matter lost: %q", got)
	}
}

func TestThematicBreakAndSetextDash(t *testing.T) {
	// `---` under a one-line paragraph is a setext H2 per CommonMark.
	d := Scan("t.md", "Above.\n---\n")
	if len(d.Blocks) != 1 || d.Blocks[0].Kind != Heading {
		t.Fatalf("blocks = %+v, want one setext heading", d.Blocks)
	}
	// A free-standing break separates paragraphs.
	d = Scan("t.md", "Above.\n\n* * *\n\nBelow.\n")
	if len(d.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2 around thematic break", len(d.Blocks))
	}
}

func TestCRLFLinesHandled(t *testing.T) {
	d := Scan("t.md", "The server MUST reply.\r\nSecond line.\r\n")
	if len(d.Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(d.Blocks))
	}
	if strings.Contains(proseText(d), "\r") {
		t.Errorf("carriage returns kept in prose")
	}
}

func TestCommentedOutFenceStaysProse(t *testing.T) {
	// A fence opener hidden inside a comment must not open a code block.
	d := Scan("t.md", "<!-- ``` -->\nThe client MUST retry.\n")
	if got := proseText(d); !strings.Contains(got, "MUST") {
		t.Errorf("commented fence swallowed prose: %q", got)
	}
}
