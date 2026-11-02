#!/usr/bin/env bash
# Regression test for the tpuf_ key gate in ci/checks.sh.
#
# Asserts that a seeded key makes the gate fail, is reported as file:line, and
# is never echoed in full — the scanner must not become a second leak.
set -uo pipefail

repo_root=$(git rev-parse --show-toplevel 2>/dev/null || true)
if [ -z "$repo_root" ]; then
  repo_root=$(cd "$(dirname "$0")/.." && pwd)
fi

tmp=$(mktemp -d "${TMPDIR:-/tmp}/tpuf-checks-selftest.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

# Build the fake key at runtime so this script does not itself match the
# pattern the gate looks for.
secret="tpuf_$(printf 'A%.0s' {1..32})"

git -C "$tmp" init -q
git -C "$tmp" config user.email selftest@example.invalid
git -C "$tmp" config user.name selftest
cp "$repo_root/ci/checks.sh" "$tmp/checks.sh"
chmod +x "$tmp/checks.sh"
printf 'api_key = "%s"\n' "$secret" > "$tmp/leak.tf"
git -C "$tmp" add -A

set +e
out=$(cd "$tmp" && ./checks.sh 2>&1)
status=$?
set -e

fail=0
if [ "$status" -eq 0 ]; then
  echo "FAIL: checks.sh exited 0 with a seeded tpuf_ key"
  fail=1
fi
case "$out" in
  *"leak.tf:1"*) ;;
  *)
    echo "FAIL: checks.sh did not report the hit as leak.tf:1"
    echo "$out"
    fail=1
    ;;
esac
case "$out" in
  *"$secret"*)
    echo "FAIL: checks.sh echoed the secret value"
    fail=1
    ;;
esac

if [ "$fail" -eq 0 ]; then
  echo "ok: checks.sh rejects seeded keys without echoing them"
fi
exit "$fail"
