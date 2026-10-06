# 2026-10-06: Path-scoped filesystem grants (S8 / P2.1)

## Findings and change

- S8 is PARTIAL-closed: the native filesystem portion is implemented. The
  original process authority finding remains open. P2.1 retains its other scopes.
- `-allow-caps filesystem:read=/data,filesystem:write=/out` accepts repeatable
  roots. Read grants authorize reads only. Write grants authorize write_file
  and mkdir only. The existing `filesystem` alias remains unrestricted.
- `internal/capability/grants.go` exposes ParseGrant and Grants. ParseGrant
  returns Capability values compatible with existing []capability.Capability
  APIs, including RunBytecode and Interpreter.AllowedCaps. Roots are Abs/Clean
  at parse time, relative to runner cwd. Direct callers should use ParseGrant.
  Scoped grants count as filesystem for coarse required-capability gates;
  path checks remain mandatory before native I/O. RequiredCapabilities still
  reports the coarse effect class, not evidence of path authorization.
- Invalid/empty roots, unknown filesystem sub-keys, and colon scopes on other
  capabilities produce the existing CLI unknown-capability error.
- `howlframe.go` parses CLI grants; `internal/vm/vm.go` enforces read_file,
  write_file, mkdir in both execution engines. Native file stores check the
  path before open/load and at storeHandle for every get/keys/put/delete.
  Database is still required. Open/get/keys need read; put/delete need both
  read and write. Memory stores are unchanged. No interpreter file-store
  implementation exists to extend.
- Containment uses filepath.Rel, rejecting parent escapes and prefix siblings.
  EvalSymlinks resolves each root and the target's longest existing ancestor;
  resolution errors and dangling symlinks fail closed. Scoped I/O uses the
  checked absolute cleaned path, preventing raw symlink/.. traversal from
  differing from the permission check. Roots themselves may be symlinks.
- Scoped receipt authorization is recorded only after all required path checks
  pass; denied paths record denied rather than a provisional coarse allowed.
- Denial precedes file I/O with the existing CAPABILITY_DENIED format in the
  VM and existing capability denial in the interpreter. Denials omit paths.

## Verification

- TestParseGrant: valid forms, invalid forms, cwd anchoring.
- TestFilesystemGrantContainment: independent read/write scopes, containment,
  prefix siblings, parent escape, unrestricted alias, symlink and dangling-link
  escape denial with nonexistent write targets.
- TestFilesystemScopesVMAndInterpreter: reads/writes/mkdir inside/outside,
  parent escape, independent scopes, coarse alias, symlink escapes, and
  unchanged files/directories after denied mutations.
- TestFileStoreFilesystemScopes: allowed put/delete, denied put/delete outside
  write coverage, read-only get/keys, denied open outside read coverage, and
  write-only open denial; denied mutations leave persisted bytes unchanged.
- TestFilesystemScopeReceiptDecision: scoped denial and success decisions,
  including read-only store mutation denial after successful open.
- TestCLIPathScopedFilesystemGrants: relative/repeated roots, coarse alias,
  write-only read denial, invalid roots/sub-keys/other-capability scopes.
- `gofmt -l .`: empty output; `go vet ./...`: passed; `go build -v ./...`:
  passed. Targeted capability/VM/CLI tests passed.
- `python3 -m unittest test_harness.py` in benchmarks/v2/harness: 8 tests
  passed. Its deliberate failing fixture attempts are expected harness probes;
  the command exits zero. `python3 scripts/test_seo.py`: all checks passed.
- Initial `go test ./...` collided with the concurrently running Python
  harness HTTP fixture on port 8080 (TestHTTPServerServeHFBC). The full Go
  suite passed after the harness finished. A subsequent `go test ./...` on
  the final code (including receipt decisions) also passed, all packages.

## Remaining work

- Process allow-list, network host scoping, and environment=VAR remain open.
- S9/P2.2 SPAWN_AGENT attenuation is unchanged; children inherit runner grants.
- Generated Go and JavaScript still use coarse howlFrameGrantHas("filesystem")
  checks through HOWLFRAME_ALLOW_CAPS. Scoped native execution does not establish
  backend parity. No production -compile-bc/HFIR default flip or #90 status change.
- Symlink checks reduce static escapes but are not atomic with I/O: concurrent
  replacement of symlinks/ancestors remains a TOCTOU risk. Hard links and host
  mount changes are not isolated by path scoping. Strong isolation needs
  descriptor-relative OS containment or a host sandbox in a later change.
- No live LLM calls, process/network scoping, commit, or push in this change.
