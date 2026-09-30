# Fetch body bytecode design spike (#90)

## Why this slice

Dev Lead Motoko flagged a design alternate for `(fetch url method body)` on the two bytecode compilers after the Assurance tip-lock. Owner William Elias kicked this session close-out as that note. The work is the design, not a compiler change.

## Starting point

The Assurance tip-lock is `4d74dbcf9654caa05e0b1d9212b15bc5398359e3` on `main` (PR #71). Assurance (Lain): Overall PASS, Promote DEFERRED. The verdict is `docs/journals/2026-09-30_assurance_tip_lock_4d74dbcf.md`. The measurement record is `docs/journals/2026-09-30_lowered_hfir_prod_flip_tip_lock.md`. The checklist is `docs/reference/lowered_hfir_prod_flip_criteria.md`.

This journal is written on `3027908d320c340415a15f918a70a46f8eb143e8` (PR #72), the commit that records that lock. The lock SHA is unchanged.

## Decision

docs-only / no build.

The design note is `docs/reference/fetch_body_bytecode_design.md`. It records the status quo (both compilers drop the body, `OpFetch` pops 2 with an empty operand list and capability `network`, and the interpreter, Go, and JavaScript can still send a body), why that drop is the accepted shared limit, and three later shapes: a presence flag on `FETCH`, a second opcode, and an unconditional pop of 3 with an absent sentinel. A non-binding recommendation prefers the presence flag. Motoko reviews it. The Owner kicks any implementation.

#90 stays Partial. Promote stays DEFERRED. Production `-compile-bc` stays `bytecode.CompileToBytecode`.

## Owner follow-up

The same docs-only slice updates `README.md` and the GitHub Pages page `docs/index.html` so they name experimental `-compile-hfir-bc`, the mediated host effects, the deferred flip, tip-lock `4d74dbcf` (Overall PASS, Promote DEFERRED), and the design note plus the prod-flip criteria. The Pages source is `docs/index.html`. No site redesign.

## What this does not do

No opcode, no capability, no edit to either bytecode compiler, the VM, the Go or JavaScript backends, conformance JSON, or `TestProdFlipCriteriaLock`. No production flip. The tip-lock SHA stays `4d74dbcf9654caa05e0b1d9212b15bc5398359e3`. A later body implementation expires that lock and still does not justify a flip.
