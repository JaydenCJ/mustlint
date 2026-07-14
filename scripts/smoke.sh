#!/usr/bin/env bash
# End-to-end smoke test for mustlint: builds the binary, lints the shipped
# example specs plus fabricated files, and asserts on real CLI output and
# exit codes. No network, idempotent, finishes in seconds.
#
# Output is always captured into a variable before grepping — piping
# straight into `grep -q` can kill the writer with SIGPIPE under pipefail.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

fail() {
  echo "SMOKE FAIL: $*" >&2
  exit 1
}

BIN="$WORKDIR/mustlint"

echo "1. build"
(cd "$ROOT" && go build -o "$BIN" ./cmd/mustlint) || fail "go build failed"

echo "2. version matches manifest"
OUT="$("$BIN" --version)"
[ "$OUT" = "mustlint 0.1.0" ] || fail "--version mismatch: $OUT"

echo "3. bad example spec: findings and exit code 1"
set +e
OUT="$("$BIN" check "$ROOT/examples/bad-spec.md")"
CODE=$?
set -e
[ "$CODE" -eq 1 ] || fail "bad-spec should exit 1, got $CODE"
for rule in mixed-case-keyword may-not duplicate-id id-gap ambiguous-term \
            pseudo-keyword outdated-boilerplate lowercase-keyword undeclared-keyword; do
  echo "$OUT" | grep -q "$rule" || fail "bad-spec output missing rule $rule"
done
echo "$OUT" | grep -q "9 findings (3 errors, 3 warnings, 3 info)" \
  || fail "bad-spec summary wrong"

echo "4. good example spec: clean even under --require-ids"
OUT="$("$BIN" check --require-ids "$ROOT/examples/good-spec.md")"
echo "$OUT" | grep -q "no findings" || fail "good-spec should lint clean"

echo "5. JSON format is machine-readable and versioned"
JSON="$("$BIN" check --format json --fail-on never "$ROOT/examples/bad-spec.md")"
echo "$JSON" | grep -q '"tool": "mustlint"' || fail "json envelope missing"
echo "$JSON" | grep -q '"schema_version": 1' || fail "json schema_version missing"
echo "$JSON" | grep -q '"rule": "may-not"' || fail "json findings missing may-not"

echo "6. GitHub annotation format"
OUT="$("$BIN" check --format github --fail-on never "$ROOT/examples/bad-spec.md")"
echo "$OUT" | grep -q "^::error file=.*may-not" || fail "github format missing ::error"

echo "7. --disable and --fail-on gate the exit code"
"$BIN" check --fail-on never "$ROOT/examples/bad-spec.md" >/dev/null \
  || fail "--fail-on never should exit 0"
OUT="$("$BIN" check --disable may-not --fail-on never "$ROOT/examples/bad-spec.md")"
echo "$OUT" | grep -q "may-not" && fail "--disable may-not was ignored"

echo "8. inline directive suppresses a finding"
cat > "$WORKDIR/directive.md" <<'EOF'
The key words "MUST" and "MAY" in this document are to be interpreted as
described in BCP 14 [RFC2119] [RFC8174] when, and only when, they appear
in all capitals, as shown here.

<!-- mustlint-disable-next-line may-not -->
REQ-1: Participants MAY NOT be reachable during failover.
EOF
OUT="$("$BIN" check "$WORKDIR/directive.md")"
echo "$OUT" | grep -q "no findings" || fail "inline directive not honored"

echo "9. --require-ids flags ID-less requirements"
cat > "$WORKDIR/noid.md" <<'EOF'
The key words "MUST" and "MAY" in this document are to be interpreted as
described in BCP 14 [RFC2119] [RFC8174] when, and only when, they appear
in all capitals, as shown here.

The relay MUST forward every frame.
EOF
OUT="$("$BIN" check --require-ids --fail-on never "$WORKDIR/noid.md")"
echo "$OUT" | grep -q "missing-id" || fail "--require-ids did not fire"

echo "10. directory walk checks both examples"
OUT="$("$BIN" check --fail-on never "$ROOT/examples")"
echo "$OUT" | grep -q "3 files checked" || fail "directory walk wrong file count"

echo "11. stats inventory"
OUT="$("$BIN" stats "$ROOT/examples/good-spec.md")"
echo "$OUT" | grep -q "MUST" || fail "stats table missing keywords"
OUT="$("$BIN" stats --format json "$ROOT/examples/good-spec.md")"
echo "$OUT" | grep -q '"requirement_ids": 6' || fail "stats json ID count wrong"

echo "12. rules listing"
OUT="$("$BIN" rules)"
echo "$OUT" | grep -q "duplicate-id" || fail "rules listing incomplete"

echo "13. usage errors exit 2, runtime errors exit 3"
set +e
"$BIN" check --format yaml "$ROOT/examples/good-spec.md" >/dev/null 2>&1
[ $? -eq 2 ] || fail "bad --format should exit 2"
"$BIN" check "$WORKDIR/absent.md" >/dev/null 2>&1
[ $? -eq 3 ] || fail "missing file should exit 3"
set -e

echo "SMOKE OK"
