// Tests for the twelve rules and the engine: firing conditions,
// non-firing conditions, corpus-wide checks, suppression directives, and
// deterministic ordering. Each case documents the failure mode it guards.
package rules

import (
	"strings"
	"testing"

	"github.com/JaydenCJ/mustlint/internal/spec"
)

// bp8174 is the modern BCP 14 boilerplate used by clean fixtures.
const bp8174 = `The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT",
"SHOULD", "SHOULD NOT", "RECOMMENDED", "NOT RECOMMENDED", "MAY", and
"OPTIONAL" in this document are to be interpreted as described in BCP 14
[RFC2119] [RFC8174] when, and only when, they appear in all capitals, as
shown here.

`

// bp2119 is the legacy RFC 2119-only boilerplate; note it does not
// declare SHALL.
const bp2119 = `The key words "MUST", "MUST NOT", "SHOULD", and "MAY" in this
document are to be interpreted as described in RFC 2119.

`

// run lints one in-memory document with the default config.
func run(t *testing.T, content string) []Finding {
	t.Helper()
	return runCfg(t, content, Config{Disabled: map[string]bool{}})
}

func runCfg(t *testing.T, content string, cfg Config) []Finding {
	t.Helper()
	return Check([]*spec.Document{spec.Build("spec.md", content, cfg.IDPattern)}, cfg)
}

// byRule filters findings down to one rule.
func byRule(fs []Finding, rule string) []Finding {
	var out []Finding
	for _, f := range fs {
		if f.Rule == rule {
			out = append(out, f)
		}
	}
	return out
}

func TestMissingBoilerplate(t *testing.T) {
	// Fires on keyword use without a declaration, anchored at the first
	// uppercase keyword …
	fs := byRule(run(t, "The client MUST send hello.\n"), "missing-boilerplate")
	if len(fs) != 1 {
		t.Fatalf("missing-boilerplate = %+v, want exactly 1", fs)
	}
	if fs[0].Line != 1 || fs[0].Col != 12 {
		t.Errorf("anchored at %d:%d, want 1:12 (the first MUST)", fs[0].Line, fs[0].Col)
	}
	// … stays silent when the boilerplate exists …
	fs = byRule(run(t, bp8174+"The client MUST send hello.\n"), "missing-boilerplate")
	if len(fs) != 0 {
		t.Fatalf("fired despite boilerplate: %+v", fs)
	}
	// … and a README with no requirement language needs no boilerplate.
	if fs := run(t, "Just prose. Nothing normative here.\n"); len(fs) != 0 {
		t.Fatalf("findings on plain prose: %+v", fs)
	}
}

func TestOutdatedBoilerplate(t *testing.T) {
	src := bp2119 + "Clients MUST register. The relay should log failures.\n"
	fs := byRule(run(t, src), "outdated-boilerplate")
	if len(fs) != 1 {
		t.Fatalf("outdated-boilerplate = %+v, want 1", fs)
	}
	if !strings.Contains(fs[0].Message, "RFC 8174") {
		t.Errorf("message should point at RFC 8174: %q", fs[0].Message)
	}
	// The modern boilerplate makes lowercase instances legitimate.
	src = bp8174 + "Clients MUST register. The relay should log failures.\n"
	if fs := byRule(run(t, src), "outdated-boilerplate"); len(fs) != 0 {
		t.Fatalf("fired despite RFC 8174 boilerplate: %+v", fs)
	}
}

