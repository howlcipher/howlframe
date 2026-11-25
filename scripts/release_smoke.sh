#!/usr/bin/env bash
# Smoke-test a published HowlFrame release on the machine it runs on (Linux or macOS, amd64 or arm64).
#
#   scripts/release_smoke.sh [VERSION]        # VERSION like v0.3.0; default: the latest release
#
# It downloads the archive for this OS and CPU and SHA256SUMS from GitHub Releases, verifies the checksum,
# and runs the extracted binary by absolute path, so no installed or development howlframe is used. It then
# checks version, help, interop, a standalone program, rejection of invalid source, filesystem and exec grant
# scoping, and the instruction budget. On macOS it also records Gatekeeper behavior for the unsigned binary
# (information only, never a failure).
#
# Results go to stdout and to $SMOKE_DIR/results.md (default SMOKE_DIR: a new temporary directory). Under
# GitHub Actions the results are also appended to the job summary. Exit status: 0 when every check passed.
# Windows: use scripts/release_smoke.ps1.
#
# Portable to macOS's bash 3.2: no associative arrays, no `timeout`, no `sha256sum` assumption.
set -u

REPO="howlcipher/howlframe"
VERSION="${1:-}"
SMOKE_DIR="${SMOKE_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/howlframe-smoke.XXXXXX")}"
mkdir -p "$SMOKE_DIR"
RESULTS="$SMOKE_DIR/results.md"
: > "$RESULTS"

npass=0; nfail=0
out() { printf '%s\n' "$*" | tee -a "$RESULTS"; }
pass() { npass=$((npass + 1)); out "| PASS | $1 |"; }
fail() { nfail=$((nfail + 1)); out "| FAIL | $1 |"; }
check() { if eval "$2"; then pass "$1"; else fail "$1"; fi; }
die() { echo "release_smoke: $*" >&2; exit 2; }

# run_limited SECONDS CMD...: run CMD, kill it after SECONDS. Status 124 means it was killed.
run_limited() {
  limit=$1; shift
  "$@" & pid=$!
  ( sleep "$limit"; kill -9 "$pid" 2>/dev/null ) & watcher=$!
  wait "$pid"; rc=$?
  if kill -0 "$watcher" 2>/dev/null; then
    kill "$watcher" 2>/dev/null; wait "$watcher" 2>/dev/null
  else
    rc=124
  fi
  return $rc
}

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$@"; else shasum -a 256 "$@"; fi
}

# canonical PATH: absolute path with every symlink resolved (the form a process:exec= grant needs).
canonical() {
  if command -v realpath >/dev/null 2>&1; then realpath "$1"; return; fi
  if readlink -f / >/dev/null 2>&1; then readlink -f "$1"; return; fi
  p=$1
  while [ -L "$p" ]; do
    t=$(readlink "$p"); case $t in /*) p=$t ;; *) p=$(dirname "$p")/$t ;; esac
  done
  echo "$(cd -P "$(dirname "$p")" && pwd -P)/$(basename "$p")"
}

case "$(uname -s)" in
  Linux) OS=linux ;;
  Darwin) OS=darwin ;;
  *) die "unsupported OS $(uname -s); on Windows use scripts/release_smoke.ps1" ;;
esac
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) die "unsupported CPU $(uname -m)" ;;
esac
command -v curl >/dev/null 2>&1 || die "curl is required"

if [ -z "$VERSION" ]; then
  latest=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest") ||
    die "cannot resolve the latest release"
  VERSION=${latest##*/}
