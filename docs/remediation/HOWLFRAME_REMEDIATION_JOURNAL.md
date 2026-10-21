# HowlFrame Remediation Journal

Branch `fix/post-audit-stabilization`, base `5d2983b2c2311daf9b7494311ad719ff5dc51c09`
(== origin/main at start; 0 commits landed since the audit; CI on base: success).
Toolchain: go1.26.0 local, Python 3.14.4. Audit inputs: /run/media/system/tallgeese/dev/howlframe-independent-audit-20261021/ (unchanged).

## 2026-10-21 Phase 0
- Verified baseline == audited commit; worktree created at dev/worktrees/howlframe-post-audit-remediation.
- Design decision HF-01: use `os.Root` (Go 1.25: needs Root.Rename/Chmod/MkdirAll). Rejected: hand-written
  openat walkers per platform (no Windows/macOS parity, release builds ship both), and staying on Go 1.21
  (no race-free primitive). `go 1.25` in go.mod; `GOTOOLCHAIN=auto` fetches it for 1.21+ users.
- Design decision HF-02: Linux per-exec supervisor acting as a subreaper (cgroup delegation and user namespaces
  are not reliably available; probed on this host: cgroup not writable). Other OSes: process group only, documented.

## HF-01 filesystem containment
- Red first: `internal/vm/filesystem_race_test.go` (read, write, mkdir, file store while a symlink inside the root flips
  between an in-root dir and an outside dir). Baseline: all four FAIL (read escaped 2063/4000, write/mkdir/store landed
  outside). Log: evidence/hf01_baseline_red.log.
- Fix: `internal/capability/scopedfs.go` (`OpenScope`, `Scope`, `FileSystem`), `matchRoot` split from `allows` (decision logic
  unchanged), VM/interpreter read/write/mkdir and file-backed store open/persist routed through the root handle.
- Behavior preserved on purpose: missing granted root => ordinary I/O error (M2 R1.3 exit 66); mkdir under a missing granted
  root still creates it; coarse `filesystem` alias stays unrestricted.
- Behavior change (documented, fail closed): an absolute symlink inside the root is refused even if it points back in;
  relative in-root links work. Test: TestAbsoluteInRootSymlinkFailsClosed.
- Side effect of go 1.25: new vet check flagged a real latent bug (`"undefined var: "+bodyVar` as a format string); fixed.
- Test setup change (not assertion): store_timing tests construct a bare BCVM; they now carry a coarse filesystem grant
  because persistence is authorized through the VM's grants.
- Green: internal/vm and internal/capability pass; race tests 15x stable and clean under -race.