func TestLowercaseKeyword(t *testing.T) {
	// Flagged under a plain 2119 boilerplate …
	src := bp2119 + "Clients MUST register. The relay should log failures.\n"
	fs := byRule(run(t, src), "lowercase-keyword")
	if len(fs) != 1 {
		t.Fatalf("lowercase-keyword = %+v, want 1 (the lowercase should)", fs)
	}
	// … but lowercase "may" is everyday English and stays exempt …
	src = bp2119 + "Clients MUST register. Users may prefer JSON.\n"
	if fs := byRule(run(t, src), "lowercase-keyword"); len(fs) != 0 {
		t.Fatalf("lowercase 'may' flagged: %+v", fs)
	}
	// … and RFC 8174 makes lowercase non-normative by definition.
	src = bp8174 + "Clients MUST register. The relay should log failures.\n"
	if fs := byRule(run(t, src), "lowercase-keyword"); len(fs) != 0 {
		t.Fatalf("fired under RFC 8174: %+v", fs)
	}
}

func TestUndeclaredKeyword(t *testing.T) {
	src := bp2119 + "The gateway SHALL retry.\n" // SHALL absent from bp2119
	fs := byRule(run(t, src), "undeclared-keyword")
	if len(fs) != 1 || !strings.Contains(fs[0].Message, `"SHALL"`) {
		t.Fatalf("undeclared-keyword = %+v, want one about SHALL", fs)
	}
	src = bp2119 + "The gateway MUST retry.\n" // MUST is declared
	if fs := byRule(run(t, src), "undeclared-keyword"); len(fs) != 0 {
		t.Fatalf("declared keyword reported: %+v", fs)
	}
}

func TestMixedCaseKeywordError(t *testing.T) {
	fs := byRule(run(t, bp8174+"The client MUST not retry.\n"), "mixed-case-keyword")
	if len(fs) != 1 {
		t.Fatalf("mixed-case-keyword = %+v, want 1", fs)
	}
	if fs[0].Severity != Error {
		t.Errorf("severity = %v, want error", fs[0].Severity)
	}
}

func TestMayNot(t *testing.T) {
	fs := byRule(run(t, bp8174+"Peers MAY NOT reconnect.\n"), "may-not")
	if len(fs) != 1 || fs[0].Severity != Error {
		t.Fatalf("may-not = %+v, want one error", fs)
	}
	// Plain-English lowercase "may not" is not normative intent.
	fs = byRule(run(t, bp8174+"The cache MUST expire. Results may not be fresh.\n"), "may-not")
	if len(fs) != 0 {
		t.Fatalf("plain-English 'may not' flagged: %+v", fs)
	}
}

func TestPseudoKeyword(t *testing.T) {
	fs := byRule(run(t, bp8174+"The server WILL respond. Retries are FORBIDDEN.\n"), "pseudo-keyword")
	if len(fs) != 2 {
		t.Fatalf("pseudo-keyword = %+v, want 2 (WILL, FORBIDDEN)", fs)
	}
	if !strings.Contains(fs[0].Message, "MUST") {
		t.Errorf("WILL message should suggest MUST: %q", fs[0].Message)
	}
	// Headings are exempt (all-caps titles) and lowercase is plain prose.
	fs = byRule(run(t, "# WHAT THE SERVER WILL DO\n\nThe server will respond.\n"), "pseudo-keyword")
	if len(fs) != 0 {
		t.Fatalf("pseudo fired on heading or lowercase: %+v", fs)
	}
}

func TestMissingIDIsOptIn(t *testing.T) {
	src := bp8174 + "The client MUST send hello.\n"
	if fs := byRule(run(t, src), "missing-id"); len(fs) != 0 {
		t.Fatalf("missing-id fired without --require-ids: %+v", fs)
	}
	cfg := Config{Disabled: map[string]bool{}, RequireIDs: true}
	if fs := byRule(runCfg(t, src, cfg), "missing-id"); len(fs) != 1 {
		t.Fatalf("missing-id with --require-ids = %+v, want 1", fs)
	}
}

