// Tests for the document model: sentence segmentation, position mapping,
// boilerplate parsing, requirement-ID extraction, and normativity.
package spec

import (
	"regexp"
	"strings"
	"testing"
)

// sentences returns the plain sentence texts of a built document,
// whitespace-normalized for easy comparison.
func sentences(content string) []string {
	d := Build("t.md", content, nil)
	var out []string
	for _, s := range d.Sentences {
		out = append(out, strings.Join(strings.Fields(s.Text), " "))
	}
	return out
}

func TestSentenceSplitBasic(t *testing.T) {
	got := sentences("First rule applies. Second rule applies! Third applies?\n")
	want := []string{"First rule applies.", "Second rule applies!", "Third applies?"}
	if len(got) != len(want) {
		t.Fatalf("sentences = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sentence %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAbbreviationsAndVersionsDoNotSplit(t *testing.T) {
	got := sentences("Formats (e.g. JSON) apply, i.e. the wire form. Next sentence.\n")
	if len(got) != 2 {
		t.Fatalf("sentences = %q, want 2 (e.g./i.e. must not split)", got)
	}
	got = sentences("Deploy v0.1.0 with TLS 1.3 today. Then verify.\n")
	if len(got) != 2 {
		t.Fatalf("sentences = %q, want 2 (dotted versions must not split)", got)
	}
}

func TestLowercaseContinuationDoesNotSplit(t *testing.T) {
	// "etc." followed by lowercase keeps the sentence together …
	got := sentences("Retries, backoff, etc. are covered later. Second.\n")
	if len(got) != 2 {
		t.Fatalf("sentences = %q, want 2", got)
	}
	// … while "etc." before an uppercase start ends the sentence.
	got = sentences("It covers retries, backoff, etc. The next section differs.\n")
	if len(got) != 2 {
		t.Fatalf("sentences = %q, want 2 (etc. before uppercase splits)", got)
	}
	if !strings.HasPrefix(got[1], "The next") {
		t.Errorf("second sentence = %q, want to start at 'The next'", got[1])
	}
}

func TestSentenceSpansSourceLines(t *testing.T) {
	d := Build("t.md", "The server MUST\nreply within 5 seconds.\n", nil)
	if len(d.Sentences) != 1 {
		t.Fatalf("sentences = %d, want 1", len(d.Sentences))
	}
	s := d.Sentences[0]
	if len(s.Matches) != 1 {
		t.Fatalf("matches = %+v, want one MUST", s.Matches)
	}
	p := s.PosAt(s.Matches[0].Start)
	if p.Line != 1 || p.Col != 12 {
		t.Errorf("MUST at %d:%d, want 1:12", p.Line, p.Col)
	}
	// An offset inside the wrapped part must map to line 2.
	idx := strings.Index(s.Text, "reply")
	p = s.PosAt(idx)
	if p.Line != 2 || p.Col != 1 {
		t.Errorf("'reply' at %d:%d, want 2:1", p.Line, p.Col)
	}
}

func TestColumnsCountRunesNotBytes(t *testing.T) {
	// Multibyte runes before the keyword: columns must count runes so
	// editors jump to the right spot.
	d := Build("t.md", "«Ключ» clients MUST comply.\n", nil)
	s := d.Sentences[0]
	p := s.PosAt(s.Matches[0].Start)
	if p.Col != 16 {
		t.Errorf("MUST col = %d, want 16 (rune count)", p.Col)
	}
}

func TestBoilerplateDetectedPlain2119(t *testing.T) {
	d := Build("t.md", `The key words "MUST" and "SHOULD" in this document are to be
interpreted as described in RFC 2119.
`, nil)
	if d.Boilerplate == nil {
		t.Fatal("boilerplate not detected")
	}
	if d.Boilerplate.Has8174 {
		t.Error("plain RFC 2119 boilerplate misread as RFC 8174")
	}
	if !d.Boilerplate.Sentence.Boiler {
		t.Error("boilerplate sentence not marked")
	}
}

func TestBoilerplate8174Detected(t *testing.T) {
	d := Build("t.md", `The key words "MUST", "MUST NOT", and "MAY" in this document are to be
interpreted as described in BCP 14 [RFC2119] [RFC8174] when, and only
when, they appear in all capitals, as shown here.
`, nil)
	if d.Boilerplate == nil || !d.Boilerplate.Has8174 {
		t.Fatalf("BCP 14 boilerplate not recognized: %+v", d.Boilerplate)
	}
}

func TestDeclaredKeywordListParsed(t *testing.T) {
	d := Build("t.md", `The key words "MUST", "MUST NOT", and "SHOULD" in this document are
to be interpreted as described in RFC 2119.
`, nil)
	want := []string{"MUST", "MUST NOT", "SHOULD"}
	got := d.Boilerplate.Declared
	if len(got) != len(want) {
		t.Fatalf("declared = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("declared[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	// Word processors curl the quotes; the parser must not care.
	d = Build("t.md", "The key words “MUST” and “MAY” in this document are to be interpreted as described in RFC 2119.\n", nil)
	if d.Boilerplate == nil || len(d.Boilerplate.Declared) != 2 {
		t.Fatalf("curly-quoted keywords not parsed: %+v", d.Boilerplate)
	}
}

func TestIDExtractionDefaultPattern(t *testing.T) {
	d := Build("t.md", "REQ-1: The device MUST boot. REQ-AUTH-17 applies too.\n", nil)
	s := d.Sentences[0]
	if len(s.IDs) != 1 || s.IDs[0].ID != "REQ-1" {
		t.Fatalf("first sentence IDs = %+v, want [REQ-1]", s.IDs)
	}
	s2 := d.Sentences[1]
	if len(s2.IDs) != 1 || s2.IDs[0].ID != "REQ-AUTH-17" {
		t.Fatalf("second sentence IDs = %+v, want [REQ-AUTH-17]", s2.IDs)
	}
	if s2.IDs[0].Series != "REQ-AUTH" || s2.IDs[0].Num != 17 {
		t.Errorf("split = (%q, %d), want (REQ-AUTH, 17)", s2.IDs[0].Series, s2.IDs[0].Num)
	}
}

func TestCitationStoplistAndCustomPattern(t *testing.T) {
	// The default pattern refuses document citations …
	d := Build("t.md", "Peers MUST follow RFC-2119 and ISO-8601 and SHA-256.\n", nil)
	if got := d.Sentences[0].IDs; len(got) != 0 {
		t.Fatalf("citations matched as requirement IDs: %+v", got)
	}
	// … while a user-supplied pattern is trusted verbatim, stoplist off.
	re := regexp.MustCompile(`\bRFC-[0-9]+\b`)
	d = Build("t.md", "Peers MUST follow RFC-2119.\n", re)
	got := d.Sentences[0].IDs
	if len(got) != 1 || got[0].ID != "RFC-2119" {
		t.Fatalf("custom pattern IDs = %+v, want [RFC-2119]", got)
	}
}

func TestSectionIDInheritance(t *testing.T) {
	src := `## REQ-3 Handshake

The client MUST send hello.

## Overview

The server MUST reply.
`
	d := Build("t.md", src, nil)
	var hello, reply *Sentence
	for _, s := range d.Sentences {
		if strings.Contains(s.Text, "hello") {
			hello = s
		}
		if strings.Contains(s.Text, "reply") {
			reply = s
		}
	}
	if hello.SectionID != "REQ-3" {
		t.Errorf("hello SectionID = %q, want REQ-3", hello.SectionID)
	}
	if reply.SectionID != "" {
		t.Errorf("reply SectionID = %q, want empty (plain heading resets)", reply.SectionID)
	}
}

func TestNormativeClassification(t *testing.T) {
	src := `## The client MUST wave

The key words "MUST" in this document are to be interpreted as described in RFC 2119.

The client MUST wave. The client must wave. The client WILL wave.
`
	d := Build("t.md", src, nil)
	var got []bool
	for _, s := range d.Sentences {
		got = append(got, s.Normative())
	}
	// heading, boilerplate, MUST, must, WILL
	want := []bool{false, false, true, false, false}
	if len(got) != len(want) {
		t.Fatalf("sentences = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sentence %d normative = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestNormalizedStripsIDsCaseAndPunctuation(t *testing.T) {
	d1 := Build("a.md", "REQ-1: The server MUST reply, promptly.\n", nil)
	d2 := Build("b.md", "REQ-9 — the SERVER must reply promptly!\n", nil)
	n1 := d1.Sentences[0].Normalized()
	n2 := d2.Sentences[0].Normalized()
	if n1 != n2 {
		t.Errorf("normal forms differ:\n a: %q\n b: %q", n1, n2)
	}
	if strings.Contains(n1, "req") {
		t.Errorf("ID leaked into normal form: %q", n1)
	}
}

func TestTableRowSentences(t *testing.T) {
	d := Build("t.md", "| REQ-2 | The relay MUST forward frames |\n", nil)
	if len(d.Sentences) != 1 {
		t.Fatalf("sentences = %d, want 1", len(d.Sentences))
	}
	s := d.Sentences[0]
	if !s.TableRow || !s.Normative() {
		t.Errorf("table row sentence = %+v, want normative TableRow", s)
	}
	if len(s.IDs) != 1 || s.IDs[0].ID != "REQ-2" {
		t.Errorf("row IDs = %+v, want [REQ-2]", s.IDs)
	}
}
