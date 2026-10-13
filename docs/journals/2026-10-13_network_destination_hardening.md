# Network destination hardening (#171, #172)

Date: 2026-10-13. Worktree: hf-blockers-net, based on main 1bda2f15.
No commit or push.

## Consumer findings and changes

The three network implementations needed the same destination policy.
Hostname grants now block loopback, RFC1918, ULA, NAT64, link-local,
unspecified and multicast results. Literal IP grants and the exact name
localhost deliberately permit private access; multicast is always refused.
Deterministic lookup overrides resolve private.test to 127.0.0.1 in the
socket tests: the target receives zero requests. Literal and localhost
opt-outs each reach the target. Boundary fixtures cover all requested
ranges, IPv4-mapped IPv6, and public addresses beside private ranges.

Go checks every returned address before dialing, then checks the selected
address again with Dialer.Control. Node checks DNS results and pins its
native HTTP connection's lookup to a checked result. A rebinding fixture
returns a public address first and loopback on any second lookup; the
request connects using the first result and performs exactly one lookup.
Browser execution keeps host, literal multicast and redirect checks but
cannot inspect DNS or pin connections. No DOM was added.

An 11-redirect chain stops after the initial request plus ten followed
redirects. Proxy tests count TCP connections, not just HTTP handler calls.
HTTP and HTTPS traps see zero connections in core and generated Go.
Node 24.21.0, selected with `npx --yes --package=node@24`, also passes with
HTTP_PROXY, HTTPS_PROXY and NODE_USE_ENV_PROXY=1 set at process startup.
Mutating back to native fetch produces a proxy connection and fails.
The startup trap closes connections immediately so a mutated CONNECT
request cannot stall the test. An earlier trap that served an ordinary
HTTP response stalled that mutation; it was replaced, the task's leftover
test processes were stopped, and the mutation was rerun successfully.
Node 20.19.2 is the default environment.

HTTPS downgrades are refused before contacting the HTTP destination even
when both ports are granted. Port-changing redirects strip Authorization
and Cookie. A three-hop Go consumer revealed that net/http can copy the
initial request's headers again on a later same-origin hop; stripping now
continues for the rest of the chain after an origin change. Generated Go
also compares effective origins, preserving credentials across a change
in hostname case or spelling of the default port. Node's native response
reader retains gzip/deflate/Brotli decoding and UTF-8 BOM removal from the
previous fetch behavior.

Allowed bytecode fetch receipt effects record
`private_destination_opt_out` for literal or localhost authority, including
redirect effects. This records permission, not successful network access.
Denied effects and ordinary hostname grants do not record that opt-out.

Process grants refuse interpreter code-loading variables case-insensitively:
PYTHONPATH, PYTHONSTARTUP, NODE_OPTIONS, NODE_PATH, BASH_ENV, ENV, PERL5LIB,
PERL5OPT, RUBYOPT and RUBYLIB, alongside existing LD_*/DYLD_* refusals.
Generated Go/JS child-environment tests also supply raw grants directly,
so bypassing grant parsing does not pass the variables through.

WHATWG previously canonicalized JS grants such as 127.1 to 127.0.0.1.
JS now refuses ambiguous numeric grants and URL authorities; strict URL
syntax prevents alternate slash/backslash forms from bypassing the check.
Go trailing-dot hostname diagnostics now identify the trailing dot rather
than reporting an ambiguous numeric host.

## Breaking changes and app documentation

The Unreleased Breaking entry and network/process authority references
describe the tightened name-destination policy, multicast/downgrade
refusals, stripped credentials, receipt field, environment exclusions and
JS numeric-grant correction. Private access requires explicit literal or
localhost authority. Granting a route localhost:<its-server-port> permits
its /admin and all other paths on that origin.

apps/webhook_relay/README.md now makes the runtime UPSTREAM_URL selection
and fixed 127.0.0.1:8099 route authority explicit. Changing the variable
cannot expand the grant; changing upstream requires updating both route
and runner grants. The app source is unchanged, preserving token fixtures.

## Negative tests and mutation evidence

Each mutation below temporarily changed only the named production guard;
the selected test failed behaviorally (not because compilation failed).
The original source was restored in a finally block after every run.
All 34 mutations were detected.

