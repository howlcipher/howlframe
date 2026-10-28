# HowlFrame release security (v0.2.0 release candidate)

HowlFrame is a capability-governed runtime. It is **not** an OS sandbox and is **not** safe for arbitrary hostile
code. This report separates what the candidate guarantees (verified here), what it does not, and how each open
limitation is classified for an explicitly experimental release. All probes used the RC binary on Linux x86-64 with
harmless files under a temporary directory; the #216 probe used a marked `sleep` and removed it afterwards.

## Guarantees verified on the candidate

| Guarantee | Probe | Result |
|---|---|---|
| Effects are denied unless granted | `write_file`, `exec`, `fetch` with no grant | `CAPABILITY_DENIED`, exit 1, no file written |
| Path-scoped filesystem grants | `filesystem:read=<root>`: read outside the root; read through a symlink in the root pointing outside; read inside | Denied; denied; allowed |
| `exec` needs an exact program grant | Bare `process`; `process:exec=` on a symlinked program path; `process:exec=<canonical path> *` | Denied; denied (`traverses a symlink`); runs |
| `process:env=` refuses steering variables | `process:env=LD_PRELOAD` | Refused before running |
| Outbound fetch needs a host grant | Coarse `network` with `fetch` | Denied (`fetch host is not in the allow-list`) |
| Budgets | Infinite loop (default and `--max-instructions 5000`); `--deadline 300ms`; string doubling with `--max-memory-bytes 1MiB`; unbounded recursion | `LIMIT_EXCEEDED`: instruction limit, wall-clock deadline, memory limit, call depth (1000) |
| Invalid budgets refuse to run | `--max-instructions 0`, `--max-memory-bytes 0`, `-run-bc -max-memory-bytes -5` | Refused (exit status inconsistent, see below) |
| Runaway descendants of `exec` are contained (Linux) | `exec` of a shell that starts a `setsid` sleeper, then returns | 0 survivors after the call |
| Artifacts are verified before execution | 11 malformed variants; a `+Inf` constant | Refused on every load path |
| EvidenceDesk denials (network, environment, database, `filesystem:read`, `filesystem:write`, static read, small instruction budget) | `verify_workflows.py` with the staged binary | All PASS |
| Numeric policy values are finite | `HOWLFRAME_NUMERIC_CORRECTNESS.md` | NaN/Inf cannot reach a comparison on the VM or interpreter |

## Open limitations and classification

| Item | Evidence | Classification | Reason |
|---|---|---|---|
| **#216** Hostile code that kills its supervisor leaves detached descendants | Reproduced on the RC: the exec'd shell starts a `setsid` sleeper and `kill -9 $PPID`; `exec` returns `signal: killed` and **1** sleeper survives (0 when the supervisor is not killed) | **Acceptable for an experimental release, documented** | The guarantee claimed (`SECURITY.md`, remediation doc) is runaway containment, which holds. Hostile containment is explicitly not claimed. Exploiting it needs a grant to exec a program able to kill processes, which the operator chooses. |
| **#217** macOS/BSD: session-escaped descendants not contained; Windows: direct child only | Code inspection (`exec_contain_unix.go`, other platforms); no macOS/Windows host to execute on | **Acceptable, documented restriction**: run `process:exec` workloads on Linux | Documented in `SECURITY.md`, installation guide and release notes. The binaries themselves are unexecuted (Owner decision below). |
| **#182** `process:env=` is a deny-list | Owner deferred the allow-list on 2026-10-23 (would break TaskFlow's frozen `T-R11-env-isolation`); step 1 refusals merged (#219, #221) | **Deferred** | No claim of completeness is made; documented as "grant only the names a program needs". |
| Filesystem: coarse `filesystem` unrestricted; hard links in a root; parent path trusted | `docs/remediation/HOWLFRAME_SECURITY_REMEDIATION.md` residual limits | Acceptable, documented | Path-scoped grants verified above. |
| Budgets are VM accounting | `--max-memory-bytes` counts cumulative allocation; no RSS or child-process CPU/memory limit | Acceptable, documented | Now stated in `SECURITY.md`. |
| Invalid budget exit status is 1 or 2 depending on flag and path; `interop --json` documents 2 | `run --max-instructions 0` and every `-run-bc` case exit 1; `run --max-memory-bytes 0` exits 2 | Acceptable, documented in release notes | Fails closed either way; a contract-accuracy issue for tools, not a security one. Follow-up recommended. |
| Generated Go/JS are separate code with their own checks; generated Go numeric semantics differ | NUMERIC_CONTRACT section 13; remediation doc | Acceptable, documented | The release positions the standalone VM as the governed path. |
| Artifacts and binaries unsigned; HFBC checksum is not authentication | Envelope format | Acceptable, documented | `SHA256SUMS` covers download integrity only. |

No issue was classified release-blocking on the code. One documentation defect was:

## Unsupported claims found and corrected

The public project page (`docs/index.html`, served at howlcipher.github.io/howlframe) described a "sandboxed
bytecode VM", tagged every target including generated Go and JavaScript `[SANDBOXED]`, promised "deterministic
architectural guarantees ... isolated bytecode execution", said grants prevent side effects "from escaping the VM
sandbox", and said grants are "cryptographically ... attached". These contradict `SECURITY.md` and the product
directive (section 3.3) and would market HowlFrame as an isolation boundary. **Release-blocking until corrected;
corrected on the RC branch** (`[GOVERNED]`, "capability-governed", an explicit "not an OS sandbox" statement, grants
"explicitly granted by the runner"). The SEO checks (`scripts/test_seo.py`) still pass. README, `SECURITY.md` and the
v0.1.0 release notes already disclaimed sandboxing.

## Claims the release must not make

OS or container isolation; safety for hostile or arbitrary code; containment of descendants on macOS, BSD or Windows,
or against code that targets its supervisor; complete environment filtering; tamper-proof artifacts; formal
verification; parity of generated Go/JavaScript with the VM; execution on macOS or Windows having been tested.
