# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-07-12

### Added

- Prose-aware Markdown analysis: fenced/indented code, inline code spans,
  HTML comments, link destinations, autolinked and bare URLs, list and
  blockquote markers, tables, setext/ATX headings, and YAML front matter
  are scrubbed byte-for-byte so every finding lands on an exact
  line:column of the raw source.
- Token-based BCP 14 keyword matching: all eleven RFC 2119 keywords,
  compounds across line wraps, case classification (upper / lower /
  title / mixed), the undefined `MAY NOT`, and pseudo-normative all-caps
  words (`WILL`, `MIGHT`, `CANNOT`, `MANDATORY`, …).
- Twelve rules across four groups: boilerplate presence, RFC 8174
  currency and declared-keyword coverage; keyword misuse (mixed-case
  compounds, `MAY NOT`, pseudo-keywords, ambiguous lowercase); requirement
  IDs (uniqueness across files, series numbering gaps, opt-in
  `--require-ids` coverage with section-heading inheritance); duplicated
  requirement text and ~30 vague qualifiers inside normative sentences.
- `check` subcommand with text, JSON (`schema_version: 1`), and GitHub
  annotation output; `--fail-on` severity gate with documented exit
  codes; `--disable`, `--id-pattern`, `--quiet`; recursive directory
  walking with deterministic ordering.
- Inline suppression via `<!-- mustlint-disable … -->`,
  `<!-- mustlint-enable … -->`, and `<!-- mustlint-disable-next-line … -->`
  HTML-comment directives, ignored inside code so docs can show them.
- `stats` subcommand: per-file keyword inventory, normative-sentence and
  requirement-ID counts, as a text table or JSON.
- `rules` subcommand listing every rule with severity and summary.
- Parallel example specs (`examples/bad-spec.md`, `examples/good-spec.md`)
  and a full rules reference (`docs/rules.md`).
- 90 deterministic offline tests (unit + in-process CLI integration) and
  `scripts/smoke.sh`.

[0.1.0]: https://github.com/JaydenCJ/mustlint/releases/tag/v0.1.0
