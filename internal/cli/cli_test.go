// In-process CLI integration tests: Run(argv, stdout, stderr) is exercised
// exactly as main() would, against real files in t.TempDir(), asserting on
// output and exit codes together.
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goodSpec lints clean: modern boilerplate, IDs, no ambiguity.
const goodSpec = `# Frame Relay Protocol

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT",
"SHOULD", "SHOULD NOT", "RECOMMENDED", "NOT RECOMMENDED", "MAY", and
"OPTIONAL" in this document are to be interpreted as described in BCP 14
[RFC2119] [RFC8174] when, and only when, they appear in all capitals, as
shown here.

REQ-1: The relay MUST forward every frame within 50 ms.

REQ-2: The relay MAY batch frames smaller than 128 bytes.
`

// badSpec trips several rules on known lines.
const badSpec = `# Widget Protocol

Devices MUST register before use.

Devices MUST not skip registration.

Peers MAY NOT reconnect after eviction.
`

// write creates a file inside dir and returns its path.
func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// runCLI invokes the CLI and captures both streams.
func runCLI(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestVersionSubcommandAndFlag(t *testing.T) {
	for _, arg := range []string{"version", "--version", "-v"} {
		code, out, _ := runCLI(arg)
		if code != ExitOK || out != "mustlint 0.1.0\n" {
			t.Errorf("%s: code=%d out=%q", arg, code, out)
		}
	}
}

func TestHelpListsSubcommands(t *testing.T) {
	code, out, _ := runCLI("--help")
	if code != ExitOK {
		t.Fatalf("help exit = %d", code)
	}
	for _, want := range []string{"check", "stats", "rules", "--fail-on", "--require-ids"} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q", want)
		}
	}
}

func TestUsageErrorsExitTwo(t *testing.T) {
	p := write(t, t.TempDir(), "s.md", goodSpec)
	cases := map[string][]string{
		"no args":        {},
		"unknown flag":   {"--frobnicate"},
		"bad format":     {"check", "--format", "yaml", p},
		"bad fail-on":    {"check", "--fail-on", "fatal", p},
		"bad disable":    {"check", "--disable", "no-such-rule", p},
		"bad id-pattern": {"check", "--id-pattern", "([", p},
		"check no files": {"check"},
	}
	for name, args := range cases {
		if code, _, errOut := runCLI(args...); code != ExitUsage {
			t.Errorf("%s: exit = %d (stderr %q), want %d", name, code, errOut, ExitUsage)
		}
	}
}

func TestSubcommandHelpExitsZero(t *testing.T) {
	// An explicit help request is not a usage error: flag's -h/--help must
	// print the flag reference and exit 0, unlike a genuinely bad flag.
	for _, args := range [][]string{{"check", "--help"}, {"check", "-h"}, {"stats", "--help"}} {
		if code, _, errOut := runCLI(args...); code != ExitOK || !strings.Contains(errOut, "-format") {
			t.Errorf("%v: exit = %d (stderr %q), want 0 with flag help", args, code, errOut)
		}
	}
}

func TestRuntimeErrorsExitThree(t *testing.T) {
	code, _, errOut := runCLI("check", filepath.Join(t.TempDir(), "absent.md"))
	if code != ExitRuntime || errOut == "" {
		t.Errorf("missing file: code=%d stderr=%q", code, errOut)
	}
	code, _, errOut = runCLI("check", t.TempDir())
	if code != ExitRuntime || !strings.Contains(errOut, "no Markdown files") {
		t.Errorf("empty dir: code=%d stderr=%q", code, errOut)
	}
}

func TestCleanSpecExitsZero(t *testing.T) {
	p := write(t, t.TempDir(), "s.md", goodSpec)
	code, out, _ := runCLI("check", p)
	if code != ExitOK {
		t.Fatalf("exit = %d, output:\n%s", code, out)
	}
	if !strings.Contains(out, "no findings") {
		t.Errorf("clean output = %q", out)
	}
	// --quiet prints nothing at all on a clean run.
	if _, out, _ = runCLI("check", "--quiet", p); out != "" {
		t.Errorf("quiet clean run printed %q", out)
	}
}

func TestFindingsExitOneAndBarePathDefaultsToCheck(t *testing.T) {
	p := write(t, t.TempDir(), "s.md", badSpec)
	code, out, _ := runCLI("check", p)
	if code != ExitFail {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitFail, out)
	}
	for _, want := range []string{"mixed-case-keyword", "may-not", "missing-boilerplate"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing rule %q:\n%s", want, out)
		}
	}
	// A bare path (no subcommand) runs check.
	code, out2, _ := runCLI(p)
	if code != ExitFail || out2 != out {
		t.Errorf("bare path differs from check: code=%d", code)
	}
}

func TestFailOnThresholds(t *testing.T) {
	// missing-boilerplate + ambiguous-term are warnings; no errors here.
	p := write(t, t.TempDir(), "s.md", "Devices MUST reply in a timely manner.\n")
	if code, out, _ := runCLI("check", "--fail-on", "error", p); code != ExitOK {
		t.Errorf("exit = %d, want 0 (warnings only, threshold error)\n%s", code, out)
	}
	if code, _, _ := runCLI("check", p); code != ExitFail {
		t.Errorf("default threshold should fail on warnings")
	}
	bad := write(t, t.TempDir(), "bad.md", badSpec)
	if code, _, _ := runCLI("check", "--fail-on", "never", bad); code != ExitOK {
		t.Errorf("--fail-on never must always exit 0")
	}
}

