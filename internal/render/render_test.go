// Tests for the three output formats. Output is a public contract —
// scripts grep the text form, CI parses the JSON form, GitHub parses the
// workflow commands — so these assert exact shapes.
package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/JaydenCJ/mustlint/internal/rules"
	"github.com/JaydenCJ/mustlint/internal/spec"
)

func sample() []rules.Finding {
	return []rules.Finding{
		{File: "a.md", Line: 3, Col: 7, Rule: "may-not", Severity: rules.Error,
			Message: `"MAY NOT" is ambiguous`},
		{File: "a.md", Line: 9, Col: 1, Rule: "ambiguous-term", Severity: rules.Warning,
			Message: "vague"},
		{File: "b.md", Line: 2, Col: 4, Rule: "id-gap", Severity: rules.Info,
			Message: "gap"},
	}
}

func TestTextFormatLinesAndSummary(t *testing.T) {
	var buf bytes.Buffer
	WriteText(&buf, sample(), 2, false)
	out := buf.String()
	if !strings.Contains(out, "a.md:3:7  error    may-not") {
		t.Errorf("finding line malformed:\n%s", out)
	}
	if !strings.Contains(out, "2 files checked: 3 findings (1 error, 1 warning, 1 info)") {
		t.Errorf("summary malformed:\n%s", out)
	}
}

func TestTextCleanRunSummary(t *testing.T) {
	var buf bytes.Buffer
	WriteText(&buf, nil, 1, false)
	if got := buf.String(); got != "1 file checked: no findings\n" {
		t.Errorf("clean summary = %q", got)
	}
}

func TestQuietOmitsSummary(t *testing.T) {
	var buf bytes.Buffer
	WriteText(&buf, sample(), 2, true)
	if strings.Contains(buf.String(), "checked") {
		t.Errorf("quiet output still has a summary:\n%s", buf.String())
	}
	buf.Reset()
	WriteText(&buf, nil, 1, true)
	if buf.Len() != 0 {
		t.Errorf("quiet clean run printed %q, want nothing", buf.String())
	}
}

func TestJSONEnvelopeRoundTrips(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSON(&buf, sample(), 2); err != nil {
		t.Fatal(err)
	}
	var rep struct {
		Tool          string `json:"tool"`
		Version       string `json:"version"`
		SchemaVersion int    `json:"schema_version"`
		FilesChecked  int    `json:"files_checked"`
		Summary       Summary
		Findings      []struct {
			File     string `json:"file"`
			Line     int    `json:"line"`
			Severity string `json:"severity"`
			Rule     string `json:"rule"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &rep); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if rep.Tool != "mustlint" || rep.SchemaVersion != 1 || rep.FilesChecked != 2 {
		t.Errorf("envelope = %+v", rep)
	}
	if len(rep.Findings) != 3 || rep.Findings[0].Severity != "error" {
		t.Errorf("findings = %+v", rep.Findings)
	}
	if rep.Summary.Errors != 1 || rep.Summary.Total != 3 {
		t.Errorf("summary = %+v", rep.Summary)
	}
	// A clean run must marshal findings as [], never null.
	buf.Reset()
	if err := WriteJSON(&buf, nil, 1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"findings": []`) {
		t.Errorf("empty findings must marshal as [], got:\n%s", buf.String())
	}
}

func TestGitHubFormatAndEscaping(t *testing.T) {
	fs := []rules.Finding{{
		File: "a.md", Line: 3, Col: 7, Rule: "may-not", Severity: rules.Error,
		Message: "50% odd\nsecond line",
	}}
	var buf bytes.Buffer
	WriteGitHub(&buf, fs)
	got := buf.String()
	want := "::error file=a.md,line=3,col=7,title=mustlint may-not::50%25 odd%0Asecond line\n"
	if got != want {
		t.Errorf("github line = %q, want %q", got, want)
	}
	// Property values additionally escape ":" and "," — a comma in a file
	// path would otherwise truncate the property list.
	buf.Reset()
	WriteGitHub(&buf, []rules.Finding{{
		File: "notes, v2.md", Line: 1, Col: 1, Rule: "may-not", Severity: rules.Error,
		Message: "m",
	}})
	if got := buf.String(); !strings.Contains(got, "file=notes%2C v2.md,") {
		t.Errorf("property escaping missing: %q", got)
	}
	// Severity mapping: error/warning stay, info becomes notice.
	buf.Reset()
	WriteGitHub(&buf, sample())
	out := buf.String()
	for _, want := range []string{"::error ", "::warning ", "::notice "} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestSummarizeCounts(t *testing.T) {
	s := Summarize(sample())
	if s.Errors != 1 || s.Warnings != 1 || s.Infos != 1 || s.Total != 3 {
		t.Errorf("summary = %+v", s)
	}
}

func TestStatsTextTableWithTotals(t *testing.T) {
	all := []spec.Stats{
		{File: "a.md", Keywords: map[string]int{"MUST": 2, "SHALL": 1}, Normative: 3, Defined: 2},
		{File: "b.md", Keywords: map[string]int{"MAY": 1}, Normative: 1, Defined: 0},
	}
	var buf bytes.Buffer
	WriteStatsText(&buf, all)
	out := buf.String()
	if !strings.Contains(out, "file") || !strings.Contains(out, "total") {
		t.Errorf("header or totals row missing:\n%s", out)
	}
	// SHALL is not a table column; it must fold into "other".
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 4 {
		t.Fatalf("lines = %d, want 4 (header + 2 files + total)", len(lines))
	}
	if !strings.Contains(lines[3], "total") {
		t.Errorf("last line is not totals: %q", lines[3])
	}
	// A single file needs no totals row.
	buf.Reset()
	WriteStatsText(&buf, all[:1])
	if strings.Contains(buf.String(), "total") {
		t.Errorf("single-file stats should skip the totals row:\n%s", buf.String())
	}
}

func TestStatsJSON(t *testing.T) {
	all := []spec.Stats{{File: "a.md", Keywords: map[string]int{"MUST": 2}, Normative: 2, Defined: 1}}
	var buf bytes.Buffer
	if err := WriteStatsJSON(&buf, all); err != nil {
		t.Fatal(err)
	}
	var rep struct {
		Tool  string `json:"tool"`
		Files []struct {
			File      string         `json:"file"`
			Keywords  map[string]int `json:"keywords"`
			Normative int            `json:"normative_sentences"`
		} `json:"files"`
	}
	if err := json.Unmarshal(buf.Bytes(), &rep); err != nil {
		t.Fatalf("stats JSON invalid: %v", err)
	}
	if rep.Tool != "mustlint" || len(rep.Files) != 1 || rep.Files[0].Keywords["MUST"] != 2 {
		t.Errorf("stats JSON = %+v", rep)
	}
}
