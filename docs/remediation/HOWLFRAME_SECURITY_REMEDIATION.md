# HowlFrame security remediation: guarantees and limits

Branch `fix/post-audit-stabilization`, base `5d2983b2c2311daf9b7494311ad719ff5dc51c09`.
Findings: HF-01 (filesystem scope), HF-02 / #181 (exec descendants), HF-04 (slow reader).
**None of this is OS sandboxing.** HowlFrame remains a capability-governed runtime. These changes make two of its
logical guarantees hold under concurrency; they do not isolate a hostile process from the host.

## 1. Filesystem scope (HF-01)

### Guarantee
For a **scoped** grant (`filesystem:read=<root>`, `filesystem:write=<root>`), every governed file operation
(`read_file`, `write_file`, `mkdir`, and file-backed store open, write and rename) is performed through an `os.Root`
opened on the granted directory. The operating system, not a path check made earlier, refuses any step that would leave
the root, including following a symlink out of it. A path component that another process replaces between the
authorization check and the open can no longer redirect the operation.

How: `internal/capability/scopedfs.go` (`OpenScope`, `Scope`, `FileSystem`). The existing lexical and
symlink-resolved containment decision (`matchRoot`, formerly the body of `allows`) is unchanged, so denial codes,
receipts and ordering are unchanged. `OpenScope` then pins the I/O to the root that decision selected. The VM and
interpreter call it for reads, writes and mkdir; the store layer uses it for the initial read and for the whole atomic
replace (temporary file, sync, chmod, rename, directory sync), so every step stays under the same root handle.