func TestInlineOrSectionIDSatisfiesRequireIDs(t *testing.T) {
	cfg := Config{Disabled: map[string]bool{}, RequireIDs: true}
	src := bp8174 + "REQ-1: The client MUST send hello.\n"
	if fs := byRule(runCfg(t, src, cfg), "missing-id"); len(fs) != 0 {
		t.Fatalf("inline ID not honored: %+v", fs)
	}
	src = bp8174 + "## REQ-2 Handshake\n\nThe client MUST send hello.\n"
	if fs := byRule(runCfg(t, src, cfg), "missing-id"); len(fs) != 0 {
		t.Fatalf("section ID not inherited: %+v", fs)
	}
}

func TestDuplicateIDWithinFile(t *testing.T) {
	src := bp8174 + "REQ-1: The client MUST wave.\n\nREQ-1: The server MUST bow.\n"
	fs := byRule(run(t, src), "duplicate-id")
	if len(fs) != 1 {
		t.Fatalf("duplicate-id = %+v, want 1", fs)
	}
	if !strings.Contains(fs[0].Message, "spec.md:") {
		t.Errorf("message should cite the first definition: %q", fs[0].Message)
	}
}

func TestDuplicateIDAcrossFiles(t *testing.T) {
	a := spec.Build("a.md", bp8174+"REQ-1: The client MUST wave.\n", nil)
	b := spec.Build("b.md", bp8174+"REQ-1: The server MUST bow.\n", nil)
	fs := byRule(Check([]*spec.Document{a, b}, Config{}), "duplicate-id")
	if len(fs) != 1 || fs[0].File != "b.md" {
		t.Fatalf("cross-file duplicate = %+v, want one finding in b.md", fs)
	}
	if !strings.Contains(fs[0].Message, "a.md:") {
		t.Errorf("message should cite a.md: %q", fs[0].Message)
	}
}

func TestCrossReferenceIsNotADefinition(t *testing.T) {
	// The second ID in a sentence and IDs in non-normative prose are
	// references; only real definitions may collide.
	src := bp8174 + "REQ-1: The client MUST wave.\n\n" +
		"REQ-2: The server MUST bow, superseding REQ-1.\n\n" +
		"See REQ-1 for the handshake.\n"
	if fs := byRule(run(t, src), "duplicate-id"); len(fs) != 0 {
		t.Fatalf("cross-references misread as definitions: %+v", fs)
	}
}

func TestIDGapReported(t *testing.T) {
	src := bp8174 + "REQ-1: The client MUST wave.\n\nREQ-4: The server MUST bow.\n"
	fs := byRule(run(t, src), "id-gap")
	if len(fs) != 1 {
		t.Fatalf("id-gap = %+v, want 1", fs)
	}
	if !strings.Contains(fs[0].Message, "REQ-2, REQ-3") {
		t.Errorf("message should name the missing IDs: %q", fs[0].Message)
	}
}

func TestNoGapForConsecutiveOrUnrelatedSeries(t *testing.T) {
	src := bp8174 + "REQ-1: The client MUST wave.\n\nREQ-2: The server MUST bow.\n"
	if fs := byRule(run(t, src), "id-gap"); len(fs) != 0 {
		t.Fatalf("id-gap on consecutive IDs: %+v", fs)
	}
	// SEC-1 followed by REQ-9 is not a gap; series are tracked separately,
	// and a single-member series can have no gap at all.
	src = bp8174 + "SEC-1: Peers MUST verify certs.\n\nREQ-9: The server MUST bow.\n"
	if fs := byRule(run(t, src), "id-gap"); len(fs) != 0 {
		t.Fatalf("gap across unrelated series: %+v", fs)
	}
}

func TestDuplicateRequirement(t *testing.T) {
	src := bp8174 + "REQ-1: The relay MUST forward every frame unchanged.\n\n" +
		"REQ-2: The relay MUST forward every frame, unchanged!\n"
	fs := byRule(run(t, src), "duplicate-requirement")
	if len(fs) != 1 {
		t.Fatalf("duplicate-requirement = %+v, want 1 (IDs and punctuation differ only)", fs)
	}
	// Short rows repeat legitimately in overview tables.
	src = bp8174 + "Servers MUST support TLS.\n\nServers MUST support TLS.\n"
	if fs := byRule(run(t, src), "duplicate-requirement"); len(fs) != 0 {
		t.Fatalf("short repeated statement flagged: %+v", fs)
	}
}

