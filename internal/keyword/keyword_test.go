// Tests for keyword matching: the eleven BCP 14 keywords, compound
// handling, case classification, MAY NOT, pseudo-keywords, and the word
// boundaries that keep MUSTARD out of the report.
package keyword

import (
	"strings"
	"testing"
)

// one asserts exactly one match and returns it.
func one(t *testing.T, text string) Match {
	t.Helper()
	ms := Find(text)
	if len(ms) != 1 {
		t.Fatalf("Find(%q) = %d matches (%+v), want 1", text, len(ms), ms)
	}
	return ms[0]
}

func TestFindsAllElevenCanonicalKeywords(t *testing.T) {
	for _, k := range Canonical {
		m := one(t, "The peer "+k+" comply.")
		if m.Canonical != k {
			t.Errorf("keyword %q matched as %q", k, m.Canonical)
		}
		if m.Kind != KindStandard {
			t.Errorf("keyword %q kind = %v, want KindStandard", k, m.Kind)
		}
		if m.Case != CaseUpper {
			t.Errorf("keyword %q case = %v, want CaseUpper", k, m.Case)
		}
	}
}

func TestCompoundConsumedAsSingleMatch(t *testing.T) {
	m := one(t, "Clients MUST NOT retry.")
	if m.Canonical != "MUST NOT" || m.Actual != "MUST NOT" {
		t.Errorf("match = %+v, want MUST NOT", m)
	}
}

func TestNotRecommendedCompound(t *testing.T) {
	m := one(t, "Fallback is NOT RECOMMENDED here.")
	if m.Canonical != "NOT RECOMMENDED" {
		t.Errorf("canonical = %q, want NOT RECOMMENDED", m.Canonical)
	}
}

func TestLowercaseAndTitleCaseClasses(t *testing.T) {
	if m := one(t, "you must comply"); m.Case != CaseLower {
		t.Errorf("lowercase must: case = %v, want CaseLower", m.Case)
	}
	if m := one(t, "Must we comply"); m.Case != CaseTitle {
		t.Errorf("Title Must: case = %v, want CaseTitle", m.Case)
	}
	if m := one(t, "Should not everyone agree"); m.Case != CaseTitle {
		t.Errorf("Title compound: case = %v, want CaseTitle", m.Case)
	}
}

func TestMixedCaseCompoundDetected(t *testing.T) {
	for _, text := range []string{"It MUST not fail.", "It must NOT fail.", "It Must NOT fail."} {
		if m := one(t, text); m.Case != CaseMixed {
			t.Errorf("Find(%q) case = %v, want CaseMixed", text, m.Case)
		}
	}
}

func TestMayNotHasItsOwnKind(t *testing.T) {
	m := one(t, "Peers MAY NOT reconnect.")
	if m.Kind != KindMayNot {
		t.Errorf("kind = %v, want KindMayNot", m.Kind)
	}
	if m.Case != CaseUpper {
		t.Errorf("case = %v, want CaseUpper", m.Case)
	}
	// Plain English "may not" is still identified so rules can decide to
	// ignore it based on case, not on a lossy match.
	m = one(t, "it may not matter")
	if m.Kind != KindMayNot || m.Case != CaseLower {
		t.Errorf("match = %+v, want lowercase KindMayNot", m)
	}
}

func TestPseudoKeywordsUppercaseOnly(t *testing.T) {
	for _, w := range []string{"WILL", "MIGHT", "CANNOT", "MANDATORY", "OPTIONALLY", "FORBIDDEN", "PROHIBITED"} {
		m := one(t, "The node "+w+" comply.")
		if m.Kind != KindPseudo {
			t.Errorf("%q kind = %v, want KindPseudo", w, m.Kind)
		}
		if got := Find("The node " + strings.ToLower(w) + " comply."); len(got) != 0 {
			t.Errorf("lowercase %q matched: %+v", w, got)
		}
	}
	// WILL NOT forms a pseudo compound, like the real keywords do.
	m := one(t, "The server WILL NOT respond.")
	if m.Canonical != "WILL NOT" || m.Kind != KindPseudo {
		t.Errorf("match = %+v, want WILL NOT pseudo", m)
	}
}

func TestWordBoundariesExcludeSuperstrings(t *testing.T) {
	for _, text := range []string{
		"Add MUSTARD to taste.",
		"His mustache SHALLOWLY twitched.",
		"REQUIREDNESS is not a word we use.",
	} {
		if got := Find(text); len(got) != 0 {
			t.Errorf("Find(%q) = %+v, want none", text, got)
		}
	}
	// "MuSt" is neither prose nor a keyword; flagging it would be noise.
	if got := Find("The MuSt token is odd."); len(got) != 0 {
		t.Errorf("scrambled case matched: %+v", got)
	}
}

func TestCompoundSpansNewline(t *testing.T) {
	// Blocks join wrapped lines with \n; the compound must survive the wrap.
	m := one(t, "Implementations MUST\nNOT block.")
	if m.Canonical != "MUST NOT" {
		t.Errorf("canonical = %q, want MUST NOT across newline", m.Canonical)
	}
}

func TestPunctuationBreaksCompound(t *testing.T) {
	ms := Find("You MUST, NOT maybe, comply.")
	if len(ms) != 1 || ms[0].Canonical != "MUST" {
		t.Errorf("matches = %+v, want single MUST (comma breaks compound)", ms)
	}
}

func TestOffsetsAreExactByteRanges(t *testing.T) {
	text := "Servers SHOULD NOT panic."
	m := one(t, text)
	if text[m.Start:m.End] != "SHOULD NOT" {
		t.Errorf("offsets [%d:%d] = %q, want SHOULD NOT", m.Start, m.End, text[m.Start:m.End])
	}
}

func TestMultipleMatchesInOrder(t *testing.T) {
	ms := Find("Clients MUST register and SHOULD retry; they MAY cache.")
	want := []string{"MUST", "SHOULD", "MAY"}
	if len(ms) != len(want) {
		t.Fatalf("matches = %d, want %d", len(ms), len(want))
	}
	for i, w := range want {
		if ms[i].Canonical != w {
			t.Errorf("match %d = %q, want %q", i, ms[i].Canonical, w)
		}
	}
}
