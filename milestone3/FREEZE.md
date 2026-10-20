# Milestone 3 SPEC freeze

**Frozen 2026-10-20.** The seats voted 5/5 to freeze the spec and then build P-ECO, the receipt and grant-audit verifier.

## 1. What is frozen

| Field | Value |
|---|---|
| Spec freeze SHA (base) | `e6fdd48a0c60564a2ad3833cf2b990dff7ad7c14` (main; squash-merge of #205) |
| **Frozen spec content** | **`e6fdd48a` + R1 + R2 + R3 + R4** (§4; R1 is superseded by R4), as merged by this PR. The final `milestone3/M3_ACCEPTANCE_SPEC.md` sha256 is **`3a3165bfcc4c41c2f8ef8cf977d0850a5386ada1960d5b7890250153ceb28a23`**. |
| Spec path | `milestone3/M3_ACCEPTANCE_SPEC.md` |
| Spec sha256 at the base `e6fdd48a` | `630b6ae4dde33e98a893ef209275866b78a154efddb77cd6209fef13fee679b4` |
| Held-out seal reference | `milestone3/heldout/SEALED.sha256`, sha256 `2bdb70870f55cbe029ecaab1889955ec878931671ab834aa5ce0518a0d3acf3b`, unchanged from `e6fdd48a` through R4 (sealed-v1; spec §7.3) |
| §7 text sha256, frozen (after R2; unchanged by R3 and R4) | `7dbeb891f72c35c8416ea59e67ce4d1fb26877c93ca870a2f5e1f7aabdb3bd09`. At `e6fdd48a` and R1: `9d630db86e67bd1cdf7a8dfbc00ebd839ba21e22551dae403c12038ccc4bd798`. The span runs from the start of the `## 7.` line up to, but not including, the `## 8.` line, and includes the final newline. |
| Owner acknowledgement | [`OWNER_ACK_2026-10-20.md`](OWNER_ACK_2026-10-20.md): items 1, 2 and 4 APPROVED, item 3 DECLINED (2026-10-20 15:23 ET, relayed by EngMgr). Owner-named-venue path dropped and R1 superseded (16:32 ET, that file's §4). R2, R3 and §3 are not covered (that file's §5). |

**A note on `b44ae42a` and `b15a2028`.**
* `b44ae42a84092839ca4fb7b4228e7c160b92c80ed592bdcb77d3e1d036b8be70` is the spec sha256 at `3fbd11ca` (`e6fdd48a` + R1 + R2).
  Motoko's #206 review cites it as final. It is the **pre-R3** value.
* `b15a2028d8a3db45d6d7b1983bdf1060d26abd86e4e5f90772f6f388e7fcca60` is the value at `b2867afa` (after R3). It is the **pre-R4** value.
* R3 and R4 edit the spec outside §7, so the final hash is `3a3165bfcc4c41c2f8ef8cf977d0850a5386ada1960d5b7890250153ceb28a23`. The §7 hash and `SEALED.sha256` do not change.

Verify with (`<ref>` = `e6fdd48a`, `ee77db33` (R1), `3fbd11ca` (R2), `b2867afa` (R3), or the merge commit of this PR (R4/final)):
* `git show <ref>:milestone3/M3_ACCEPTANCE_SPEC.md | sha256sum`
  * `e6fdd48a`: `630b6ae4…`
  * R1: `267076e9…`
  * R2: `b44ae42a…`
  * R3: `b15a2028…`
  * final (R4): `3a3165bf…`
* `git show <ref>:milestone3/M3_ACCEPTANCE_SPEC.md | awk '/^## 7\./{p=1} /^## 8\./{p=0} p' | sha256sum`
  * `9d630db8…` at `e6fdd48a`/R1
  * `7dbeb891…` at R2, R3 and final
* `git show <ref>:milestone3/heldout/SEALED.sha256 | sha256sum`: `2bdb7087…` at every ref
* `go test ./milestone3/` checks the final values (§5).

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
| Alternate venue | none, and no venue can be added (spec §5 rule 6, R4). Not set at requirements freeze. The Owner-named venue, consent and window fields were removed from the §1.1.1 template by R4. |
| Owner ack | carried from [`OWNER_ACK_2026-10-20.md`](OWNER_ACK_2026-10-20.md) |

## 3. Binding conditions (set by the seats)

1. **This file merges to `main` before any P-ECO verifier code is written to the repo, pinned or built.** "Commit" alone is not enough.
2. **Before any verifier code lands,** two things are committed or bound at the freeze:
   * Assurance's (Lain's) `sealed-v1-eco-bind` (spec §7.1 step 3);
   * the Auditor's E-gate suite (E1-E8 suites and corpus, spec §1.2).
3. **The verifier's builder is never its reviewer or its E-gate suite author** (spec §1.1).

## 4. Post-freeze revisions

**Revision rule.**
* After this freeze merges, the spec changes only by a **dated revision row here**, with the spec sha256 after.
* Each such revision carries an **Owner ack**, recorded in an `OWNER_ACK_<date>.md` file.
* A revision is **never made after any result of a gate that it affects is known**.
* R1-R4 are part of the frozen content: they were made before this file merges and were seat-reviewed.
  * R2 and R3 are **not** covered by an Owner ack (`OWNER_ACK_2026-10-20.md` §5).
  * R1 touched the Owner-reserved venue rule (§5 rule 6). The Owner's 16:32 ET decision acked it as superseded
    (`OWNER_ACK_2026-10-20.md` §4).
  * R4 carries that decision.

