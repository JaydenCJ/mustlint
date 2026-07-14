# mustlint examples

Two parallel specifications for the same fictional protocol, offline and
self-contained.

## bad-spec.md

A draft "Beacon Pairing Protocol" seeded with one instance of most rules:
an outdated RFC 2119-only boilerplate, a mixed-case `MUST not`, a
duplicated `REQ-2`, an undeclared `SHALL`, an ambiguous "reasonable time",
a numbering gap (`REQ-2` → `REQ-5`), a `MAY NOT`, a pseudo-normative
`WILL`, and a lowercase `should`.

```bash
mustlint check examples/bad-spec.md          # 9 findings, exit code 1
mustlint check --format json examples/bad-spec.md
```

## good-spec.md

The cleaned-up counterpart: BCP 14 (RFC 8174) boilerplate, unique
consecutive requirement IDs, capitalized compound keywords, concrete
numbers instead of hedges. It exits 0 even in the strictest mode:

```bash
mustlint check --require-ids examples/good-spec.md   # no findings, exit 0
```

Diff the two files to see every fix side by side:

```bash
diff examples/bad-spec.md examples/good-spec.md
```

Both files are plain Markdown with pinned content, so the findings (rule,
line, column) are identical on every machine.