### Grants naming a single file
A grant may name one file (the EvidenceDesk projector's launcher does). The root handle is then opened on the file's
directory, which would let the OS follow a symlink at the file's name to a *sibling*. So single-file scopes are
"pinned": the granted name is inspected first and a symlink there is refused; the file then opened must be the same file
that was inspected (`os.SameFile`); a missing file is created exclusively, so a link planted at the name cannot be
followed; and truncation happens only after that check, so a swapped-in link cannot cost another file its contents.
(This was found by the independent review of this remediation: the first version reached siblings through a relative
symlink. It is covered by `TestFileRootGrantRefusesRelativeSymlinkToSibling` and by the file-grant race test, which
flips the granted name through an outside link, a sibling link and a regular file.)

### Behavior changes (all fail closed)
| Case | Before | Now |
|---|---|---|
| Symlink inside the root with an **absolute** target that points back inside the root | followed | **denied** (`CAPABILITY_DENIED`); relative in-root links still work |
| A granted **file** whose name is a symlink (to anywhere, including a sibling) | followed | **denied** |
| Granted root does not exist: read | ordinary I/O error (not found) | unchanged (ordinary I/O error) |
| `mkdir` under a granted root that does not exist | created the root | unchanged |
| Coarse `filesystem` alias | unrestricted | unchanged (unrestricted by design) |

### Platforms and toolchain
`os.Root` is portable (Linux, macOS, Windows, BSDs), so the guarantee is not Linux-only. It needs Go 1.25
(`Root.Rename`, `Root.Chmod`, `Root.MkdirAll`), so `go.mod` now declares `go 1.25` (was 1.21). Evaluated alternatives:
(a) hand-written `openat` walkers with per-platform build tags, rejected because release builds ship Windows and macOS
and the code would be larger and easier to get wrong; (b) staying on 1.21, which has no race-free primitive. Users on
Go 1.21 or newer get the right toolchain automatically (`GOTOOLCHAIN=auto`). The CI `build` job, the race job and
the acceptance job use 1.25.x. The `latest-go` job uses the newest stable toolchain. The release workflow was updated to 1.25.

### Residual limits
* The grant root itself is named by path. A local actor who can replace the **root directory's own parent components**
  controls what the grant means. This is outside the threat model for a grant named by the operator.
* The coarse `filesystem` alias remains unrestricted. Hard links inside a root are ordinary files and can alias data
  elsewhere on the same filesystem; `os.Root` does not prevent that.
* Generated Go and JavaScript targets have their own file-access code and were **not** changed (directive: the
  standalone VM is the priority; backend parity is not a gate). They should not be assumed to have this guarantee.
* No claim is made about hostile code that finds a bug in the Go standard library or kernel.

### Tests (all confined to `t.TempDir()`, harmless marker files)
`internal/vm/filesystem_race_test.go`: read, write, mkdir and file-store while a symlink inside the root flips between an
in-root directory and an outside one; a file grant under the same flip; absolute-link fail-closed; harness sanity.
`internal/capability/scopedfs_test.go`: pins the `os.Root` escape message against the running toolchain, missing-root
semantics, mkdir creating a missing root, single-file grants, file symlink escape.
Baseline (unfixed) result: all four race tests failed (2,063 of 4,000 reads returned outside content; the write, mkdir and
store writes landed outside). After: zero, stable over 15 repeated runs and under `-race`.
`go test -race` also covers the new code. Existing tests (`TestFilesystemScopesVMAndInterpreter`,
`TestFileStoreFilesystemScopes`, grants tests) pass unchanged.

## 2. Exec descendant containment (HF-02, #181)

### Guarantee (Linux)
Every governed `exec` runs its program under a supervisor, a copy of the runner binary started with an internal flag
(`--howlframe-exec-supervisor-v1`, handled in package `vm`'s `init`). The supervisor marks itself a *child subreaper*
(`PR_SET_CHILD_SUBREAPER`), so any process orphaned anywhere beneath it, **including one that called `setsid`**, is
re-parented to the supervisor instead of escaping. **When an exec call ends, no process it started is still running.**
Specifically:
* Normal return: unchanged semantics. The call still waits for the program and for any process still holding its output.
  Only then is everything left in the tree killed (this removes background and detached processes that closed their output).
* Deadline, output-limit failure, or the runner dying (control pipe closed): the tree is killed at once.
* The sweep repeats until nothing remains, because killing a parent re-parents its children to the supervisor.
* Exit status, signal death, captured output, exec output limits, environment grants and start-failure reporting are unchanged.
* A program that closes its own stdout and stderr and keeps running is waited for and reports its real exit status
  (the supervisor reports program exit over a status pipe; output end-of-file is never taken for program exit, which
  the first version got wrong; `TestExecProgramThatClosesItsOutputEarlyKeepsRunning`).

### Other platforms
| Platform | Containment |
|---|---|
| Other Unix (macOS, BSD) | Own process group, group killed when the call ends. **A descendant that starts a new session is not contained.** |
| Windows and other non-Unix | Direct child only. **Descendants are not contained.** |

### Residual limits
* **A program written to defeat containment can.** The supervisor is the program's parent, and a same-user program
  can find and kill it. If it does, descendants that already detached into their own session are no longer re-parented
  to the supervisor and can survive (reproduced by the independent review). Containment therefore protects against
  runaway, forgotten or accidentally detached descendants (issue #181's case), **not** against hostile code that targets
  the supervisor. The process-group fallback still removes non-detached descendants in that case. Not implemented: making
  the runner itself a second-level subreaper, which would need a registry of its other children to avoid killing them.
* A process the runner's user cannot signal (for example one that changed privilege) is not covered.
* The supervisor is the same binary as the runner, so `exec` needs `os.Executable()` to be readable; if it cannot be
  located, exec fails with `exec containment unavailable` rather than running uncontained.
* The internal flag lets anyone who can run the binary run a command as themselves, which grants nothing they did not have.
* If the supervisor itself hangs for more than five seconds after the control pipe closes, its process group is killed
  as a last resort; children that left the group in that case could survive.
* cgroups and PID namespaces would give a stronger kernel-enforced guarantee. Probed here: cgroup v2 not delegated
  to the user, user namespaces dependent on distribution policy, so neither can be relied on. Not implemented.

### Tests
`internal/vm/exec_containment_linux_test.go`: normal exit, deadline, and output-limit stop, each with foreground,
background and session-escaped sleepers identified by unique `/proc` command-line markers, with a `t.Cleanup` that
kills any survivor so a failing run leaves nothing behind. Baseline: all three failed (seven surviving processes).
After: pass, stable over ten runs, no leftovers. Existing exec tests (exit codes, signal death, output caps, output
inherited by a background process) pass unchanged. TaskFlow's 32 frozen tests pass; the audit's own deadline probe
(failed in the audit) now passes.

## 3. Slow readers (HF-04)

With `(serialize_requests)` the handler's response is buffered in memory while the process-wide lock is held and
written to the client **after** the lock is released, under a write deadline (30 s). Handlers without the lock get the
same deadline on each write. One client that stops reading no longer stalls unrelated requests, and its connection is
released when the deadline passes. Limits: a buffered response must fit in memory (handlers already build it in
memory), and a client can still hold its own connection for up to the write timeout. Test:
`internal/vm/slow_reader_test.go` (in-process loopback server; unrelated `/health` must answer within 1.5 s, the
stalled connection must be cut short and released, a 2 MiB response is still intact). The audit's reproduction went from
a timeout beyond 60 s to a 0.0 s answer.

## 4. What this does not change
No OS sandbox, no live-memory bound (HF-14), no network isolation beyond the existing grants, and no change to the
environment-forwarding list (HF-11), file permissions (HF-13) or listener default (HF-20). See the follow-up register
in `HOWLFRAME_FINAL_HANDOFF.md`.