func TestDisableFlagSuppressesRule(t *testing.T) {
	p := write(t, t.TempDir(), "s.md", badSpec)
	_, out, _ := runCLI("check", "--disable", "may-not", "--fail-on", "never", p)
	if strings.Contains(out, "may-not") {
		t.Errorf("--disable may-not ignored:\n%s", out)
	}
	if !strings.Contains(out, "mixed-case-keyword") {
		t.Errorf("other rules were lost:\n%s", out)
	}
}

func TestRequireIDsAndCustomPattern(t *testing.T) {
	p := write(t, t.TempDir(), "s.md", goodSpec+"\nThe relay SHOULD retry once.\n")
	_, out, _ := runCLI("check", "--require-ids", "--fail-on", "never", p)
	if !strings.Contains(out, "missing-id") {
		t.Errorf("--require-ids did not enable missing-id:\n%s", out)
	}
	// With a custom pattern, [R-9] satisfies the rule but REQ-1/REQ-2 no
	// longer count as IDs.
	src := goodSpec + "\n[R-9] The relay SHOULD retry once.\n"
	p2 := write(t, t.TempDir(), "s2.md", src)
	_, out, _ = runCLI("check", "--require-ids", "--id-pattern", `\bR-[0-9]+\b`,
		"--fail-on", "never", p2)
	if got := strings.Count(out, "missing-id"); got != 2 {
		t.Errorf("missing-id count = %d, want 2 (REQ-marked lines):\n%s", got, out)
	}
}

func TestJSONOutputParses(t *testing.T) {
	p := write(t, t.TempDir(), "s.md", badSpec)
	code, out, _ := runCLI("check", "--format", "json", p)
	if code != ExitFail {
		t.Fatalf("exit = %d, want 1", code)
	}
	var rep struct {
		Tool     string `json:"tool"`
		Findings []struct {
			Rule string `json:"rule"`
			Line int    `json:"line"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("JSON output invalid: %v", err)
	}
	if rep.Tool != "mustlint" || len(rep.Findings) == 0 {
		t.Errorf("report = %+v", rep)
	}
}

func TestGitHubOutputFormat(t *testing.T) {
	p := write(t, t.TempDir(), "s.md", badSpec)
	_, out, _ := runCLI("check", "--format", "github", "--fail-on", "never", p)
	if !strings.Contains(out, "::error file=") {
		t.Errorf("github format missing workflow commands:\n%s", out)
	}
}

func TestDirectoryWalkFiltersAndSorts(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "b.md", badSpec)
	write(t, dir, "a.markdown", badSpec)
	write(t, dir, "notes.txt", "MUST not — not markdown, must be skipped\n")
	write(t, dir, ".hidden/h.md", badSpec)
	write(t, dir, "sub/c.md", badSpec)
	code, out, _ := runCLI("check", "--fail-on", "never", dir)
	if code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, "3 files checked") {
		t.Errorf("want 3 files (md/markdown, not txt/hidden):\n%s", out)
	}
	// Findings must come out sorted by path.
	iA := strings.Index(out, "a.markdown")
	iB := strings.Index(out, string(filepath.Separator)+"b.md")
	iC := strings.Index(out, "c.md")
	if !(iA < iB && iB < iC) {
		t.Errorf("files not sorted: a=%d b=%d c=%d\n%s", iA, iB, iC, out)
	}
}

func TestDuplicateIDAcrossFilesViaCLI(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.md", goodSpec)
	write(t, dir, "b.md", goodSpec) // same REQ-1/REQ-2 → cross-file duplicates
	_, out, _ := runCLI("check", "--fail-on", "never", dir)
	if !strings.Contains(out, "duplicate-id") {
		t.Errorf("cross-file duplicate-id not reported:\n%s", out)
	}
	if !strings.Contains(out, "duplicate-requirement") {
		t.Errorf("cross-file duplicate-requirement not reported:\n%s", out)
	}
}

func TestStatsSubcommand(t *testing.T) {
	p := write(t, t.TempDir(), "s.md", goodSpec)
	code, out, _ := runCLI("stats", p)
	if code != ExitOK || !strings.Contains(out, "MUST") {
		t.Fatalf("stats text: code=%d out=%q", code, out)
	}
	code, out, _ = runCLI("stats", "--format", "json", p)
	if code != ExitOK {
		t.Fatalf("stats json exit = %d", code)
	}
	var rep struct {
		Files []struct {
			Keywords map[string]int `json:"keywords"`
			Defined  int            `json:"requirement_ids"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("stats JSON invalid: %v", err)
	}
	if len(rep.Files) != 1 || rep.Files[0].Keywords["MUST"] != 1 || rep.Files[0].Defined != 2 {
		t.Errorf("stats = %+v", rep)
	}
	// stats has no github format.
	if code, _, _ := runCLI("stats", "--format", "github", p); code != ExitUsage {
		t.Errorf("stats accepted github format")
	}
}

func TestRulesSubcommandListsAllRules(t *testing.T) {
	code, out, _ := runCLI("rules")
	if code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	for _, want := range []string{"may-not", "duplicate-id", "ambiguous-term", "missing-boilerplate"} {
		if !strings.Contains(out, want) {
			t.Errorf("rules listing missing %q", want)
		}
	}
}

func TestInlineDirectiveEndToEnd(t *testing.T) {
	src := goodSpec + "\n<!-- mustlint-disable-next-line may-not -->\nREQ-3: Peers MAY NOT reconnect.\n"
	p := write(t, t.TempDir(), "s.md", src)
	code, out, _ := runCLI("check", p)
	if code != ExitOK {
		t.Errorf("directive not honored end-to-end: code=%d\n%s", code, out)
	}
}
