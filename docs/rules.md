# mustlint rules reference

mustlint analyzes the *prose* of a Markdown document: fenced and indented
code, inline code spans, HTML comments, link destinations, and URLs are
all invisible to the rules, and headings are never treated as normative
statements. A sentence is **normative** when it contains at least one
BCP 14 keyword written in capitals (or in mixed capitals, which is a bug
but plainly normative intent).

Severities: `error` > `warning` > `info`. The default `--fail-on warning`
makes errors and warnings fail the run; `info` findings never fail it
unless you pass `--fail-on info`.

## Boilerplate rules

### missing-boilerplate (warning)

The document uses uppercase RFC 2119 keywords but never declares them.
Readers cannot know whether `MUST` is normative or just loud. Fix: add the
BCP 14 paragraph — *"The key words 'MUST', 'MUST NOT', … in this document
are to be interpreted as described in BCP 14 [RFC2119] [RFC8174] when, and
only when, they appear in all capitals, as shown here."* The finding is
anchored at the first uppercase keyword.

### outdated-boilerplate (warning)

The boilerplate cites RFC 2119 alone — without the RFC 8174 "all
capitals" clause — while lowercase instances of must/shall/should exist.
Under plain RFC 2119 those lowercase words are technically normative too,
which is almost never what the author meant. Fix: adopt the RFC 8174
wording above.

### undeclared-keyword (info)

The boilerplate enumerates its key words in quotes, and the document uses
an uppercase keyword that is not in that list (the classic idnits
finding). Reported once per keyword, at its first use. If the boilerplate
does not enumerate keywords at all, this rule stays silent — there is
nothing to check against.

## Keyword-usage rules

### lowercase-keyword (info)

A lowercase or sentence-case `must`, `must not`, `shall`, `shall not`,
`should`, or `should not` under a plain RFC 2119 boilerplate. Ambiguous:
capitalize it if it is a requirement, reword it (e.g. "needs to") if not.
`may`, `required`, `optional`, and `recommended` are exempt — they are
everyday English and would drown the report. Silent under RFC 8174, which
makes lowercase words non-normative by definition.

### mixed-case-keyword (error)

A compound keyword with inconsistent capitals: `MUST not`, `must NOT`,
`Should NOT`. Diff tools, extraction scripts, and readers disagree about
what these mean. Write both words in capitals.

### may-not (error)

`MAY NOT` is not an RFC 2119 keyword and has two contradictory readings:
"is forbidden to" and "is allowed not to". Use `MUST NOT` to forbid, or
rephrase as `MAY omit`. Lowercase "may not" is plain English and ignored.

### pseudo-keyword (warning)

An all-caps word that mimics normative force but has no RFC 2119 meaning:
`WILL`, `WILL NOT`, `MIGHT`, `CANNOT`, `MANDATORY`, `OPTIONALLY`,
`FORBIDDEN`, `PROHIBITED`. Each message suggests the keyword the author
probably meant. Lowercase forms and headings are ignored. `CAN` is
deliberately not on the list — it collides with the CAN bus protocol.

## Requirement-ID rules

A requirement ID matches `--id-pattern` (default: dash-joined uppercase
tokens ending in a number, like `REQ-1`, `SEC-042`, `REQ-AUTH-17`).
Citations such as `RFC-2119`, `ISO-8601`, or `SHA-256` are excluded by a
stoplist; a custom `--id-pattern` is trusted verbatim and bypasses it.

**Definition convention:** the *first* ID in a heading or normative
sentence defines that requirement. Any further IDs in the same sentence,
and all IDs in descriptive prose, are cross-references. A heading ID also
covers the section below it (until the next heading), so the common
"`### REQ-7 Retry policy`, then several MUST sentences" style works.

### missing-id (warning, off by default)

With `--require-ids`, every normative sentence needs an inline ID or a
section-heading ID. Stable IDs are what reviews, conformance tests, and
cross-references hang on to.

### duplicate-id (error)

The same ID is defined twice — anywhere in the checked corpus, so
collisions across files are caught. The message cites the first
definition's exact position.

### id-gap (info)

A numbering hole inside a series (`REQ-2` … `REQ-5` with 3 and 4 never
defined). Usually a sign that requirements were deleted; renumbering
would break external references, so retire IDs explicitly instead.

## Duplication and ambiguity

### duplicate-requirement (warning)

Two normative sentences are identical after normalization (IDs stripped,
case folded, punctuation removed, whitespace collapsed) — a copy-paste
that will drift apart under maintenance. Checked corpus-wide. Statements
shorter than six words are exempt: brief rows repeat legitimately in
overview tables.

### ambiguous-term (warning)

A vague qualifier inside a normative sentence: *as appropriate*, *if
necessary*, *where applicable*, *best effort*, *reasonable*, *in a timely
manner*, *gracefully*, *and/or*, *etc.*, and about twenty more. Each
message carries a tailored hint ("give a concrete deadline or timeout").
Descriptive prose may hedge freely; only requirement sentences are held
to this standard.

## Suppressing findings

Three HTML-comment directives, invisible in rendered Markdown:

```markdown
<!-- mustlint-disable-next-line may-not -->
REQ-9: Participants MAY NOT be reachable during failover.

<!-- mustlint-disable id-gap ambiguous-term -->
… any number of lines …
<!-- mustlint-enable -->
```

A bare `mustlint-disable` (no rule list) silences every rule until a
matching `mustlint-enable`. Directives shown inside inline code or fenced
code blocks are documentation, not directives — mustlint ignores them
there, which is how this file can show them safely.