| Rev | Date | Change | Spec sha256 after |
|---|---|---|---|
| R1 | 2026-10-20 | **SUPERSEDED by R4 (Owner decision 2026-10-20 16:32 ET).** §5 rule 6: an Owner-named venue must be registered before the gate's first run on that venue (wording as amended by R3), and no venue may be named or added for a gate after it records a FAIL (Motoko's nit). §7 untouched. | `267076e955516afa2a92c7c01167d7d092867abf1f32d6603d29d088f47bc6a5` |
| R2 | 2026-10-20 | §7.1 step 5 wording (Assurance, Lain; Motoko's nit): the reviewer "runs `sha256sum -c` itself" instead of "verifies in person". No change of meaning, no check, count or commitment change. `SEALED.sha256` unchanged (`2bdb7087…`). §7 text sha256 becomes `7dbeb891f72c35c8416ea59e67ce4d1fb26877c93ca870a2f5e1f7aabdb3bd09`. | `b44ae42a84092839ca4fb7b4228e7c160b92c80ed592bdcb77d3e1d036b8be70` |

| R3 | 2026-10-20 | Heinrich's and Motoko's #206 reviews. §5 rule 6 R1 wording (superseded by R4) "first run **on that venue**", so an Owner-named venue can still be registered for a gate that has run elsewhere with no FAIL (the no-new-venue-after-FAIL rule is kept). Status line DRAFT → FROZEN, plus the revision-rule sentence. The Owner block and §1.1.1 Owner ack give William's selected option "Approve, but my PC isn't a backup venue" and the chat record. The Owner-ack link is set to `milestone3/OWNER_ACK_2026-10-20.md`. `OWNER_ACK_2026-10-20.md` is restructured. §7 untouched. | `b15a2028d8a3db45d6d7b1983bdf1060d26abd86e4e5f90772f6f388e7fcca60` |
| R4 | 2026-10-20 | **Owner decision, 2026-10-20 16:32 ET** (relayed by EngMgr; Owner ack: [`OWNER_ACK_2026-10-20.md`](OWNER_ACK_2026-10-20.md) §4): "Drop the Owner-named venue path; blocked checks can only be waived, never passed". Made before any gate result; no gate has run. Changes:<br>• §5 rule 6: the Owner-named-venue path is removed, together with the R1 text (superseded), the `venue_evidence.txt` minimums that applied only to an Owner-named venue, and "How an Owner-named venue is tried". BLOCKED-VENUE now has exactly one outcome, `WAIVED, NOT PASSED`, and no venue may be named or added for any gate.<br>• §1.1.1 template: the Owner-named venue, consent and window fields are removed.<br>• Owner block item 3 notes the 16:32 decision.<br>• §7 untouched. | `3a3165bfcc4c41c2f8ef8cf977d0850a5386ada1960d5b7890250153ceb28a23` |

The §7 text hash and `SEALED.sha256` are unchanged by R1, R3 and R4. b44ae42a was the spec hash after R2 (pre-R3). Lain's §7 wording nit is expected on top. The §7 text hash will
change then, by design, but the sealed-file commitments in `SEALED.sha256` must not. R2 is that nit: the §7 text hash changed as expected and `SEALED.sha256` did not.

## 5. CI pins (`milestone3/spec_freeze_test.go`)

`go test ./...` runs the package `github.com/howlcipher/howlframe/milestone3`, which has test files only. Its pins are the
**post-R4 values**: the spec hash pinned is the value after revisions R1-R4, not the `e6fdd48a` value in §1.

| Pinned | sha256 |
|---|---|
| `milestone3/M3_ACCEPTANCE_SPEC.md` (post-R4) | `3a3165bfcc4c41c2f8ef8cf977d0850a5386ada1960d5b7890250153ceb28a23` |
| `milestone3/heldout/SEALED.sha256` | `2bdb70870f55cbe029ecaab1889955ec878931671ab834aa5ce0518a0d3acf3b` |
| Spec §7 text span (`## 7.` up to `## 8.`) | `7dbeb891f72c35c8416ea59e67ce4d1fb26877c93ca870a2f5e1f7aabdb3bd09` |
| `milestone3/OWNER_ACK_2026-10-20.md` | `5b8a7e7a04b0b887ba85ef2da946513f80674f8de9be03f242fad181611a7ce3` |

This file is not pinned, because it records these pins. Any change to a pinned file needs a dated revision in §4 and a new
pin in the same commit. R4 filled the former PENDING venue-path slot in `OWNER_ACK_2026-10-20.md` and re-pinned it.

The test also enforces §3:
* **Conditions 1-2.** It fails if P-ECO verifier source exists (any file under `milestone3/eco/src/`, or any non-Markdown
  file under `milestone3/eco/` outside `requirements/`, `tests/`, `evidence/` and `heldout/`) unless all of these exist:
  * `milestone3/FREEZE.md`;
  * `milestone3/eco/requirements/FREEZE.md`;
  * a `sealed-v1-eco-bind` row in spec §7.3;
  * `milestone3/eco/tests/acceptance/`.
* **Condition 3.** Once `milestone3/eco/DECISION.md` exists, it must carry a `Reviewer: <agent id>` line. The test fails if any
  commit touching verifier source has an `Agent: <agent id>` git trailer equal to that reviewer.
  * **Convention:** builders add `Agent: <agent id>` trailers. No commit on record carries one yet.
  * **History:** the check needs full git history and **fails closed** on a shallow or missing history. CI's
    `actions/checkout@v4` uses its default `fetch-depth: 1`, which is shallow. So before `DECISION.md` lands, `fetch-depth: 0`
    must be set in `.github/workflows/ci.yml`. That needs `workflow` scope, which is the Owner's call.
