# Change Log

## Unreleased

### Applications
* EvidenceDesk's ledger module (`milestone3/evidencedesk/app/ledger.howl`) exports `parse_within(raw, maximum, max_bytes)`, the same validation as `parse` under a caller-chosen byte ceiling, so HowlBoard's ledger import can reuse it at HowlBoard's own input sizes. `parse` delegates with the frozen 262144-byte ceiling; EvidenceDesk's API and projector behavior is unchanged (its full workflow suite and a 14-case black-box projector comparison are identical before and after). `TestEvidenceDeskLedgerParseWithin` pins both ceilings.

### Correctness
* HF-05: a value-less form in a value position is rejected before anything runs. The checker already refused void initializers, arguments, operands and list elements (HFREC-009) but missed three positions: a `try_let` binding, a `dict` value and a `return` value. `(try_let (r (write_file ...)) ...)` passed `check`, `build`, `-compile-bc` and `-compile-hfir-bc`, wrote the file, and then failed with `STACK_UNDERFLOW`. Those three positions now report "... is a void expression and produces no value" on every compile path. The independent review of that first fix found the same defect in more positions, now also refused: arguments of simple built-ins (`valueArgumentForms`: `read_file`/`mkdir` paths, `write_file` data, `sleep`, `stderr`, conversions, `encode_json`, string and map helpers, ...), a `for` iterable, a `match` subject, an `if` without `else` or a `match` without `default` arm used as a value (no value when the condition is false or nothing matches), `store_put`/`store_delete` used as a value, and (from the second and third independent reviews) any branching form used as a value with a path that yields nothing: a `try_let` body, an `if` branch, a `match` arm, or the last form of a `do`/`let`, followed recursively (`yieldsNoValue`), because type joins turn void-or-value into `any` and hid these paths. A path that exits or returns is allowed, since it never falls through. `store_put`/`store_delete` keep their inferred type, so a helper whose last form is a store write (HowlBoard's `log_event`) still checks; typing them void falsely rejected it. The HFIR model adapter, which never runs the source checker, accepted a value-less node in *every* value role (including `let` and `print` values); `DecodeCandidate`/`CompileCandidate` now reject a `write_file`, `mkdir`, `print`, `stderr`, `exit`, `set`, `map_set`, `map_delete`, `append`, `sleep`, `for`, `while`, `defun` or `return` node, or a `let`/`sequence` ending in one, wired into a value role, with `HFIR_TRANSPORT_TYPE`; an `if` without `else`, or an `if`/`try` with a value-less branch, in a value role is refused the same way. `TestValueLessKindsMatchOpcodeTable` ties that list to the opcode table's `Pushes`. Breaking only for programs that could not run correctly: use `(do (effect ...) value)` to bind the outcome of an effect. Across 9,845 local `.howl` programs, one changed verdict: the benchmark v2 task 08 reference, which leaked its secret to `leaked.txt` and then crashed whenever the write was granted; it now binds `(do (write_file ...) true)` (46 to 50 tokens). The adapter deadline test's `caught_sleep` fixture had the same shape and now sleeps inside a `do`; its assertion is unchanged. No new opcode, form, capability or HFIR lowering change; #90 stays Partial.

### Interop
* `howlframe interop --json` prints the `howl.interop/v1` descriptor that other Howl components and `howl doctor` check (contract: `docs/INTEROP.md` in howlcipher/howl): every subcommand with its flags, the legacy single-dash flags, the HFBC format, and the exit codes. `TestInteropDescriptorMatchesFlagSets` reads the flag sets out of `howlframe.go`, so a flag added without updating the descriptor fails the build.
* The vendored `contracts/howl/` schemas are re-synced to HowlDream `dc6019a` (`howl.exploration/v1` had gained optional fields since `bd10b18`), and CI now fails when a vendored copy differs from the commit `SOURCE.md` pins or that commit is not on HowlDream's default branch (`scripts/check_vendored_schemas.py`).

### Verification
* EvidenceDesk's `tests/verify_workflows.py` runs against current HowlBoard. The superseded R5 adapter (Owner decision D) is applied only when the consumer declares no root `(grants ...)`, as at `cd2f2fd`; a Board that declares its own is built unmodified and run with exactly the authority it declares, as HowlBoard's Makefile runs it. The declaration is pinned in the script, so a Board that asks for more fails instead of being granted it, and the Board process gets only `PATH` and `NO_PROXY` from the host. `scripts/test.sh` passes against HowlBoard `0b9236b` and `cd2f2fd`; EvidenceDesk's own capability checks are unchanged.
* Publication reconciliation retains remote deferred store-lock release/timing diagnostics and interop/schema synchronization; the descriptor includes listener-host flags. Historical assurance and the separate remote M3 draft remain attributable to their original scopes.
* Publish reconciliation preserves concurrent remote store-lock hardening, timing diagnostics, Owner assurance/waiver history and the separate Milestone 3 draft. Combined Ticketdesk frozen/adversarial/historical tests and repository CI-equivalent checks are rerun before publication.
* All three fresh product milestones accepted at implementation `b028e813`, 2026-10-21: committed clean reproduction, independent final reviews, exact large-ledger arithmetic, real Chromium and actual adapted HowlBoard consumption. Closure records preserve historical failures and bounded carryover.
* Fresh requirements-first CLI/backend replacements and Milestone 3 EvidenceDesk: retained original contracts, independent adversarial findings, separate reviewer runs and committed-tree reproduction scripts. Source candidates are preserved under milestone1/rebuild and milestone2/rebuild.
* Fresh all-milestone rebuild authorized 2026-10-20; froze independent Milestone 3 EvidenceDesk and real HowlBoard projector requirements before implementation. Prior acceptance records remain historical evidence.
* Milestone 2 closure: recorded independent Round 4 acceptance, refreshed the handoff, and indexed verification evidence in `milestone2/MILESTONE2_DECISION.md`. No runtime or frozen-test changes.
* Verification jobs explicitly forbid local inference through HOWL_FORBID_LOCAL_INFERENCE=1.

### Fixed
* Derive advertised HFBC envelope format from the actual artifact writer; CLI version and interop no longer report format 1 while emitting format 2. A public CLI regression compiles a real artifact and checks all three advertised surfaces.

* Fresh Ledgerlens exact aggregate arithmetic across the complete allowed amount/row-count range; uncapped decimal budgets. Fresh Taskflow segmented expansion and side-effect-free multi-output permission preflight; fresh Healthwatch distinguishes absent and invalid empty state.
* Fresh Ticketdesk atomically publishes ticket buckets and complete token state, validates persisted state before startup mutation, and provides non-overwriting offline legacy migration.
* HTTP serial-body framing failures preserve route/auth ordering and correlated errors; ignored GET/DELETE bodies do not stall handlers. Added runner IP binding, observational scoped filesystem grant queries, and JavaScript fetch headers/component encoding required by the full-stack application.
* EXT-006: gob artifact decoding resolves stable opcode names before structural verification, matching JSON decoding. Release-era CONVERT instructions no longer become CALL after enum additions; unknown names and malformed jumps/calls remain rejected.
* EXT-007: bare relative file stores create atomic-write temporary siblings in the current directory rather than system temp, preventing cross-device rename failure.
* EXT-008: positive exec fixtures bind canonical installed executable paths in both source and exact grants. Frozen ABI source assets and explicit symlink-denial tests stay intact; the conformance runner uses isolated fixture copies.
* HOWL-E2E-UX-001 EXT-001C: browser relative fetch uses the host page URL before the existing exact-host grant checks. Node keeps its absolute-URL requirement; coarse network and cross-host requests remain denied. Existing ESM transport and process-polyfill limits are explicit in the consumer journal.
* HOWL-E2E-UX-001 EXT-001A: target validation distinguishes let/try_let bindings, function parameters and dictionary entries from executable forms. Ordinary names such as `rec` no longer prevent Board JavaScript compilation. Actual unsupported sugar remains rejected in initializer, dictionary value and function body positions.

### Breaking
* `process:env` steering families, follow-up from the independent review of #219: `process:env=NAME` also refuses the prefix families `RUSTUP_`, `DOTNET_`, `COMPLUS_` and `CORECLR_` (rustup toolchain selection; .NET startup hooks, profilers and runtime settings) and the names `RUSTFLAGS`, `RUSTDOCFLAGS`, `RUSTC`, `RUSTC_WRAPPER`, `RUSTC_WORKSPACE_WRAPPER`, `CC`, `CXX`, `CFLAGS`, `CXXFLAGS`, `CPPFLAGS`, `LDFLAGS`, `MAKEFLAGS`, `COMSPEC`, `PATHEXT` and `WGETRC`, on every path as in step 1. `TestSharedTables` and `TestJSSharedTables` now also compare the suffix table. Nearby application names (`CCACHE_DIR`, `RUST_LOG`, `DOTNETAPP`, `CC_EMAIL`) still pass. No known consumer grants any of these names.
* `process:env` steering families (#182 step 1, Owner decision H). `process:env=NAME` now also refuses, on every path (a parse error on the interpreter and `-run-bc`; generated Go and JavaScript skip the name): the prefix families `OPENSSL_`, `GLIBC_`, `MALLOC_`, `SSL_`, `CURL_`, `PYTHON`, `PIP_`, `NODE_`, `NPM_CONFIG_`, `YARN_`, `PERL`, `RUBY`, `GEM_`, `BUNDLE_`, `PHP`, `ERL_`, `JAVA_`, `JDK_` and `CARGO_`; any name ending in `_PROXY` (case-insensitive, so `https_proxy` and `NO_PROXY` too); and the names `PAGER`, `EDITOR`, `VISUAL`, `LESSOPEN`, `LESSCLOSE`, `BROWSER`, `SHELL`, `SSH_ASKPASS`, `SUDO_ASKPASS`, `TMPDIR`, `TMP`, `TEMP`, `RES_OPTIONS`, `LOCALDOMAIN`, `TERMINFO`, `TERMINFO_DIRS`, `MANPATH`, `GOPROXY`, `GOPRIVATE`, `GONOPROXY`, `GONOSUMDB`, `GONOSUMCHECK`, `GOSUMDB`, `GOINSECURE`, `GOAUTH`, `GOCACHE`, `GOMODCACHE`, `GOBIN`, `GOEXPERIMENT`, `GOWORK` and `GOVCS`. Each steers an exec child's code loading, configuration, helper programs, temporary files or network route. The audit's 19 gap names (`OPENSSL_CONF`, `GLIBC_TUNABLES`, `MALLOC_CHECK_`, `*_PROXY`, `NODE_USE_ENV_PROXY`, `GEM_PATH`, `PHPRC`, `ERL_LIBS`, `TMPDIR`, `LESSOPEN`, `PAGER`, `EDITOR`, ...) are all refused. Go toolchain names are exact so application names such as `GOAL_ID` still pass. The suffix rule is a new single-sourced table, `capability.CodeLoadingEnvSuffixes`, rendered into generated Go and JavaScript like the others; the shared parity lists (`internal/nettest/env.go`) gain the gap names and nearby allowed names (`LC_ALL`, `TERM`, `NO_COLOR`, `CI`, `GOAL_ID`, `MYPROXY`, `APP_PROXY_URL`, `TEMPLATE_DIR`, ...), and removing the suffix rule fails all three targets. No known consumer is affected: across the local Howl repositories the only granted names are test names, `LANG`, and TaskFlow's `HF_TEST_PASS`. Application-defined names still pass with an explicit grant; an allowlist (#182 step 2) is not part of this change.
* Network and env hardening follow-ups (#177) and runner-owned adapter deadline (#175). Breaking: (1) `process:env=` now refuses one single-sourced table (`capability.CodeLoadingEnvNames` + `CodeLoadingEnvPrefixes`, rendered into generated Go and JavaScript) that adds `PATH`, `HOME`, `XDG_CONFIG_HOME`/`XDG_CONFIG_DIRS`/`XDG_DATA_HOME`/`XDG_DATA_DIRS`, every `GIT_*`, `BASH_FUNC_*` and `LUA_*` name, `SHELLOPTS`, `BASHOPTS`, `PS4`, `PROMPT_COMMAND`, `IFS`, `CDPATH`, `GLOBIGNORE`, `ZDOTDIR`, `PYTHONHOME`, `PYTHONUSERBASE`, `PYTHONINSPECT`, `PYTHONWARNINGS`, `PYTHONBREAKPOINT`, `PYTHONEXECUTABLE`, `NODE_EXTRA_CA_CERTS`, `NODE_TLS_REJECT_UNAUTHORIZED`, `NODE_REPL_EXTERNAL_MODULE`, `PERLLIB`, `PERL5DB`, `GOFLAGS`, `GODEBUG`, `GOROOT`, `GOPATH`, `GOTOOLCHAIN`, `GOENV`, `JAVA_TOOL_OPTIONS`, `_JAVA_OPTIONS`, `JDK_JAVA_OPTIONS`, `CLASSPATH`, `GCONV_PATH`, `LOCPATH`, `NLSPATH`, `HOSTALIASES`, `RESOLV_HOST_CONF`, `SSL_CERT_FILE`, `SSL_CERT_DIR`, `CURL_CA_BUNDLE` and `REQUESTS_CA_BUNDLE`; grants such as `process:env=HOME` or `process:env=PATH` that used to parse now fail at parse time (interpreter, `-run-bc`) and are skipped by generated Go/JS. (2) On a redirect that changes scheme, host or effective port, every request header except `capability.RedirectSafeHeaders` (`Accept`, `Accept-Encoding`, `Accept-Language`, `Content-Type`, `Content-Length`, `Content-Encoding`, `User-Agent`) is dropped, including `Proxy-Authorization`, `Referer` and custom auth headers; previously only `Authorization` and `Cookie` were. (3) Redirect targets carrying userinfo are refused on all three targets. (4) The `localhost` opt-out now requires every resolved address to be loopback (127/8, `::1`). (5) `3fff::/20` and `5f00::/16` join `capability.SpecialPrefixes`, and IPv4-translated `::ffff:0:a.b.c.d` is decoded and checked as IPv4. (6) Node vets and pins the whole resolved address set rather than the first entry, and treats `[::ffff:a.b.c.d]` as `a.b.c.d` like core and generated Go (a grant for one now matches the other; same-literal redirects keep headers). (7) #175: `vm.ExecutionPolicy.AdapterDeadline` lets the runner raise the model-adapter wall-clock budget from the 1s default up to `vm.MaxAdapterDeadline` (30s); larger values are clamped, a shorter `Deadline` still wins, and `vm.AdapterDeadlineFlag` / `vm.ParseAdapterDeadline` give runners a `-adapter-deadline` flag that rejects zero, negative and over-cap values. Candidates cannot set it, and artifact runs are unaffected. New parity tests check that the generated Go and JS tables equal the capability tables, and the blocked/public address lists now also run against generated Go. Pins, freeze, tip-lock, arity and target parity are unchanged. Journal: [Network/env follow-ups and adapter deadline](docs/journals/2026-10-13_network_env_followups_adapter_deadline.md).
* Network destination hardening (#171, #172). On the capability client, generated Go and Node JavaScript, hostname grants now refuse DNS results in loopback (127/8, ::1), RFC1918 (10/8, 172.16/12, 192.168/16), ULA (fc00::/7), NAT64 (64:ff9b::/96), link-local and unspecified ranges at connection time. Explicit IP-literal grants and the exact name `localhost` opt out; allowed bytecode receipt effects record `private_destination_opt_out: "explicit IP-literal or localhost grant"`. Multicast (224/4, ff00::/8) is always refused, including explicit literals. Review follow-up (#176, part of #177): the blocked set is one shared table (`capability.SpecialPrefixes`) rendered into generated Go and JavaScript. It adds CGNAT 100.64/10 (including 100.100.100.200 metadata), 0/8, 192.0.0/24, 198.18/15, 240/4 (including broadcast), the documentation ranges, NAT64 64:ff9b:1::/48, Teredo 2001::/32, 100::/64, fec0::/10 and any non-global-unicast address. IPv4 inside IPv4-compatible (::a.b.c.d) and 6to4 (2002::/16) addresses is decoded and checked. A redirect may land on an IP-literal or `localhost` destination only when the request started on that same host: a hostname grant can no longer bounce into a granted private IP or localhost, and localhost cannot bounce into 127.0.0.1. HTTPS-to-HTTP redirects are refused even when both destinations are granted; Authorization and Cookie are stripped after any host, port or scheme change for the rest of the redirect chain. Ten-hop redirect caps and ignored HTTP_PROXY/HTTPS_PROXY are regression-tested on core, generated Go and JS; Node uses native HTTP requests with the checked DNS address pinned and no proxy agent, including under Node 24's NODE_USE_ENV_PROXY. JavaScript previously accepted short IPv4 grants such as `network:fetch=127.1` through WHATWG canonicalization; it now refuses ambiguous numeric grants and URLs like Go. Trailing-dot names report an invalid hostname rather than an ambiguous numeric host. Browser JavaScript retains host/redirect checks but cannot inspect DNS results. Migration: use an explicit IP-literal or `localhost:<port>` grant only when private access is intended; granting a route its own localhost port also grants access to its `/admin`. `process:env=` now also refuses PYTHONPATH, PYTHONSTARTUP, NODE_OPTIONS, NODE_PATH, BASH_ENV, ENV, PERL5LIB, PERL5OPT, RUBYOPT and RUBYLIB, case-insensitively, alongside LD_*/DYLD_*; generated children omit them even if a raw grant bypasses parsing. `apps/webhook_relay` still reads UPSTREAM_URL at runtime but its route remains pinned to 127.0.0.1:8099; changing the environment does not expand its authority. No production HFIR flip; #90 stays Partial; pins, freeze tests and target exclusions unchanged. Reference: `docs/reference/network_authority.md`; evidence: `docs/journals/2026-10-13_network_destination_hardening.md`.
* HFIR model adapter resource bounds (#169, #170): shared closed scalar `const`/`binary` subexpressions now lower once into private functions instead of exponentially expanded fragments; effects, variable reads and allocating expressions retain their evaluation sites and order. Candidates exceeding 2048 dependency levels, 65536 expanded nodes or 4194304 aggregate cached-fragment nodes fail closed with `HFIR_TRANSPORT_TOO_DEEP` / `HFIR_TRANSPORT_TOO_LARGE`, including callers that construct `Candidate` directly. Breaking for adapter consumers: in-memory programs returned by `CompileCandidate` now have a maximum one-second wall-clock budget (a shorter runner deadline still applies); a sleep beyond the remaining budget is refused immediately with `HFIR_DEADLINE_EXCEEDED`, which `try_let` cannot catch. Receipt paths report both the effective budget and typed failure. The adapter constraint is runner-owned and ephemeral; HFBC serialization remains unchanged, and artifact runners still select their execution policy. Explicit-deadline standalone VM sleeps also refuse durations beyond their remaining budget with the existing `LIMIT_EXCEEDED` code. Production HFIR compilation, #90 (Partial), target fences, SHA-256 pins, freeze tests and the Assurance tip-lock remain unchanged. Journal: [HFIR adapter resource blockers](docs/journals/2026-10-13_hfir_adapter_resource_blockers.md).
* HFIR model adapter (#88) now covers the full `-compile-hfir-bc` language. `hfir-model-adapter/v1` accepts every node kind `LowerToBytecode` lowers from source (`hfir.AdapterNodeKinds`): `defun`/`param`/`call`/`return`, `while`, `for`, `try`/`catch`, `read_file`, `write_file`, `mkdir`, `exec`, `fetch`, `parse_json`, `is_nil`, `map_keys`, `html_escape`, `attr_escape`, `regex_match`, `time_now`, `sleep` (transport role `duration`), `cli_args`, `req_query`/`req_header`/`req_path`, and `(map_get <expr> key)` via a `target` edge, on top of the Phase-1 kinds. `for`/`while` control edges are derived from validated roles like `if`; the schema still has no control-edge field. Kinds the source lowerer does not lower (`spawn`, `spawn_agent`, `grants`, `llm_generate`, `http_server`/`res*`, store/db forms, `read_line`, `http_request`, `match`, ...) still fail with `HFIR_TRANSPORT_KIND`. No new opcode, capability, grant, lowering or language form; effects reached through the adapter are checked by the same run-time grants and fail closed without them. Breaking for adapter clients: candidate bounds rise to 4096 nodes, 64 inputs per node and 1 MiB per transport (from 128 / 32 / 64 KiB); the transport type check now types BOOL consts as bool, `to_int`/`to_float` as numbers and `str_split` as a list, and accepts string `+` string, so a candidate mixing a bool into arithmetic, or a `sleep` whose duration is a known non-number, is now rejected with `HFIR_TRANSPORT_TYPE`; a `return` outside a `defun` body is rejected with `HFIR_TRANSPORT_RETURN` unless it is a straight-line main-level statement (program/sequence/let body or if branch), which source already allows; cycles are rejected (`HFIR_TRANSPORT_CYCLE`) before type inference; non-const nodes never carry `literal_kind`. New experimental flag `-hfir-adapter-roundtrip` (with `-compile-hfir-bc`) sends the source graph through the adapter (encode, strict decode, `CompileCandidate`). `TestModelAdapterCoversArityTable` requires every one of the 386 corpus programs `-compile-hfir-bc` lowers to round-trip byte for byte, and every arity-table form to be covered (126 of 159) or carry an exemption with a rejected sample (33) that the test checks stays outside `-compile-hfir-bc`. The prod-flip criteria's model-adapter row is updated; the promote-blocker fence, production `-compile-bc`, #90 (Partial) and the tip-lock are unchanged. Pinned token benchmarks unchanged.
* Network scoping (#165, #166). HowlFrame is a capability-governed runtime, not a sandbox; see `docs/reference/network_authority.md`. (1) The coarse `network` grant no longer authorizes outbound `fetch` / `http_request` on any path (interpreter, `-run-bc` for AST and HFIR artifacts, generated Go and JavaScript). Each host needs `network:fetch=<host>[:<port>]`; matching is exact (no wildcard or suffix match, IP literals only match the same canonical IP, ambiguous numeric hosts such as `127.1` are refused, an omitted port means the scheme default). Every redirect hop is re-checked, a granted DNS name may not connect to a link-local/unspecified address (cloud metadata), and proxy environment variables are ignored. Migration: replace `-allow-caps network` with `-allow-caps network,network:fetch=api.example.com` (keep `network` only if the program also serves HTTP or calls the model forms). (2) Route handlers no longer keep `network` implicitly: a route holds only its `(grants ...)` list or the server default. `res`, `res_json`, `res_header`, `http_res_header` and `req_method` no longer require any capability (`RES`, `RES_JSON`, `HTTP_RES_HEADER` and `HTTP_REQ_METHOD` now have an empty capability field). A route that fetches must list `network:fetch=` hosts, which the runner must hold (`apps/webhook_relay` now grants its upstream explicitly). (3) A denied fetch raises `CAPABILITY_DENIED` and the receipt's denied `FETCH`/`HTTP_REQUEST` effect carries `grant: "network:fetch=host:port"`; allowed fetches record the matched grant, and each redirect hop is its own effect. (4) Generated Go refuses `fetch`/`http_request` inside a route or middleware with `UNSUPPORTED_ON_TARGET`, because it cannot scope a single route. (5) A failing route handler now returns a generic `500 {"error":"internal server error"}` to the client; the runtime-error JSON is written to the server log (and receipt) only. (6) `process:env=` refuses dynamic-loader variables (`LD_*`, including `LD_PRELOAD`, and `DYLD_*`, case-insensitive) on every path, and generated Go/JS never pass them to exec children. (7) `-required-caps` reports `network:fetch=host:port` for constant-URL fetches and `network:fetch=<dynamic>` for computed URLs instead of `network`. Receipt `grant` strings are copied verbatim from the normalized grant, including `process:exec=` pattern literals; do not put secrets in grant patterns.
* Authority hardening (process allow-lists and S9 attenuation). HowlFrame is a capability-governed runtime, not a sandbox. (1) A bare `process` grant no longer authorizes `exec` on any path (interpreter, `-run-bc` for AST and HFIR artifacts, generated Go/JS): each program needs `process:exec=<absolute path>[ <pattern>...]` with exact-argv matching (`literal`, `*`, trailing `**`). Bare names (no PATH lookup), relative or non-clean paths, and symlinked program paths are refused. Migrate `-allow-caps process` + `(exec "find" ...)` to `-allow-caps 'process:exec=/usr/bin/find **'` + `(exec "/usr/bin/find" ...)`. (2) `spawn_agent` children now start with **no grants** instead of inheriting the parent's; `(spawn_agent name (grants "cap" ...) task)` hands an explicit subset, which must be covered by the parent, so nested spawns cannot escalate. A nested spawner needs `(grants "process")`. Spawning still requires coarse `process`. (3) Denials are typed `CAPABILITY_DENIED` with a reason and are recorded in the `--receipt` (EXEC / SPAWN_AGENT, decision `denied`). New `grants` construct (opaque, arity 0+). No new opcode; the child grant list rides in `SPAWN_AGENT` `string_operand_2`. Pinned benchmark results unchanged; #90 stays Partial; no production flip. Reference: `docs/reference/process_authority.md`. Review follow-ups: (4) a `*` slot no longer matches an empty argument or one starting with `-` (grant flags as literals; `\*` matches a literal `*`; `**` still accepts anything). (5) exec children start with an empty environment on every path; `process:env=NAME` passes one variable through. (6) `spawn` bodies and HTTP route handlers are attenuated like `spawn_agent`: `(spawn (grants ...) (lambda ...))` (default none) and `(route path (grants ...) (lambda ...))` (default `network` only, which a handler needs to respond; one `(grants ...)` directly under `http_server` sets the default for its routes); over-requests fail closed at `SPAWN`/`HTTP_ROUTE`. Apps updated: counter_api (server-wide `(grants "database")`, plus two equivalent simplifications to stay under the Python token count: `n` is initialized at startup, so `nz` fallbacks were dropped, and `/inc` binds the string once), rest_tasks, task_api (`database`), status_api, webhook_relay (`environment`); task_api's network-only run now fails at startup instead of per request. Receipts carry the matched exec grant; spawn denials no longer echo the requested grant; generated Go/JS refuse a non-final `**`. Interpreter denials are typed but still write no receipt (documented).

### Added
* Milestone 1 runtime additions (frozen CLI applications `milestone1/`): (1) `(read_stdin)` reads the rest of standard input byte-exact, because `read_line` cannot distinguish end of input from an empty line and strips `\r` (new opcode `READ_STDIN`, appended last; not yet lowered by the experimental `-compile-hfir-bc`, so it is in the promote-blocker fence). (2) `(exec program list)`: a single trailing list argument is the argument vector, checked against the `process:exec=` allow-list after expansion. (3) A `try_let` catch around a non-zero child exit now sees `"exit_code"` and `"output"`; the uncaught error, receipts and evidence are unchanged. Fixed: `(starts_with "" " ")` returned true (it desugars through `str_split`, and splitting `""` yields `[""]`), which made `while (starts_with s " ")` trim loops spin forever on an empty string.
* Records and JSON sugar (heldout-v3 records slice): `(rec k v …)` record literals with name keys, `(get m k …)` / `(put m k … v)` nested path read/write, `(read_json p)` / `(write_json p v)` one-call file JSON under the existing filesystem grant, `(pluck lst k)`, `(select lst k …)`, `(group_by x lst key)`, `(merge a b …)`. Desugar-only: no opcode or capability; arity entries; Go/JS reject with `UNSUPPORTED_ON_TARGET`; governed scan fails closed through every form. `parse_json` now runs under `-run`. `encode_json` already writes sorted keys, so output is stable.
* Dogfood round 4: `apps/fetch_cli`, `apps/template_cli`, `apps/stamp_api`; `(map_has)`, `(when)`/`(unless)`, n-ary `and`/`or` desugar. Suite total HowlFrame 2331 vs Python 3025 / Go 3295 / Node 2927. Journal: `docs/journals/2026-10-10_dogfood_apps_round4.md`.
* Dogfood round 3: `apps/wc_cli`, `apps/json_get`, `apps/counter_api`; density sugar `(str …)` / `(nz x d)`; `list_len` accepts string/bytes; `http_server` startup `store_open`/`store_put`/`do`/`let`. Real-app harness extended; totals HowlFrame 1907 vs Python 2395 / Go 2642 / Node 2297. Journal: `docs/journals/2026-10-10_dogfood_apps_round3.md`.
* Dogfood round 2: `apps/notes_cli`, `apps/auth_api`, `apps/grep_cli` built on standalone bytecode; `store_open` accepts a runtime URI expression; AST density sugar `(switch …)` and `(inc x)`. Real-app token harness extended with Python/Go/Node ports and parity. Journal: `docs/journals/2026-10-10_dogfood_apps_round2.md`.

### Fixed
* #159, #155: experimental HFIR bytecode (`-compile-hfir-bc`) now matches the interpreter and governed AST bytecode byte for byte on every frozen benchmark solution (v0, heldout-v0/v1/v2/v3, postcf, postrec: 268 programs; 23 diverged before). Two lowering root causes: (1) `parse_json` evaluated its body symbol onto the operand stack in addition to `PARSE_JSON` reading it by name, so the stray raw string was later popped by `WRITE_FILE` as its path (a false `CAPABILITY_DENIED` inside the grant), by `MAP_GET` as its key (`TYPE_ERROR`), or by `PRINT` (#155 printed the raw text); a content edge that is not the bound symbol now fails closed. (2) `map_get` whose dict is an expression had no HFIR shape and failed to lower; it now emits the AST compiler's exact sequence (`MAP_GET` with an empty variable operand). No opcode, capability or grant change; production `-compile-bc` is unchanged and #90 stays Partial. Every benchmark verifier and `tools/difftest` now run HFIR as a third path, and `TestHFIRBenchmarkSolutionParity` fails CI on any divergence.
* #151: float and mixed int/float `%` and `//` follow Python exactly. `%` is `math.fmod` floored to the divisor's sign (computed exactly by an existing-construct doubling/subtraction loop, so `(% MaxInt64 1e33)` is `9.223372036854776e+18`, `(% 1 0.1)` is `0.09999999999999995`, `(% 4.0 -2.0)` is `-0`), and `//` is CPython's float floor division returned as an int (`(// MaxInt64 1e33)` is `0`). Integral floats that print like ints (`189944.0`) paired with a big int no longer run the int core in float arithmetic. Inf/NaN operands fail with `VALUE_ERROR: % and // need finite operands`, and a `//` result outside int64 with `VALUE_ERROR: // result outside int64`. Mixed int/float comparisons no longer treat the float 2^63 as MinInt64 or round the int through float64. A non-numeric operand now reports `% requires numeric operands (int or float)` at the form instead of `< compares incompatible types`, and template VALUE_ERRORs (destructuring included) carry `at line L, column C`. Instruction cost (-run-bc): a 20k-iteration loop with one `%` and one `//` takes 4.30M instructions (3.17M after #149, 1.55M before #149), so about 460 iterations fit the default 100k limit; `to_base` of MinInt64 in base 2 takes about 16.2k (13.4k after #149); the exact float path loops about twice the binary-exponent gap (worst case `1e308 % 5e-324`, about 53k). Still pure sugar: no opcode or capability; Go/JS still reject with `UNSUPPORTED_ON_TARGET`.
* #149: integer `%`, `//` and `to_base` use exact int64 arithmetic over the whole range with Python floor semantics (previously one float division, wrong above 2^53; `(// (2^53-1) -3)` was already off by one). `(% MaxInt64 10)` is 7, `(// 123456789012345678 1000)` is 123456789012345, `MinInt64 // -1` is the existing `integer overflow` (not `CONVERSION_ERROR`), and `to_base` handles MinInt64. Operands inside (-2^52, 2^52) and float operands keep the old expansion. `let` destructuring of a list of the wrong length fails with `VALUE_ERROR: destructuring expects N elements`. Still pure sugar: no opcode or capability; Go/JS still reject with `UNSUPPORTED_ON_TARGET`.
* HOWL-E2E-UX-001 UX-003: browser JavaScript fetch accepts explicit host grants without reading Node process.env. Missing grants still deny before requests; Node environment authority and exit behavior are preserved. Browser bootstrap errors preserve the original failure.
* S8/P2.1 filesystem portion: native VM and interpreter enforce repeatable `filesystem:read=<root>` and `filesystem:write=<root>` grants, with symlink containment checks. File stores require read coverage and write coverage for mutations. Coarse `filesystem` remains unrestricted; process/network scoping and generated backends remain open. Journal: `docs/journals/2026-10-06_path_scoped_filesystem.md`.
* Review C3/C7: executable VM effect-gate conformance coverage; demo decisions use escaped JSON; executor capability checks precede writes. Effects remain ordered, not atomic. Journal: `docs/journals/2026-10-05_effect_gate_conformance_demo_honesty.md`.
* Checker enforces `docs/reference/NUMERIC_CONTRACT.md`: mixed int/float `+`, `-`, `*` are valid and typed float, `/` always types float, and int/float comparisons are valid (HFREC-063). Void expressions in value position (`let`, `set`, call/print/list arguments, `append`/`map_set` values) and statically provable builtin argument mismatches (`str_split`, `str_join`, `regex_match`, `list_len`) are rejected before any backend runs (HFREC-009, HFREC-010). `parse_json` bodies must be variable names outside `web_app` (HFREC-004). Generated JavaScript keeps typed parameter names (HFREC-049). Go and JavaScript `to_int` truncate toward zero like the VM; `web_app` rejects integer literals and JSON integers outside +/-(2^53-1) instead of rounding. Wasm integer `/` fails closed. Artifact format version 2 (sorted function table, deterministic bytes) still reads version 1 artifacts; version 1 readers cannot read version 2 artifacts (see `docs/reference/artifact_compatibility.md`). New corpus: `tests/parity_stabilization/`.

### Changed
* Falsifiable harness: all 60 oracles now `reference` (zero placeholders); still scaffold, no live H1 run or PASS/KILL claim. #90 remains Partial; no HFBC/runtime change. Journal: [oracle fill](docs/journals/2026-10-06_fill_harness_oracles.md).
* Review C5: optional runner-written `--receipt` v0 JSON on both bytecode runners records artifact hash, grant, limits, instruction usage, redacted capability decisions, and exit outcome. Journal: [C5 execution receipt](docs/journals/2026-10-06_c5_execution_receipt.md).
* Review C4b: bytecode cumulative allocation accounting enforces MaxMemoryBytes (64 MiB, `--max-memory-bytes`); fetch body and exec output caps default to 10 MiB (`--max-fetch-bytes`, `--max-exec-output-bytes`). Optional `--deadline` cancels fetch/exec/sleep/model calls and checks the instruction loop. Limits raise structured `LIMIT_EXCEEDED`; AST interpreter unchanged, no C5. Journal: [C4b resource limits](docs/journals/2026-10-06_c4b_resource_limits.md).
* Review C4a: bytecode `CALL` now enforces `MaxCallDepth`, with default raised 128→1000 (shared with SPAWN_AGENT nesting), and `--max-call-depth` on `-run-bc` and `run`. Recursion fails with structured `LIMIT_EXCEEDED` instead of Go stack overflow. C4b (allocation accounting, deadline, fetch/exec byte caps) not included; AST interpreter `-run` unchanged. Journal: `docs/journals/2026-10-06_c4a_max_call_depth.md`.
* Review C1/C2: `bytecode.RequiredCapabilities`, `inspect --caps` and `-required-caps` report compact JSON without execution. `check|build --profile governed` rejects excluded constructs, outside-root imports and Go/JS targets before output. Default behavior and HFBC encoding are unchanged. Journal: `docs/journals/2026-10-05_c1_c2_required_caps_governed_profile.md`.
* `SPAWN_AGENT` body EXIT/VmExit is now isolated like RETURN and other child failures, so parents and siblings continue without a completion line for the failed child. The diagnostic includes the exit code; LIMIT_EXCEEDED still propagates. Journal: [SPAWN_AGENT exit isolation](docs/journals/2026-10-05_spawn_agent_exit_isolation.md).
* `SPAWN_AGENT` children inherit argv, stdin, and the same buffered line reader, preserving depth-first input ordering and avoiding child `read_line` nil-pointer failures. Journal: [SPAWN_AGENT argv and stdin](docs/journals/2026-10-05_spawn_agent_argv_stdin.md).
* `CONFIDENCE`, `NEURAL_CIRCUIT`, and `EPHEMERAL_CIRCUIT` now require the `network` grant on `-run-bc`, and `confidence` and `lazy_synthesize` calls require it on `-run`. Before this change they reached the local model host under an empty grant (bugs.md #57). No opcode or artifact-format change; an existing artifact that uses them needs `-allow-caps network`. Production `-compile-bc` is still AST bytecode, #90 stays Partial, and the tip-lock was not retaken. Journal: `docs/journals/2026-10-05_model_call_capability_gates.md`.
* `SPAWN_AGENT` body RETURN is now isolated like other child failures, so parents and siblings continue without a completion line for the failed child. VmExit and limits still propagate. Journal: [SPAWN_AGENT return isolation](docs/journals/2026-10-05_spawn_agent_return_isolation.md).
* `SPAWN_AGENT` supports depth-first nested task bodies with shared instruction accounting and a MaxCallDepth guard. Child failures print one diagnostic and let parents and siblings continue; limits and escaping returns still propagate. Journal: `docs/journals/2026-10-05_spawn_agent_nesting_isolation.md`.
* `SPAWN_AGENT` now executes optional `(task "desc" body...)` forms synchronously in a child bytecode VM with captured variables and shared capabilities, limits, stores, and output. Description-only tasks retain the two-line stub. Body errors propagate, and `process` denial still precedes body execution. The existing opcode now carries an integer body length. Dogfood checks body output through serialized HFBC on `-compile-bc` and `build`; production compilation stays AST bytecode, #90 stays Partial, and the tip-lock was not retaken. Journal: `docs/journals/2026-10-05_spawn_agent_task_body.md`.
* The bytecode VM executes `TASK` and `SPAWN_AGENT` instead of raising `UNSUPPORTED_CONSTRUCT`. `TASK` pushes its description string; `SPAWN_AGENT` stays gated by `process`, pops a string task, and completes cooperatively, synchronously, and in-process with two deterministic `[Swarm VM]` lines per agent. A non-string task is `TYPE_ERROR`. New dogfood `examples/swarm_dogfood/swarm_dogfood.howl` writes `.hfbc` via `-compile-bc` and `build` and runs on `-run-bc -allow-caps process`. No new opcodes, no opcode metadata change, no production `-compile-bc` flip, `-compile-hfir-bc` still rejects `spawn_agent`, #90 stays Partial, and the Assurance tip-lock `4d74dbcf9654caa05e0b1d9212b15bc5398359e3` was not retaken. Journal: `docs/journals/2026-10-05_vm_task_spawn_agent.md`.
* The bare/default path no longer falls through to `gogen`/`server.go` for leftover or unnamed roots. Named roots (`cli_app`, `http_server`, `web_app`, `wasm_app`) still write `.hfbc`. Explicit `howlframe build --target=go` is unchanged. No production `-compile-bc` flip and no tip-lock retake. Journal: `docs/journals/2026-10-05_bare_path_fail_closed_unnamed.md`.
* The legacy no-flag `cli_app` path now writes the same `.hfbc` artifact that `build` defaults to: HFIR gate, then `bytecode.CompileToBytecode`. Explicit `--target=go|js|wasm` are unchanged. This does not flip production `-compile-bc` onto lowered HFIR; #90 stays Partial, and the Assurance tip-lock `4d74dbcf9654caa05e0b1d9212b15bc5398359e3` was not retaken.
* Experimental `-compile-hfir-bc` lowers `(sleep …)` onto the existing `SLEEP` opcode. The duration edge keeps the empty name. The opcode already pops 1, pushes 0, has an empty operand list, and grants nothing. Parity with production `-compile-bc` is one identical `SLEEP` after the compiled duration child, including `sleep` nested with `if`, `while`, `for`, and `defun`. The comparison does not sleep. Production `-compile-bc` is still AST bytecode. The decision stays Defer the production flip. #90 stays Partial. No new Assurance tip-lock. Journal: `docs/journals/2026-10-03_lowered_hfir_abi_sleep.md`.
* Experimental `-compile-hfir-bc` lowers `(time_now)` onto the existing `TIME_NOW` opcode. The opcode already pops 0, pushes 1, and grants nothing. There is no data edge and no operand. Parity with production `-compile-bc` is one identical `TIME_NOW` per call and no timestamp constant, including `time_now` nested with `if`, `while`, `for`, and `defun`. The comparison does not match two live Unix timestamps. Production `-compile-bc` is still AST bytecode. The decision stays Defer the production flip. #90 stays Partial. No new Assurance tip-lock. Journal: `docs/journals/2026-10-03_lowered_hfir_abi_time_now.md`.
* Fence inventory of `main` at `ffb753b970c385aadef1a05ebaa253c9186e6dbc`. Docs only. The promote-blocker fence has 27 names. `time_now`, `sleep`, and `read_line` already have capability-free opcodes (`TIME_NOW`, `SLEEP`, `READ_LINE`) and `LowerToBytecode` has no case for them. The other 24 are a host effect, more than one opcode, or no opcode of their own. `hfirRejectedParity` is empty, and that empty list is not a production flip while the fence is non-empty. This inventory does not flip production `-compile-bc`, does not mark #90 Done, and does not retake the Assurance tip-lock `4d74dbcf9654caa05e0b1d9212b15bc5398359e3`. Journal: `docs/journals/2026-10-03_lowered_hfir_fence_inventory_ffb753b.md`.
* Readiness score of `main` at `2e5c8e98b5881d202aca7b4d138460aaab3c129d` against `docs/reference/lowered_hfir_prod_flip_criteria.md`. Verdict: flip deferred. #90 stays Partial. Docs only. `hfirRejectedParity` is empty and the promote-blocker fence is still non-empty, so that empty list is not a production flip. The score does not refresh the Assurance tip-lock `4d74dbcf9654caa05e0b1d9212b15bc5398359e3` and does not flip production `-compile-bc`. Journal: `docs/journals/2026-10-03_lowered_hfir_prod_flip_score_2e5c8e98.md`.
* Experimental `-compile-hfir-bc` lowers `(regex_match pattern text)` onto the existing `REGEX_MATCH` opcode, pattern then text. The opcode already pops 2, pushes 1, and grants nothing. The conformance suite compares that path with production `-compile-bc` on representative matches, including `regex_match` nested with `if`, `while`, `for`, and `defun`. A non-string the checker cannot prove is `TYPE_ERROR` on both bytecode compilers. `tests/parity/07_strings.howl` runs on `-compile-hfir-bc`. Production `-compile-bc` is still AST bytecode. The decision stays Defer the production flip. #90 stays Partial. No new Assurance tip-lock. Journal: `docs/journals/2026-10-03_lowered_hfir_abi_regex_match.md`.
* Readiness score of `main` at `33cb13252bc8e29f4a1bb9d58ce95a82ef6c7358` against `docs/reference/lowered_hfir_prod_flip_criteria.md`. Verdict: flip deferred. #90 stays Partial. Docs only. The score does not refresh the Assurance tip-lock `4d74dbcf9654caa05e0b1d9212b15bc5398359e3` and does not flip production `-compile-bc`. Journal: `docs/journals/2026-10-03_lowered_hfir_prod_flip_score_33cb1325.md`.
* Experimental `-compile-hfir-bc` lowers `(attr_escape text)` onto the existing `ATTR_ESCAPE` opcode. The conformance suite compares that path with production `-compile-bc` on representative text, including `attr_escape` nested with `if`, `while`, `for`, and `defun`, and on a non-string `TYPE_ERROR`. `tests/parity/13_html_escape.howl` runs on `-compile-hfir-bc`. `regex_match` stays outside the lowerer. Production `-compile-bc` is still AST bytecode. The decision stays Defer the production flip. #90 stays Partial. No new Assurance tip-lock. Journal: `docs/journals/2026-10-03_lowered_hfir_abi_attr_escape.md`.
* Readiness score of `main` at `5a229d6553588f6916f9fe4d6b596cd1e6fa0de6` against `docs/reference/lowered_hfir_prod_flip_criteria.md`. Verdict: flip deferred. #90 stays Partial. The score does not refresh the Assurance tip-lock `4d74dbcf9654caa05e0b1d9212b15bc5398359e3` and does not flip production `-compile-bc`. Journal: `docs/journals/2026-10-03_lowered_hfir_prod_flip_score_5a229d65.md`.
* `README.md` and the GitHub Pages page `docs/index.html` name experimental `-compile-hfir-bc`, the mediated host effects, the deferred production flip, tip-lock `4d74dbcf` (Overall PASS, Promote DEFERRED), and point at `docs/reference/fetch_body_bytecode_design.md` and `docs/reference/lowered_hfir_prod_flip_criteria.md`. Docs only. #90 stays Partial.
* Design spike for a later `(fetch url method body)` change on both bytecode compilers together. The note is `docs/reference/fetch_body_bytecode_design.md`. Docs only. No opcode, no capability, and no production `-compile-bc` flip. The Assurance tip-lock stays `4d74dbcf9654caa05e0b1d9212b15bc5398359e3`. #90 stays Partial. Promote stays DEFERRED. Journal: `docs/journals/2026-09-30_fetch_body_bytecode_design.md`.
* Assurance (Lain) tip-lock verdict for `main` SHA `4d74dbcf9654caa05e0b1d9212b15bc5398359e3`: Overall PASS, Promote DEFERRED, #90 stays Partial. Node on that run was v20.19.2. Logs: `evidence/howl-90-assurance-tip-lock-4d74dbcf/`. Journal: `docs/journals/2026-09-30_assurance_tip_lock_4d74dbcf.md`. Production `-compile-bc` is still AST bytecode.
* Assurance tip-lock of `main` SHA `4d74dbcf9654caa05e0b1d9212b15bc5398359e3` against `docs/reference/lowered_hfir_prod_flip_criteria.md` and `TestProdFlipCriteriaLock`. The decision stays Defer. Production `-compile-bc` is still AST bytecode. #90 stays Partial. Journal: `docs/journals/2026-09-30_lowered_hfir_prod_flip_tip_lock.md`.
* Experimental `-compile-hfir-bc` lowers `(html_escape text)` onto the existing `HTML_ESCAPE` opcode. The conformance suite compares that path with production `-compile-bc` on representative text, including `html_escape` nested with `if`, `while`, `for`, and `defun`, and on a non-string `TYPE_ERROR`. `regex_match` and `attr_escape` stay outside the lowerer. Production `-compile-bc` is still AST bytecode. #90 stays Partial. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_html_escape.md`.
* The kill, defer, and promote checklist for a future production `-compile-bc` flip onto lowered HFIR bytecode is `docs/reference/lowered_hfir_prod_flip_criteria.md`. The flip stays deferred. Experimental `-compile-hfir-bc` stays the dogfood path until the Owner authorizes a flip PR. Production `-compile-bc` is still AST bytecode. #90 stays Partial. Journal: `docs/journals/2026-09-30_lowered_hfir_prod_flip_criteria.md`.
* A differential dogfood case compiles one program that nests `env`, `read_file`, `write_file`, `mkdir`, `exec`, and `fetch` inside `if`, `while`, `for`, and `defun` with experimental `-compile-hfir-bc` and production `-compile-bc`. The suite compares an empty grant, a grant that omits `network`, and the full grant. Supported `tests/parity` files run on `-compile-hfir-bc` as well. `regex_match`, `html_escape`, and `attr_escape` fail closed. The optional request body stays off `OpFetch`. Production `-compile-bc` is still AST bytecode. #90 stays Partial. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_dogfood_multi_effect.md`.
* Experimental `-compile-hfir-bc` lowers `(fetch …)` onto the existing `FETCH` opcode, URL then method, the same operand order as the AST bytecode compiler. The conformance suite compares that path with production `-compile-bc` on the `network` grant and on denial, including `fetch` nested inside `if`, `while`, `for`, and `defun`. The optional request body stays off `OpFetch`. Production `-compile-bc` is still AST bytecode. #90 stays Partial. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_dogfood_fetch.md`.
* Experimental `-compile-hfir-bc` lowers `(exec …)` onto the existing `EXEC` opcode, command then arguments, the same operand order as the AST bytecode compiler. The conformance suite compares that path with production `-compile-bc` on the `process` grant and on denial, including `exec` nested inside `if`, `while`, `for`, and `defun`. `fetch` stays outside the lowerer. Production `-compile-bc` is still AST bytecode. #90 stays Partial. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_dogfood_exec.md`.
* The lowered-HFIR conformance suite now runs experimental `-compile-hfir-bc` on the pure core, `env`, and `read_file` cases the lowerer already emits, and on a fixture that nests `env` and `read_file` inside `if`, `while`, `for`, and `defun`. `fetch` stays outside that lowerer. Production `-compile-bc` is still AST bytecode. #90 stays Partial. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_dogfood_host_control.md`.
* A differential dogfood case compiles `write_file` and `mkdir` with experimental `-compile-hfir-bc` and production `-compile-bc`, granted and denied under `filesystem`. The nested fixture places both effects inside `if`, `while`, `for`, and `defun`. The experimental lowerer emits the existing `WRITE_FILE` and `MKDIR` opcodes. Production `-compile-bc` is still AST bytecode. #90 stays Partial. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_dogfood_fs_write.md`.
* Generated Go and JavaScript mediate `(write_file)` and `(mkdir)`. An empty, missing, or non-`filesystem` `HOWLFRAME_ALLOW_CAPS` grant fails with `CAPABILITY_DENIED` before any file write or directory creation, and the denial does not include the path. The `filesystem` grant writes the file and creates the directory. The production `-compile-bc` path is unchanged. #90 stays Partial. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_phase2e_fs_write.md`.
* A differential dogfood case compiles `tests/conformance/abi_v1/14_nested_for_if_while_defun.howl` with experimental `-compile-hfir-bc` and production `-compile-bc` and requires the same stdout, stderr, and exit. The program nests `for` with `if`, `while`, and `defun`, including a `for` inside a `for`. Production `-compile-bc` is still AST bytecode. #90 stays Partial. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_dogfood_nested_for.md`.
* Experimental `-compile-hfir-bc` compiles and runs `for` with the existing `FOR_INIT`, `FOR_NEXT`, and `JUMP` opcodes. A for header's control edges are the iterable and then the body. Production `-compile-bc` is still AST bytecode after the gate. #90 stays Partial. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_phase3d_for.md`.
* A differential dogfood case compiles `tests/conformance/abi_v1/12_nested_if_while_defun.howl` with experimental `-compile-hfir-bc` and production `-compile-bc` and requires the same stdout, stderr, and exit. The program nests `if` and `while` inside `defun` and calls that function. Production `-compile-bc` is still AST bytecode. #90 stays Partial. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_dogfood_nested_control.md`.
* Experimental `-compile-hfir-bc` compiles and runs `if` with the existing `JUMP_IF_FALSE` and `JUMP` opcodes. An if node's control edges are the condition, the then-branch, and an optional else. Production `-compile-bc` is still AST bytecode after the gate. #90 stays Partial. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_phase3c_if.md`.
* Experimental `-compile-hfir-bc` compiles and runs `while` with the existing `JUMP_IF_FALSE` and `JUMP` opcodes. A while header's control edges are the condition and then the body. Production `-compile-bc` is still AST bytecode after the gate. #90 stays Partial. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_phase3b_while.md`.
* Experimental `-compile-hfir-bc` compiles and runs `defun`, `call`, and `return` with the existing `CALL` and `RETURN` opcodes. Production `-compile-bc` is still AST bytecode after the gate. #90 stays Partial. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_phase3a_defun_call.md`.
* Generated Go and JavaScript mediate `(fetch)`. An empty, missing, or non-`network` `HOWLFRAME_ALLOW_CAPS` grant fails with `CAPABILITY_DENIED` before any HTTP request, and the denial does not include the URL. The `network` grant performs the request. The production `-compile-bc` path is unchanged. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_phase2d_fetch.md`.
* Generated Go and JavaScript mediate `(read_file)`. An empty, missing, or non-`filesystem` `HOWLFRAME_ALLOW_CAPS` grant fails with `CAPABILITY_DENIED` before any filesystem read, and the denial does not include the path. The `filesystem` grant reads the file. The production `-compile-bc` path is unchanged. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_phase2c_read_file.md`.
* Generated Go and JavaScript mediate `(exec)`. An empty, missing, or non-`process` `HOWLFRAME_ALLOW_CAPS` grant fails with `CAPABILITY_DENIED` before a subprocess starts, and the denial does not include the command. The `process` grant runs it. The production `-compile-bc` path is unchanged. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_phase2b_exec.md`.
* Generated Go and JavaScript mediate `(env)`. An empty or non-`environment` `HOWLFRAME_ALLOW_CAPS` grant fails with `CAPABILITY_DENIED` before the variable is read. The `environment` grant returns the value. The production `-compile-bc` path is unchanged. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_phase2a_env.md`.
* `map_keys` order is UTF-8 byte order on the interpreter, the bytecode VM, the Go backend, and JavaScript. JavaScript no longer uses UTF-16 code-unit sort, so a supplementary-plane key sorts with the same list as Go `sort.Strings`. No new opcode and no capability change. Journal: `docs/journals/2026-09-30_dict_key_sort.md`.
* Locked the pure-versus-store capability split in docs and tests. `map_keys` grants nothing and runs under an empty grant. `store_keys` stays `database`. A `file://` store additionally requires `filesystem`. No opcode grant changed. Journal: `docs/journals/2026-09-30_capability_surface_honesty.md`.

### Added
* Review 09: offline falsifiable experiment harness and JSON broker scaffold only; no experiment results; no PASS/KILL claim; no HFBC/compiler change; #90 Partial. [Journal](docs/journals/2026-10-06_falsifiable_experiment_harness.md).
* Lowered-HFIR ABI v1 (`docs/reference/lowered_hfir_abi_v1.md`) and a conformance suite (`tests/conformance/lowered_hfir_abi_v1.json`) run by `tools/difftest` across the interpreter, the bytecode VM, Go, and JavaScript. The production compiler is still the AST bytecode path. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_phase1.md`.
* `html_escape` and `attr_escape`, with `HTML_ESCAPE` and `ATTR_ESCAPE`. Both
  encode `&`, `<`, `>`, `"`, and `'` the same way (`&amp;`, `&lt;`, `&gt;`,
  `&#34;`, `&#39;`) and grant no capability. `attr_escape` is the attribute-context
  name for that encoding. A non-string fails closed with `TYPE_ERROR`. An empty
  string stays empty. Contract tests reject a handler body that interpolates an
  untrusted id into a `<script>` string or an inline handler; data attributes
  carry the escaped text and handler names stay constants.
* `req_query`, `req_header`, and `req_path`, with `HTTP_REQ_QUERY`,
  `HTTP_REQ_HEADER`, and `HTTP_REQ_PATH`. A handler can read one query
  parameter, one request header, and one `{name}` path segment. A missing
  name is an empty string. A bad receiver or name is `TYPE_ERROR`. The reads
  grant no capability. Literal routes stay on the existing mux.
* `map_keys` construct and `MAP_KEYS` bytecode instruction. It returns every key
  of a dictionary as a sorted list and requires no capability. HowlBoard could
  not iterate the keys of a record it received, so object-shaped responses had
  to be rewritten as lists. An empty dictionary returns an empty list, and a
  non-dictionary fails closed with `TYPE_ERROR`. The same sorted enumeration is
  available to the interpreter and to the Go and JavaScript backends.
* `time_now` in the JavaScript backend. It was supported by the bytecode VM and
  the Go backend but rejected as an unknown statement for `web_app` programs, so
  a browser interface had no way to read the clock and render relative times.
* `store_keys` construct and `STORE_KEYS` bytecode instruction, returning every
  record key in a native store as a sorted list. Go randomizes map iteration, so
  enumeration is sorted to keep listing deterministic. Every prior HowlFrame
  application (`kv_cli`, `todo_cli`, `task_api`, HowlBoard) had to maintain a
  parallel index record that could silently diverge from the records it indexed;
  `store_keys` removes that workaround. Requires the `database` capability, and
  `filesystem` additionally for `file://` stores.
* Runner-sealed, bounded negative map-state provenance for the internal direct
  HFIR experiment. It records completed map mutations and reads by backing-map
  identity, proves never-present versus effectively deleted keys, and fails
  closed on ambiguous wrong-key writers, forged evidence, or bounded-history
  overflow. Durable HFBC and public AST-backed execution remain unchanged.
* Internal HFIR failure-localization evidence: direct-lowering instruction
  provenance, a runner-bounded execution trace, fail-closed behavioral and
  runtime localization, and an automatically derived repair context for the
  bounded semantic-repair experiment. Durable HFBC artifacts and public
  AST-backed builds remain unchanged.
* An internal Phase-1 `HFIR -> BCProgram` lowering path that consumes semantic
  HFIR directly, with explicit operand-role normalization, deterministic
  differential artifact/VM evidence, capability consistency checks, preserved
  compile diagnostic provenance, and fail-closed
  `HFIR_BYTECODE_UNSUPPORTED` diagnostics. Public bytecode builds continue to
  use the AST compiler while the subset expands.
* `list_len` construct and bytecode instruction `OpListLen` to measure list lengths without manual loops.
* `is_nil` intrinsic to natively detect the absence of a value (e.g., a missing key in a dict) instead of string formatting checks.
* HowlFrame Repo Analyst, a deterministic five-module standalone application that
  discovers and classifies repository files, counts tests, configuration,
  entry points, and TODO/FIXME markers, emits or writes a versioned report, and
  proves its bytecode artifact runs after imported `.howl` sources are removed.
* A typed flat SSA/CFG lowering layer in `internal/ir`, with ordered basic
  blocks, explicit branch/jump/return terminators, phi nodes for control-flow
  joins and loop-carried values, source locations, and graph validation.
* Provider-neutral constrained-decoding mask plans in `internal/masking`,
  derived from semantic checker `TypeInfo` and full `checker.Analysis` output
  for downstream tokenizer/logit integrations.
* A local parser and instruction/type validator for the folded WebAssembly Text
  subset emitted by the Wasm backend, including function results, operands,
  control flow, memory access, constants, and data bounds.
* Semantic diagnostics for incompatible aggregate elements, keys, mutations,
  indexes, function arity, numeric operands, and branch layouts before IR
  lowering.
* Front-loaded shared-form shape diagnostics for malformed control-flow,
  conversion, string, collection, call, binding, and loop forms before backend
  emission.
* OpenAI model recommendations across the bug and improvement backlogs,
  matching the existing Claude/Gemini routing columns with GPT-5.6 Luna,
  Terra, and Sol tiers.
* Bounded, opt-in auto-patching for the observer. It validates a model-proposed
  `.howl` replacement in an isolated project copy, installs it atomically only
  after the configured tests pass, and then runs an explicit restart command.
* Isolated unit and subprocess integration coverage for path confinement,
  malformed model responses, test failure, atomic installation, and restart
  ordering.
* Bytecode-native in-memory store opcodes for structured records:
  `STORE_OPEN`, `STORE_PUT`, `STORE_GET`, and `STORE_DELETE`.
* Compiler and VM regression coverage for store lowering, named store
  attachment, idempotent deletion, missing-record behavior, record-copy
  isolation, and bytecode fixture execution.
* Compile-time optimization signatures through `optimize_signature` and
  deterministic `-optimization-plan` JSON, with checker metadata, transparent
  backend execution, documentation, and regression coverage.
* One authoritative, backend-independent construct-support registry in
  `internal/construct`, classifying every HowlFrame construct as `Supported`,
  `CompileTimeOnly`, or `Unsupported` for the standalone bytecode target, with
  parent-scoped sub-forms and a context-aware AST scan. A drift test parses
  `compileNode`'s own `switch head` so the registry and the compiler cannot
  diverge silently.

### Changed
* Dict values may now mix types. Dicts are the language's record literal, and the
  VM and native store both carry `map[string]any`, so a record combining strings,
  ints, lists, and nested dicts already executed correctly; only the analyzer
  rejected it. Heterogeneous dict literals and `map_set` writes now widen the
  element type to `any` through the existing `join` helper instead of reporting
  `dict value N has type X, want Y`. Key checks, target-kind checks, and list
  element homogeneity are unchanged.

* Documented the existing standalone HTTP JSON request composition
  (`parse_json ... req.body` with `try_let`), its bounded scope, and the
  JavaScript `web_app` compatibility-generation command. Added focused
  bytecode and JavaScript-generator conformance coverage for the external
  consumer request/response flow.
* Completed the HowlFrame identity cutover across the repository, compiler,
  canonical Go module, `.howl` source corpus, `.hfbc` bytecode examples, HFIR
  packages and diagnostics, machine-readable namespaces, documentation,
  website metadata and artwork, benchmarks, tools, and reference application.
* Output directories are now created consistently, and Go, JavaScript, and
  legacy WAT generation accept `-o` before or after the input path.
* Long valid `let` chains now use shared iterative traversal across AST
  preprocessing, semantic checking, and Go/JavaScript emission, with a
  2,000-binding regression covering transpilation and generated Go builds.
* Synchronized the HowlFrame language write-cost benchmark fixtures with the
  published 2026-07-30 token counts for Tasks B and C.
* Integer and string list/dictionary expressions now initialize Wasm linear
  memory with typed stores, dynamic dictionary keys compare interned string
  pointers, integer dictionary reads use typed loads, multiple aggregate
  literals receive independent aligned memory regions, and aggregate tables and
  memory page counts scale with their payloads.
* Refreshed the README and GitHub Pages content to reflect the current
  semantic checker, typed backend metadata, direct binary bytecode, and expanded
  WebAssembly Text backend scope.
* Removed stale transpiler branding and Go-only positioning from the README,
  GitHub Pages landing page, and current public docs.
* Re-scored the remaining self-healing backlog after auto-patching shipped,
  corrected the WebAssembly prototype's WAT scope, and restored the missing
  Ephemeral Neural Circuits detail section.
* `-compile-bc` now accepts `-o <file>` after the input path for exact bytecode
  output files while keeping the existing output-directory behavior.
* `-compile-bc` now fails closed on any construct the bytecode compiler cannot
  lower, reporting a `HFIR_TARGET_INFEASIBLE` diagnostic that names the
  construct, its source location, and the backlog item that owns the gap, and
  writing no artifact. Type annotations and forms consumed by earlier passes
  are classified separately and keep compiling unchanged.

### Fixed
* `for` over an expression no longer silently miscompiles in the JavaScript and
  Go backends. Both read the iterable's raw node value, which is empty for
  anything but a bound symbol, so `(for m (map_get d "missions") ...)` emitted
  `for (let m of )` and `for _, m := range {` - invalid output produced with no
  diagnostic, in a toolchain whose contract is to fail closed.
* `on_event` now terminates its statement. Automatic semicolon insertion does
  not apply before `(`, so any following top-level statement was parsed as a
  call of the `addEventListener` result.
* A `web_app`'s top-level statements are wrapped in an async IIFE. They routinely
  contain awaited calls, and a classic `<script>` has no top-level await, so
  every generated interface failed to parse in the browser. Function
  declarations remain at top level so inline handlers can still reach them as
  globals.
* Route handlers fail closed. A panic inside an `http_server` route handler wrote
  nothing to the `ResponseWriter`, so Go emitted `200` with an empty body and a
  denied capability was indistinguishable from a completed request. Handlers that
  fail before responding now return `500` with the structured `VMError` JSON
  (preserving codes such as `CAPABILITY_DENIED` and `LIMIT_EXCEEDED`), and the
  failure is reported on the VM's error stream rather than process stdout. A
  handler that already committed a response is left untouched.
* `SPAWN_AGENT` and `TASK` opcodes in the standalone bytecode VM now report a
  structured `UNSUPPORTED_CONSTRUCT` runtime error naming the opcode rather than
  panicking with `VM_INTERNAL` as an unknown opcode. Both opcodes are emitted
  by the compiler and registered in the opcode table, but are unimplemented in the
  runtime; reporting them with an explicit unsupported-construct error distinguishes
  known unimplemented constructs from internal VM crashes. For `SPAWN_AGENT`,
  capability checks for `capability.Process` precede the unsupported construct error. Covered by
  `TestVMUnsupportedConstructs` in `internal/vm/vm_negative_test.go` and
  `TestBytecodeRunSwarmFixtureReportsUnsupportedConstruct`, `TestBytecodeRunSwarmFixtureWithoutCapabilities`,
  `TestBytecodeRunWithMalformedOpcode`, and `TestBytecodeRunWithTimeout` in `howlframe_test.go`. (bugs.md #54)
* Bytecode compiler properly extracts typed parameter names in `defun` and
  `lazy_synthesize`, and skips return-type annotations in `defun` body compilation.
  Previously, parameters written with types like `((a int) (b int))` had their names
  collapsed to empty strings in the bytecode function definition, and return-type
  symbols were erroneously emitted as `LOAD_VAR <type>` instructions in the function
  body, causing runtime `undefined variable: <type>` panics. In addition,
  `tools/difftest/main.go` now dynamically verifies that exemptions for `"unsupported in run"`
  are still earned, failing if an exempt fixture passes all backends; the stale exemption
  for `test_improvement_46.howl` in `tools/difftest/manifest.json` was removed and now
  passes cross-backend differential testing. Covered by unit regressions in
  `internal/bytecode/bytecode_test.go` (`TestCompileDefunWithTypedParametersAndReturnType`,
  `TestCompileDefunAdversarialEdgeCases`), `internal/vm/regression_test.go`
  (`TestDefunWithTypedParametersAndReturnType`), and `difftest`. (bugs.md #51)
* Flags written after the positional input file are honoured instead of
  discarded. Go's `flag` package stops parsing at the first non-flag argument,
  so `howlframe prog.howl -mask-plan` set no mode flag at all, fell through to
  the default Go backend, exited 0 with no output and wrote an unrequested
  `server.go` into the working directory. `main` now resumes parsing over the
  trailing tokens with a second `FlagSet` that re-registers every flag sharing
  the identical `flag.Value`, so every flag works on either side of the input
  and anything unconsumable is reported rather than ignored. This replaces
  `outputFlagAfterInput`, which rescanned raw argv for the `-o` and `-o=`
  spellings alone, so `-o` is no longer the only flag with after-input
  handling, while keeping its position-dependent meaning (output directory
  before the input, exact artifact file after it). The `-run-bc` and `-run`
  argv contract is untouched: those modes skip the resumed parse entirely and
  still hand every token after the input to the target program verbatim, and
  writing either of them after the input is now rejected with a diagnostic
  instead of silently reinterpreting the program's arguments. The `build`
  subcommand resumes its own parse the same way, so it too accepts any of its
  flags after the source file and now rejects a stray positional or an
  undefined flag there rather than ignoring it. `-h` and `-help` after the
  input print the same usage and exit 0 just as they do before it, instead of
  becoming a `flag: help requested` diagnostic. Covered by
  `TestCLIFlagPositionRelativeToInput`. (bugs.md #52)
* `route` handlers are validated before they are indexed. `checkGoAppHandler`'s
  `case "route"` read `Children[2].Children[1]` and `Children[2].Children[2]`
  without checking that the handler was a well-formed `(lambda (req) body)`, so
  `(http_server 8080 (route "/" (lambda (req))))` crashed the compiler with
  `index out of range [2] with length 2` rather than reporting an error. The
  guard the `middleware` case already applied to its own lambda is now applied
  to `route` as well, and the middleware nested-route loop delegates to
  `checkGoAppHandler` instead of duplicating the validation, so nested routes
  are checked identically without adding duplication debt. A parameter that is
  not a symbol is now rejected too. Covered by `TestCheckRouteHandlerValidation`
  with 12 subtests across both forms. (bugs.md #50)

* `docs/archive/old_howlframe.go` is no longer counted as production Go. The
  file is a 4177-line archive of the pre-cutover monolithic implementation
  whose first line is `//go:build ignore`, so Go itself never compiled it, but
  `.slop`'s `go_production` scope (`scan_path: .`, `pattern: "**/*.go"`)
  matched it and counted 4177 lines of dead history as production source. It
  was one side of roughly 81% of the repository's duplicated windows, which
  held `go_production` at 291 active clones against a committed ceiling of
  290 and failed the required repository-hygiene gate for every governed task
  regardless of what that task changed. Renamed to
  `docs/archive/old_howlframe.go.txt`, which states a fact that was already
  true rather than adding an ignore rule or tombstone to the hygiene policy.
  `go_production` fell from 291 active clones across 41 sources to 84 across
  40, and the committed ceiling was ratcheted down from 290 to 84 to match.
  `go_tests` is unaffected at 128. Build, vet, test and `gofmt` output are
  unchanged, since the file was never in the build. (bugs.md #49)

* Fixed a severe issue where `str_split`, `str_join`, `regex_match`, `append`, `map_set`, `map_delete`, `map_get`, `list_get`, and `list_len` bytecode VM instructions relied on unsafe Go type assertions, which caused immediate uncatchable internal panics (`interface {} is ...`) instead of deterministic script errors. The VM now performs safe checks and produces structured `TYPE_ERROR` runtime errors when given unexpected types, enabling correct `try_let` interception.

* Bytecode `try_let` now resumes after its complete embedded instruction region
  instead of re-executing the final branch instruction and corrupting an
  enclosing `for` loop's operand stack.
* `-compile-bc` silently dropped every construct the bytecode compiler did not
  recognize. `compileNode`'s head switch had no `default` case, so unknown
  heads compiled to zero instructions and the resulting program ran to exit 0
  while skipping them entirely: `tests/test_advanced_control.howl` produced no
  output at all instead of `zero`/`one`/`other`. Unsupported constructs are now
  rejected before an artifact is written, and `compileNode` has a fail-closed
  backstop for callers that bypass the gate.
* `type_hints` and `type_param` had no `compileNode` case and were only handled
  by accident, through that same silent fall-through. They now have explicit
  no-op cases alongside `type_hint`.
