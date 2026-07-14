# Contributing to mustlint

Issues, discussions and pull requests are all welcome.

## Getting started

You need Go ≥1.22; nothing else — no runtime dependencies, no services.

```bash
git clone https://github.com/JaydenCJ/mustlint && cd mustlint
go build ./...
go test ./...
bash scripts/smoke.sh
```

`scripts/smoke.sh` builds the binary and drives every subcommand against
the shipped example specs and fabricated temp files, asserting on real
output and exit codes; it must finish by printing `SMOKE OK`.

## Before you open a pull request

1. `gofmt -l .` reports nothing (formatting is enforced).
2. `go vet ./...` passes with no findings.
3. `go test ./...` passes (90 deterministic tests, no network).
4. `bash scripts/smoke.sh` prints `SMOKE OK`.
5. Add tests for behavior changes; keep logic in pure, unit-testable
   modules (`mdscan`, `keyword`, `spec`, `rules` never touch the
   filesystem — only the CLI layer does I/O).

## Ground rules

- Keep dependencies at zero; adding one needs strong justification in
  the PR.
- No network calls, ever. mustlint reads the files you name and writes
  to stdout/stderr; nothing else. No telemetry.
- Determinism first: identical input must produce byte-identical output,
  including finding order — that is what makes diffs reviewable and CI
  gates trustworthy.
- Rules are data plus one small check function: a new ambiguous term is
  a table row in `internal/rules/ambiguity.go`; a new rule needs an entry
  in `rules.All`, a check function, a row in `docs/rules.md`, and tests
  for both the firing and the non-firing case.
- Code comments and doc comments are written in English.

## Reporting bugs

Include the output of `mustlint version`, the full command line, the
finding (or the finding you expected), and a minimal Markdown snippet
that reproduces it — the scanner is deterministic, so a snippet is a
complete repro. False positives are bugs; please report them with the
same care as misses.

## Security

Please do not open public issues for security problems; use GitHub's
private vulnerability reporting on this repository instead.