fi
case "$VERSION" in v[0-9]*) ;; *) die "VERSION must look like v0.3.0, got '$VERSION'" ;; esac
NUM=${VERSION#v}
ARCHIVE="howlframe_${VERSION}_${OS}_${ARCH}.tar.gz"
BASE="https://github.com/$REPO/releases/download/$VERSION"

out "## HowlFrame $VERSION release smoke: $OS/$ARCH"
out ""
out "- Host: \`$(uname -srm)\`"
[ "$OS" = darwin ] && out "- macOS: \`$(sw_vers -productVersion 2>/dev/null)\` ($(sw_vers -buildVersion 2>/dev/null))"
[ -n "${GITHUB_ACTIONS:-}" ] && out "- Runner: GitHub Actions \`${RUNNER_NAME:-?}\`, run ${GITHUB_SERVER_URL:-}/${GITHUB_REPOSITORY:-}/actions/runs/${GITHUB_RUN_ID:-}"
out "- Work directory: \`$SMOKE_DIR\`"
out ""
out "| Result | Check |"
out "|---|---|"

DL="$SMOKE_DIR/dl"; W="$SMOKE_DIR/work"; mkdir -p "$DL" "$W"
cd "$DL" || die "cannot enter $DL"
check "download $ARCHIVE and SHA256SUMS" \
  'curl -fsSLO "$BASE/$ARCHIVE" && curl -fsSLO "$BASE/SHA256SUMS"'
check "SHA256SUMS lists $ARCHIVE and it matches" \
  'grep -q " $ARCHIVE\$" SHA256SUMS && grep " $ARCHIVE\$" SHA256SUMS | sha256 -c - >/dev/null'
check "archive extracts to one howlframe executable" \
  'tar -xzf "$ARCHIVE" && [ -f howlframe ] && [ -x howlframe ]'
HF="$DL/howlframe"
[ -x "$HF" ] || { out ""; out "RESULT: FAILURES (no executable; $npass passed, $nfail failed)"; exit 1; }
EXE_SHA=$(sha256 "$HF" | cut -d' ' -f1)
EXE_KIND=$(file -b "$HF" 2>/dev/null | cut -c1-60)
[ -n "$EXE_KIND" ] && EXE_KIND=" ($EXE_KIND)"

check "howlframe version is $NUM / HFBC format 2" \
  '[ "$("$HF" version)" = "$(printf "HowlFrame %s\nHFBC format: 2" "$NUM")" ]'
check "help names $NUM" '"$HF" help | head -1 | grep -qx "HowlFrame $NUM CLI"'
"$HF" interop --json > "$W/interop.json" 2>&1
check "interop --json: schema howl.interop/v1, version $NUM, hfbc_format 2" \
  'grep -q "\"schema\": \"howl.interop/v1\"" "$W/interop.json" && grep -q "\"version\": \"$NUM\"" "$W/interop.json" && grep -q "\"hfbc_format\": \"2\"" "$W/interop.json"'

cd "$W" || die "cannot enter $W"
cat > hello.howl <<'H'
(cli_app
  (print "Hello from HowlFrame"))
H
check "check hello.howl" '"$HF" check hello.howl >/dev/null'
check "build hello.howl writes an HFBC artifact" '"$HF" build hello.howl >/dev/null && [ "$(head -c 4 hello.hfbc)" = "HFBC" ]'
check "run hello.hfbc prints the greeting" '[ "$("$HF" run hello.hfbc)" = "Hello from HowlFrame" ]'

cat > bad.howl <<'H'
(cli_app
  (print (+ 1 "two")))
H
cat > unbalanced.howl <<'H'
(cli_app
  (print "missing paren")
H
check "type error rejected by check" '! "$HF" check bad.howl >/dev/null 2>&1'
check "parse error rejected by check" '! "$HF" check unbalanced.howl >/dev/null 2>&1'
check "build refuses invalid source and writes no artifact" '! "$HF" build bad.howl >/dev/null 2>&1 && [ ! -e bad.hfbc ]'

mkdir -p data && echo "fixture text" > data/note.txt && echo "outside" > outside.txt
cat > reader.howl <<'H'
(cli_app
  (print (bytes_to_string (read_file "data/note.txt"))))
H
cat > escape.howl <<'H'
(cli_app
  (print (bytes_to_string (read_file "outside.txt"))))
H
"$HF" build reader.howl >/dev/null && "$HF" build escape.howl >/dev/null
check "read_file denied without a grant" '! "$HF" run reader.hfbc >/dev/null 2>&1'
check "read_file allowed with filesystem:read=<data dir>" \
  '[ "$("$HF" run --allow-caps "filesystem:read=$W/data" reader.hfbc)" = "fixture text" ]'
check "read outside the granted root denied" \
  '! "$HF" run --allow-caps "filesystem:read=$W/data" escape.hfbc >/dev/null 2>&1'

ID=$(canonical "$(command -v id)")
cat > runner.howl <<H
(cli_app
  (print (bytes_to_string (exec "$ID"))))
H
"$HF" build runner.howl >/dev/null
check "exec denied without a grant" '! "$HF" run runner.hfbc >/dev/null 2>&1'
check "bare process grant does not authorize exec" '! "$HF" run --allow-caps process runner.hfbc >/dev/null 2>&1'
check "process:exec=$ID authorizes exactly that program" \
  '"$HF" run --allow-caps "process:exec=$ID" runner.hfbc | grep -q "uid="'

cat > spin.howl <<'H'
(cli_app
  (let (i 0)
    (while true
      (set i (+ i 1)))))
H
"$HF" build spin.howl >/dev/null
run_limited 60 "$HF" run --max-instructions 10000 spin.hfbc >/dev/null 2>"$W/spin.err"; rc=$?
check "unbounded loop stopped by --max-instructions (exit $rc, not killed)" \
  '[ $rc -ne 0 ] && [ $rc -ne 124 ] && grep -q LIMIT_EXCEEDED "$W/spin.err"'

out ""
out "- Executable: \`$EXE_SHA\`$EXE_KIND"

if [ "$OS" = darwin ]; then
  # Information only. curl does not quarantine downloads; a browser does. Simulate that on a copy.
  out "- Gatekeeper: \`spctl --status\`: $(spctl --status 2>&1 | head -1)"
  out "- Gatekeeper: \`spctl -a -t exec\` on the binary: $(spctl -a -t exec -vv "$HF" 2>&1 | head -1)"
  out "- Code signature: $(codesign -dv "$HF" 2>&1 | grep -E '^(Signature|CodeDirectory)' | tr '\n' ' ')"
  cp "$HF" "$W/howlframe-quarantined"
  if xattr -w com.apple.quarantine "0083;$(printf %x "$(date +%s)");Safari;" "$W/howlframe-quarantined" 2>/dev/null; then
    run_limited 30 "$W/howlframe-quarantined" version > "$W/q.out" 2>&1; qrc=$?
    out "- Quarantined copy (as if downloaded by a browser), \`howlframe version\`: exit $qrc, output: \`$(head -1 "$W/q.out")\`"
  else
    out "- Quarantined copy: could not set the quarantine attribute"
  fi
fi

out ""
if [ $nfail -eq 0 ]; then out "RESULT: ALL PASS ($npass checks)"; else out "RESULT: FAILURES ($nfail of $((npass + nfail)) checks failed)"; fi
[ -n "${GITHUB_STEP_SUMMARY:-}" ] && cat "$RESULTS" >> "$GITHUB_STEP_SUMMARY"
[ $nfail -eq 0 ]
