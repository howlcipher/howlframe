# Milestone 3 SPEC freeze

**Frozen 2026-10-20.** The seats voted 5/5 to freeze the spec and then build P-ECO, the receipt and grant-audit verifier.

## 1. What is frozen

| Field | Value |
|---|---|
| Spec freeze SHA | `e6fdd48a0c60564a2ad3833cf2b990dff7ad7c14` (main; squash-merge of #205) |
| Spec path | `milestone3/M3_ACCEPTANCE_SPEC.md` |
| Spec sha256 at the freeze SHA | `630b6ae4dde33e98a893ef209275866b78a154efddb77cd6209fef13fee679b4` |
| Held-out seal reference | `milestone3/heldout/SEALED.sha256`, sha256 at the freeze SHA: `2bdb70870f55cbe029ecaab1889955ec878931671ab834aa5ce0518a0d3acf3b` (sealed-v1; spec §7.3) |
| §7 text sha256 at the freeze SHA | `9d630db86e67bd1cdf7a8dfbc00ebd839ba21e22551dae403c12038ccc4bd798` (bytes from the `## 7.` heading up to the `## 8.` heading) |
| Owner acknowledgement | [`OWNER_ACK_2026-10-20.md`](OWNER_ACK_2026-10-20.md): items 1, 2 and 4 APPROVED, item 3 DECLINED (2026-10-20 15:23 ET, relayed by EngMgr) |

Verify with:
* `git show e6fdd48a:milestone3/M3_ACCEPTANCE_SPEC.md | sha256sum`
* `git show e6fdd48a:milestone3/heldout/SEALED.sha256 | sha256sum`

## 2. Spec freeze vs requirements freezes

This file is the **spec freeze**: it fixes the acceptance spec, its gates and rules, and the held-out commitments. It is
**not** a project requirements freeze. The spec's §1.1 and §1.1.1 set those out as per-project
`milestone3/<eco|web>/requirements/FREEZE.md` files. Each comes later, before that project's first implementation commit, and is
not valid without every field.

These fields are **not set here**. Each is set at that project's requirements freeze:

| §1.1.1 field | Set at |
|---|---|
| Freeze SHA (the requirements freeze commit) | P-ECO / P-WEB requirements freeze |
| Code pin | P-ECO / P-WEB requirements freeze |
| Compiler (`./howlframe` version and sha256) | P-ECO / P-WEB requirements freeze |
| Toolchain pins (Go, OS/kernel, CPU; Chrome and driver for P-WEB) | P-ECO / P-WEB requirements freeze |
| Harness and corpus pins (suites, E4 corpus, `E5_ALLOWLIST`, loader, scripts) | P-ECO / P-WEB requirements freeze |
| Seeds | P-ECO / P-WEB requirements freeze |
| W8 bounded N | P-WEB requirements freeze |
| Timing bounds | P-ECO / P-WEB requirements freeze |
| Alternate venue / Owner-named venue (consent, window) | none pre-registered (spec §5 rule 6); only by a dated Owner revision before the gate's first run |
| Owner ack | carried from [`OWNER_ACK_2026-10-20.md`](OWNER_ACK_2026-10-20.md) |

## 3. Binding conditions (set by the seats)

1. **This file merges to `main` before any P-ECO verifier commit.**
2. **Before any verifier code lands,** two things are committed or bound at the freeze:
   * Assurance's (Lain's) `sealed-v1-eco-bind` (spec §7.1 step 3);
   * the Auditor's E-gate suite (E1-E8 suites and corpus, spec §1.2).
3. **The verifier's builder is never its reviewer or its E-gate suite author** (spec §1.1).

## 4. Post-freeze revisions

| Rev | Date | Change | Spec sha256 after |
|---|---|---|---|
| R1 | 2026-10-20 | §5 rule 6: an Owner-named venue must be registered before the gate's first run, and no venue may be named or added for a gate after it records a FAIL (Motoko's nit). §7 untouched. | `267076e955516afa2a92c7c01167d7d092867abf1f32d6603d29d088f47bc6a5` |

The §7 text hash and `SEALED.sha256` are unchanged by R1. Lain's §7 wording nit is expected on top. The §7 text hash will
change then, by design, but the sealed-file commitments in `SEALED.sha256` must not.