func TestAmbiguousTermOnlyInNormativeSentences(t *testing.T) {
	fs := byRule(run(t, bp8174+"The relay MUST drop frames as appropriate.\n"), "ambiguous-term")
	if len(fs) != 1 {
		t.Fatalf("ambiguous-term = %+v, want 1", fs)
	}
	if !strings.Contains(fs[0].Message, `"as appropriate"`) {
		t.Errorf("message should quote the term: %q", fs[0].Message)
	}
	// Descriptive prose is allowed to hedge.
	fs = byRule(run(t, bp8174+"Deployments vary as appropriate for each site.\n"), "ambiguous-term")
	if len(fs) != 0 {
		t.Fatalf("descriptive prose flagged: %+v", fs)
	}
}

func TestDisableNextLineDirective(t *testing.T) {
	src := bp8174 + "<!-- mustlint-disable-next-line may-not -->\nPeers MAY NOT reconnect.\n"
	if fs := byRule(run(t, src), "may-not"); len(fs) != 0 {
		t.Fatalf("disable-next-line ignored: %+v", fs)
	}
}

func TestDisableEnableRangeAndBareDisable(t *testing.T) {
	src := bp8174 +
		"<!-- mustlint-disable may-not -->\nPeers MAY NOT reconnect.\n\n" +
		"<!-- mustlint-enable may-not -->\nPeers MAY NOT resubscribe.\n"
	fs := byRule(run(t, src), "may-not")
	if len(fs) != 1 {
		t.Fatalf("may-not = %+v, want 1 (only after enable)", fs)
	}
	// A bare disable silences every rule for the rest of the file.
	src = "<!-- mustlint-disable -->\n" + bp8174 + "Peers MAY NOT reconnect, WILL retry, MUST not fail.\n"
	if fs := run(t, src); len(fs) != 0 {
		t.Fatalf("bare disable leaked findings: %+v", fs)
	}
}

func TestConfigDisabledRule(t *testing.T) {
	cfg := Config{Disabled: map[string]bool{"may-not": true}}
	fs := byRule(runCfg(t, bp8174+"Peers MAY NOT reconnect.\n", cfg), "may-not")
	if len(fs) != 0 {
		t.Fatalf("--disable may-not ignored: %+v", fs)
	}
}

func TestFindingsSortedByPosition(t *testing.T) {
	src := bp8174 + "Peers MAY NOT reconnect and MUST not stall.\n\nThe server WILL log.\n"
	fs := run(t, src)
	if len(fs) < 3 {
		t.Fatalf("findings = %+v, want at least 3", fs)
	}
	for i := 1; i < len(fs); i++ {
		a, b := fs[i-1], fs[i]
		if a.Line > b.Line || (a.Line == b.Line && a.Col > b.Col) {
			t.Fatalf("findings out of order: %+v before %+v", a, b)
		}
	}
}

func TestSeverityParsingAndRuleRegistry(t *testing.T) {
	for name, sev := range map[string]Severity{"error": Error, "warning": Warning, "info": Info} {
		got, ok := ParseSeverity(name)
		if !ok || got != sev {
			t.Errorf("ParseSeverity(%q) = %v, %v", name, got, ok)
		}
		if sev.String() != name {
			t.Errorf("String() = %q, want %q", sev.String(), name)
		}
	}
	if _, ok := ParseSeverity("fatal"); ok {
		t.Error("ParseSeverity accepted an unknown name")
	}
	for _, r := range All {
		if !KnownRule(r.ID) {
			t.Errorf("registry rule %q not known", r.ID)
		}
	}
	if KnownRule("no-such-rule") {
		t.Error("KnownRule accepted a bogus ID")
	}
}
