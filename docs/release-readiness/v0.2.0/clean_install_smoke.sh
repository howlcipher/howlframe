#!/bin/bash
# Clean install of the published v0.2.0 Linux x86-64 archive, following docs/installation.md,
# then a standalone program exercise. Runs with a fresh HOME, a PATH without Go, and no dev binary.
set -u
step() { printf '\n### %s\n' "$*"; }
fail=0; check() { if eval "$2"; then echo "PASS: $1"; else echo "FAIL: $1"; fail=1; fi; }

step "environment"
echo "HOME=$HOME"; echo "PATH=$PATH"
check "no go on PATH" '! command -v go >/dev/null 2>&1'
check "no howlframe on PATH before install" '! command -v howlframe >/dev/null 2>&1'

step "install (docs/installation.md, Linux; install target is ~/bin instead of sudo /usr/local/bin)"
mkdir -p "$HOME/dl" "$HOME/bin" && cd "$HOME/dl"
VERSION=v0.2.0
curl -fsSLO https://github.com/howlcipher/howlframe/releases/download/${VERSION}/howlframe_${VERSION}_linux_amd64.tar.gz
curl -fsSLO https://github.com/howlcipher/howlframe/releases/download/${VERSION}/SHA256SUMS
check "sha256sum --check --ignore-missing" 'sha256sum --check --ignore-missing SHA256SUMS'
tar -xzf howlframe_${VERSION}_linux_amd64.tar.gz
install -m 0755 howlframe "$HOME/bin/howlframe"
check "howlframe resolves to the installed binary" '[ "$(command -v howlframe)" = "$HOME/bin/howlframe" ]'
echo "installed sha256: $(sha256sum "$HOME/bin/howlframe" | cut -d" " -f1)"

step "howlframe version"
howlframe version | tee "$HOME/version.txt"
check "version 0.2.0 / HFBC format 2" '[ "$(cat $HOME/version.txt)" = "$(printf "HowlFrame 0.2.0\nHFBC format: 2")" ]'
step "howlframe help (first 3 lines)"
howlframe help | head -3
check "help names 0.2.0" 'howlframe help | head -1 | grep -qx "HowlFrame 0.2.0 CLI"'
step "howlframe interop --json"
howlframe interop --json > "$HOME/interop.json"; head -6 "$HOME/interop.json"
check "interop schema howl.interop/v1" 'grep -q "\"schema\": \"howl.interop/v1\"" $HOME/interop.json'
check "interop version 0.2.0" 'grep -q "\"version\": \"0.2.0\"" $HOME/interop.json'
check "interop hfbc_format 2" 'grep -q "\"hfbc_format\": \"2\"" $HOME/interop.json'

step "standalone program"
W="$HOME/work"; mkdir -p "$W" && cd "$W"
cat > hello.howl <<'H'
(cli_app
  (print "Hello from HowlFrame"))
H
check "check hello.howl" 'howlframe check hello.howl'
check "build hello.howl" 'howlframe build hello.howl'
check "hello.hfbc exists and starts with HFBC" '[ "$(head -c 4 hello.hfbc)" = "HFBC" ]'
echo "hello.hfbc format word: $(od -An -tu4 -j4 -N4 hello.hfbc | tr -d " ")"
check "run hello.hfbc prints greeting" '[ "$(howlframe run hello.hfbc)" = "Hello from HowlFrame" ]'

step "invalid source"
cat > bad.howl <<'H'
(cli_app
  (print (+ 1 "two")))
H
howlframe check bad.howl; rc=$?; echo "exit=$rc"
check "type error rejected by check" '[ $rc -ne 0 ]'
cat > unbalanced.howl <<'H'
(cli_app
  (print "missing paren")
H
howlframe check unbalanced.howl; rc=$?; echo "exit=$rc"
check "parse error rejected by check" '[ $rc -ne 0 ]'
check "build refuses invalid source and writes no artifact" '! howlframe build bad.howl 2>/dev/null && [ ! -e bad.hfbc ]'

step "capabilities"
mkdir -p data && echo "secret-ish fixture" > data/note.txt
cat > reader.howl <<'H'
(cli_app
  (print (bytes_to_string (read_file "data/note.txt"))))
H
howlframe build reader.howl >/dev/null
echo "inspect --caps: $(howlframe inspect --caps reader.hfbc)"
howlframe run reader.hfbc; rc=$?; echo "exit=$rc"
check "read_file denied without a grant" '[ $rc -ne 0 ]'
check "read_file allowed with filesystem:read=<data dir>" '[ "$(howlframe run --allow-caps filesystem:read=$W/data reader.hfbc)" = "secret-ish fixture" ]'
echo "outside" > outside.txt
cat > escape.howl <<'H'
(cli_app
  (print (bytes_to_string (read_file "outside.txt"))))
H
howlframe build escape.howl >/dev/null
howlframe run --allow-caps filesystem:read=$W/data escape.hfbc; rc=$?; echo "exit=$rc"
check "read outside the granted root denied" '[ $rc -ne 0 ]'
ID=$(readlink -f /usr/bin/id); echo "exec target (canonical): $ID"
cat > runner.howl <<H
(cli_app
  (print (bytes_to_string (exec "$ID"))))
H
howlframe build runner.howl >/dev/null
howlframe run runner.hfbc; rc=$?; echo "exit=$rc"
check "exec denied without a grant" '[ $rc -ne 0 ]'
howlframe run --allow-caps process runner.hfbc; rc=$?; echo "exit=$rc"
check "bare process grant does not authorize exec" '[ $rc -ne 0 ]'
check "process:exec=<canonical path> authorizes exactly that program" 'howlframe run --allow-caps "process:exec=$ID" runner.hfbc | grep -q "uid="'

step "instruction budget"
cat > spin.howl <<'H'
(cli_app
  (let (i 0)
    (while true
      (set i (+ i 1)))))
H
check "check spin.howl" 'howlframe check spin.howl'
howlframe build spin.howl >/dev/null
timeout 60 howlframe run --max-instructions 10000 spin.hfbc; rc=$?; echo "exit=$rc"
check "unbounded loop stopped by the instruction limit (not by timeout)" '[ $rc -ne 0 ] && [ $rc -ne 124 ]'

printf '\nRESULT: %s\n' "$([ $fail = 0 ] && echo ALL PASS || echo FAILURES)"
exit $fail
