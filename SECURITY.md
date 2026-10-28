# Security Policy

## Experimental Status

HowlFrame is an **experimental** AI-native language and capability-bounded execution runtime. 

It does **NOT** provide or guarantee:
- OS-level sandbox isolation
- Container isolation equivalent to hypervisors/VMs
- Formal verification of all programs
- Production-grade protection against novel VM escapes

HowlFrame is a capability-governed runtime, not a sandbox. The instruction limits and capability gating (including `process:exec=` allow-lists, `spawn_agent` grant attenuation and host-scoped `network:fetch=` grants, see `docs/reference/process_authority.md` and `docs/reference/network_authority.md`) are logic-level restrictions evaluated within the host process, not a strict isolation boundary provided by the operating system. Do not use HowlFrame to execute untrusted code in a sensitive production environment without proper external containerization.

## Known Limitations (v0.2.0)

These hold for the v0.2.0 release candidate. They are accepted for an experimental release, not fixed. The evidence is
in `docs/release-readiness/v0.2.0/HOWLFRAME_RELEASE_SECURITY.md`.

- **Exec containment is for runaway processes, not hostile code.** On Linux every governed `exec` runs under a
  subreaper supervisor that removes forgotten or session-escaped descendants when the call ends. A program that kills
  its own supervisor after detaching a descendant leaves that descendant running (#216).
- **Weaker containment off Linux.** On macOS and BSD the exec child gets its own process group, so a descendant that
  starts a new session is not contained; on Windows only the direct child is (#217). The macOS and Windows release
  binaries are built but have not been executed in the release evidence. Run programs that need `process:exec` on Linux.
- **`process:env=NAME` uses a deny-list, not an allow-list (#182).** It refuses known code-loading and
  execution-steering names; a name it does not know is passed through when granted. Grant only the names a program needs.
- **Filesystem grants.** `filesystem:read=<root>` and `filesystem:write=<root>` confine the VM and interpreter to the
  root, symlinks included. The coarse `filesystem` grant is unrestricted, hard links inside a root can alias other
  files, and the root's own parent path is trusted.
- **Resource budgets are VM accounting, not OS limits.** Instructions, call depth, wall-clock deadline, exec output,
  fetch body, request body and cumulative allocation (`--max-memory-bytes`) are enforced by the VM. They do not cap the
  runner's resident memory, or the CPU and memory of a program started with `exec` (the deadline and the output cap
  still stop it).
- **Generated Go and JavaScript are not the governed VM.** Their capability checks are separate code, and generated Go
  follows Go's numeric semantics (see `docs/reference/NUMERIC_CONTRACT.md` section 13).
- **Artifacts and binaries are not signed.** The HFBC checksum detects corruption, not tampering. Release archives are
  covered by `SHA256SUMS` only.

## Reporting a Vulnerability

If you discover a security issue or VM escape in the bytecode evaluator, please report it privately.

Do not open a public issue. Instead, please email the maintainers directly or use GitHub's private vulnerability reporting feature for this repository if enabled. We will attempt to acknowledge and address valid capability escapes.