| Guard / compatibility behavior | Negative test | Result |
|---|---|---|
| core redirect cap | `TestFetchSecurity/RedirectCap11` | failed with mutation; restored |
| core proxy isolation | `TestFetchSecurity/ProxyEnvIgnored` | failed with mutation; restored |
| core HTTPS downgrade | `TestFetchSecurity/HTTPSDowngrade` | failed with mutation; restored |
| core multicast literals | `TestFetchSecurity/MulticastDial` | failed with mutation; restored |
| core private-name guard | `TestFetchSecurity/NamePrivateDial` | failed with mutation; restored |
| core private-range guard | `TestFetchPrivateRanges` | failed with mutation; restored |
| core credential stripping | `TestFetchSecurity/CredentialsPortChange` | failed with mutation; restored |
| core code-loading environment | `TestEnvGrantRefusesCodeLoading` | failed with mutation; restored |
| core trailing-dot diagnostic | `TestFetchTrailingDotDiagnostic` | failed with mutation; restored |
| receipt opt-out | `TestReceiptFetchPrivateOptOut` | failed with mutation; restored |
| gogen redirect cap | `TestGoFetchSecurity → TestSecurity/RedirectCap11` | failed with mutation; restored |
| gogen proxy isolation | `TestGoFetchSecurity → TestSecurity/ProxyEnvIgnored` | failed with mutation; restored |
| gogen HTTPS downgrade | `TestGoFetchSecurity → TestSecurity/HTTPSDowngrade` | failed with mutation; restored |
| gogen multicast literals | `TestGoFetchSecurity → TestSecurity/MulticastDial` | failed with mutation; restored |
| gogen private-name guard | `TestGoFetchSecurity → TestSecurity/NamePrivateDial` | failed with mutation; restored |
| gogen private-range guard | `TestGoFetchSecurity → TestSecurity/PrivateRangesDial` | failed with mutation; restored |
| gogen credential stripping | `TestGoFetchSecurity → TestSecurity/CredentialsPortChange` | failed with mutation; restored |
| gogen code-loading environment | `TestGoFetchSecurity → TestCodeLoadingEnv` | failed with mutation; restored |
| JS redirect cap | `TestJSRedirectCap11` | failed with mutation; restored |
| JS proxy isolation | `TestJSProxyEnvIgnored` | failed with mutation; restored |
| JS HTTPS downgrade | `TestJSHTTPSDowngrade` | failed with mutation; restored |
| JS multicast guard | `TestJSMulticastDial` | failed with mutation; restored |
| JS private-name guard | `TestJSNamePrivateDial` | failed with mutation; restored |
| JS private-range guard | `TestJSPrivateRanges` | failed with mutation; restored |
| JS credential stripping | `TestJSCredentialsPortChange` | failed with mutation; restored |
| JS numeric grants | `TestJSAmbiguousNumericGrant` | failed with mutation; restored |
| JS numeric URLs | `TestJSAmbiguousNumericGrant` | failed with mutation; restored |
| JS code-loading environment | `TestJSCodeLoadingEnv` | failed with mutation; restored |
| JS DNS pin | `TestJSPinsVettedLookup` | failed with mutation; restored |
| JS HTTPS downgrade real TLS | `TestJSHTTPSDowngrade` | failed with mutation; restored |
| Node 24 startup proxy isolation | `TestJSProxyEnvAtStartup (Node 24)` | failed with mutation; restored |
| JS strict numeric URL syntax | `TestJSAmbiguousNumericGrant` | failed with mutation; restored |
| JS response decoding | `TestJSFetchCompressedResponse` | failed with mutation; restored |
| gogen effective-origin compatibility | `TestGoFetchSecurity → TestSecurity/CredentialsSameOrigin` | failed with mutation; restored |

Core socket assertions run under `TestFetchSecurity`. Generated Go emits
the authoritative fetch/process helper sources into a temporary consumer
package under internal/ and runs the same assertions under
`TestGoFetchSecurity` → `TestSecurity`; temporary artifacts are removed.
Mutation filtering uses HF_NET_MUTATION_FILTER for that inner consumer.
The test fixture formerly treating private IPs as public was replaced by
public IPv4/IPv6 fixtures and explicit private-range refusal tests.
The JS code-shape assertion now names the governed request helper; network
behavior is asserted separately. No freeze test or sha256 pin was changed.

Mutation transcripts were kept outside the repository under
`/tmp/hf-net-work/mutation-*.log` during this session. The table above is
the durable record of the selected tests and restored guards.

## Validation

* Formatting and git diff whitespace checks passed.
* go build ./... and go vet ./... passed.
* go test -json ./... passed with zero failed test/package events.
* HFIR parity: 268 leaf cases passed, 0/268 mismatches.
* TestModelAdapterCoversArityTable and TestProdFlipCriteriaLock passed.
* TestHFIRParseJSONWriteFileStaysFailClosed, the governed-profile tests and
  TestIssue165GoRouteFetchFailsClosed passed; Go/JS UNSUPPORTED_ON_TARGET
  behavior remains intact.
* Existing frozen-result/pin suites passed. No sha256 pin, freeze test,
  production HFIR selection, #90 Partial status or tip-lock was changed.
* Node 24.21.0 passed private DNS, pinned lookup, downgrade, port-changing
  credential, proxy-at-startup and compressed-response consumers after all
  mutations were restored.
* Final go test ./...: pending the last run after this journal update.

## Files changed

* `apps/webhook_relay/README.md`
* `change_log.md`
* `docs/journals/2026-10-13_network_destination_hardening.md`
* `docs/reference/network_authority.md`
* `docs/reference/process_authority.md`
* `internal/backend/gogen/gogen.go`
* `internal/backend/gogen/network_security_test.go`
* `internal/backend/javascript/javascript.go`
* `internal/backend/javascript/javascript_test.go`
* `internal/backend/javascript/network_security_test.go`
* `internal/capability/grants.go`
* `internal/capability/network.go`
* `internal/capability/network_security_test.go`
* `internal/capability/network_test.go`
* `internal/capability/process.go`
* `internal/nettest/security.go`
* `internal/vm/receipt.go`
* `internal/vm/receipt_test.go`
