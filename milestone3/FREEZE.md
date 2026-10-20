# Milestone 3 SPEC freeze

**Freeze voted 2026-10-20.** The seats voted 5/5 to freeze the spec and then build P-ECO, the receipt and grant-audit
verifier.
* **The spec is FROZEN when #206 merges to `main`.**
* R1-R7 are pre-merge content. Each is listed with its ack status in `OWNER_ACK_2026-10-20.md` §7.
* The post-freeze revision rule (§4) applies from that merge on.

## 1. What is frozen

| Field | Value |
|---|---|
| Spec freeze SHA (base) | `e6fdd48a0c60564a2ad3833cf2b990dff7ad7c14` (main; squash-merge of #205) |
| **Frozen spec content** | **`e6fdd48a` + R1–R7** (§4; R1 is superseded by R4; R6 does not change the spec), as merged by #206. The final `milestone3/M3_ACCEPTANCE_SPEC.md` sha256 is **`8a20258e80b18384842bca6b65b27869dce143669610e2236e3735b5b2f8782c`**. |
| Spec path | `milestone3/M3_ACCEPTANCE_SPEC.md` |
| Spec sha256 at the base `e6fdd48a` | `630b6ae4dde33e98a893ef209275866b78a154efddb77cd6209fef13fee679b4` |
| Held-out seal reference | `milestone3/heldout/SEALED.sha256`, sha256 `2bdb70870f55cbe029ecaab1889955ec878931671ab834aa5ce0518a0d3acf3b`, unchanged from `e6fdd48a` through R7 (sealed-v1; spec §7.3) |
| §7 text sha256, frozen (after R2; unchanged by R3-R7) | `7dbeb891f72c35c8416ea59e67ce4d1fb26877c93ca870a2f5e1f7aabdb3bd09`. At `e6fdd48a` and R1: `9d630db86e67bd1cdf7a8dfbc00ebd839ba21e22551dae403c12038ccc4bd798`. The span runs from the start of the `## 7.` line up to, but not including, the `## 8.` line, and includes the final newline. |
| Owner acknowledgement | [`OWNER_ACK_2026-10-20.md`](OWNER_ACK_2026-10-20.md): items 1, 2 and 4 APPROVED, item 3 DECLINED (2026-10-20 15:23 ET, relayed by EngMgr). Owner-named-venue path dropped and R1 superseded (16:32 ET, that file's §4). History checks manual-only, R6 (16:48 ET, that file's §5). R5 acked as amended, W8 fail-closed reverted (16:51 ET, that file's §6). Ack status of every revision: that file's §7. |

**A note on the intermediate spec hashes.**
* `b44ae42a84092839ca4fb7b4228e7c160b92c80ed592bdcb77d3e1d036b8be70` is the spec sha256 at `3fbd11ca` (`e6fdd48a` + R1 + R2).
  Motoko's #206 review cites it as final. It is the **pre-R3** value.
* `b15a2028d8a3db45d6d7b1983bdf1060d26abd86e4e5f90772f6f388e7fcca60` is the value at `b2867afa` (after R3). It is the **pre-R4** value.
* `3a3165bfcc4c41c2f8ef8cf977d0850a5386ada1960d5b7890250153ceb28a23` is the value at `6cb2dbe8`/`8f78a8cc` (after R4). It is the **pre-R5** value.
* `d5513fd729a519cacc58a436473dc6f2e6c61e148d6d835ff67bb6bf969e1cb7` is the value at `f0af1151`/`c8c55939` (after R5; R6 does not change the spec). It is the **pre-R7** value.
* R3, R4, R5 and R7 edit the spec outside §7, so the final hash is `8a20258e80b18384842bca6b65b27869dce143669610e2236e3735b5b2f8782c`. The §7 hash and `SEALED.sha256` do not change.

Verify with (`<ref>` = `e6fdd48a`, `ee77db33` (R1), `3fbd11ca` (R2), `b2867afa` (R3), `6cb2dbe8` (R4), `f0af1151` (R5), or the merge commit of #206 (R7/final; R6 leaves the spec, §7 and `SEALED.sha256` unchanged)):
* `git show <ref>:milestone3/M3_ACCEPTANCE_SPEC.md | sha256sum`
  * `e6fdd48a`: `630b6ae4…`
  * R1: `267076e9…`
  * R2: `b44ae42a…`
  * R3: `b15a2028…`
  * R4: `3a3165bf…`
  * R5 and R6: `d5513fd7…`
  * final (R7): `8a20258e…`
* `git show <ref>:milestone3/M3_ACCEPTANCE_SPEC.md | awk '/^## 7\./{p=1} /^## 8\./{p=0} p' | sha256sum`
  * `9d630db8…` at `e6fdd48a`/R1
  * `7dbeb891…` at R2 through final
* `git show <ref>:milestone3/heldout/SEALED.sha256 | sha256sum`: `2bdb7087…` at every ref
* `go test ./milestone3/` checks the final values (§5), and also pins this file's own sha256, which this file does not list.

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
   * Its git-history checks are **manual-only, with a saved evidence file that blocks the merge** (R6; see §5).

## 4. Post-freeze revisions

**Revision rule.**
* **The spec is FROZEN when #206 merges to `main`.** From that merge on, the spec changes only by a **dated revision row
  here**, with the spec sha256 after.
* Each such revision carries an **Owner ack**, recorded in an `OWNER_ACK_<date>.md` file.
* A revision is **never made after any result of a gate that it affects is known**.
* The list of Owner-reserved decisions is in the spec status block (R5). Any change that relaxes a gate or touches one of them
  needs its own Owner ack.

**Pre-merge content.**
* R1-R7 below were made before #206 merges and were seat-reviewed. They are part of the frozen content, not post-freeze
  revisions, so the Owner-ack requirement above does not apply to them.
* Each one's ack status is listed in `OWNER_ACK_2026-10-20.md` §7:
  * R1: Owner-acked as superseded (16:32 ET).
  * R2: not covered.
  * R3: not covered. Its "first run on that venue" wording for §5 rule 6 **did** touch the Owner-reserved venue rule (it
    loosened R1). That is moot, because R4 superseded it with an Owner ack (Motoko's #206 review 5591460537, correcting
    EngMgr's 16:37 ET determination).
  * R4: Owner-acked (16:32 ET).
  * R5: Owner-acked as amended (16:51 ET).
  * R6: Owner-acked (16:48 ET).
  * R7: the W8/§2 revert is Owner-acked (16:51 ET); the rest is not covered.
  * The seats' binding conditions in §3 are not covered.
* EngMgr's 16:37 ET determination still holds for R2 and §3: they only reword or tighten the spec.

| Rev | Date | Change | Spec sha256 after |
|---|---|---|---|
| R1 | 2026-10-20 | **SUPERSEDED by R4 (Owner decision 2026-10-20 16:32 ET).** §5 rule 6: an Owner-named venue must be registered before the gate's first run on that venue (wording as amended by R3), and no venue may be named or added for a gate after it records a FAIL (Motoko's nit). §7 untouched. | `267076e955516afa2a92c7c01167d7d092867abf1f32d6603d29d088f47bc6a5` |
| R2 | 2026-10-20 | §7.1 step 5 wording (Assurance, Lain; Motoko's nit): the reviewer "runs `sha256sum -c` itself" instead of "verifies in person". No change of meaning, no check, count or commitment change. `SEALED.sha256` unchanged (`2bdb7087…`). §7 text sha256 becomes `7dbeb891f72c35c8416ea59e67ce4d1fb26877c93ca870a2f5e1f7aabdb3bd09`. | `b44ae42a84092839ca4fb7b4228e7c160b92c80ed592bdcb77d3e1d036b8be70` |
| R3 | 2026-10-20 | Heinrich's and Motoko's #206 reviews. §5 rule 6 R1 wording (superseded by R4) "first run **on that venue**", so an Owner-named venue can still be registered for a gate that has run elsewhere with no FAIL (the no-new-venue-after-FAIL rule is kept). Status line DRAFT → FROZEN, plus the revision-rule sentence. The Owner block and §1.1.1 Owner ack give William's selected option "Approve, but my PC isn't a backup venue" and the chat record. The Owner-ack link is set to `milestone3/OWNER_ACK_2026-10-20.md`. `OWNER_ACK_2026-10-20.md` is restructured. §7 untouched. | `b15a2028d8a3db45d6d7b1983bdf1060d26abd86e4e5f90772f6f388e7fcca60` |
| R4 | 2026-10-20 | **Owner decision, 2026-10-20 16:32 ET** (relayed by EngMgr; Owner ack: [`OWNER_ACK_2026-10-20.md`](OWNER_ACK_2026-10-20.md) §4): "Drop the Owner-named venue path; blocked checks can only be waived, never passed". Made before any gate result; no gate has run. Changes:<br>• §5 rule 6: the Owner-named-venue path is removed, together with the R1 text (superseded), the `venue_evidence.txt` minimums that applied only to an Owner-named venue, and "How an Owner-named venue is tried". BLOCKED-VENUE now has exactly one outcome, `WAIVED, NOT PASSED`, and no venue may be named or added for any gate.<br>• §1.1.1 template: the Owner-named venue, consent and window fields are removed.<br>• Owner block item 3 notes the 16:32 decision.<br>• §7 untouched. | `3a3165bfcc4c41c2f8ef8cf977d0850a5386ada1960d5b7890250153ceb28a23` |
| R5 | 2026-10-20 | Motoko's #206 review 5591460537 and Heinrich's nits. **Owner-acked 2026-10-20 16:51 ET as amended** ([`OWNER_ACK_2026-10-20.md`](OWNER_ACK_2026-10-20.md) §6). The W8 fail-closed part was reverted per the Owner, in R7. Made before any gate result; no gate has run.<br>• §5 rule 6: a gate whose venue fails isolation, or is BLOCKED-VENUE for any reason, cannot be PASSED, and its project (and M3) cannot be ACCEPTED while it is open. The only outcome is `WAIVED, NOT PASSED`. The dangling "with the venue evidence" now names the evidence of the cause.<br>• W8 and §2: fail-closed dependency. A waived P-ECO was not the "accepted P-ECO tool" without a separate Owner decision. **Reverted by R7, per the Owner (16:51 ET).**<br>• Status line: "`e6fdd48a` + R1–R5".<br>• Status block: the Owner-reserved list, moved here from this file's §4 note.<br>• §7 untouched. | `d5513fd729a519cacc58a436473dc6f2e6c61e148d6d835ff67bb6bf969e1cb7` |
| R6 | 2026-10-20 | **Owner decision, 2026-10-20 16:48 ET** (relayed by EngMgr; Owner ack: [`OWNER_ACK_2026-10-20.md`](OWNER_ACK_2026-10-20.md) §5): "Run the freeze history checks manually; leave CI as is". Made before any gate result; no gate has run, and no verifier or suite commit exists.<br>• §3 condition 3 and §5: the history checks are **MANUAL-ONLY**, with an evidence file and a merge block (§5).<br>• `spec_freeze_test.go`: the history test skips on a shallow clone. On a full clone it fails closed as before. Every non-history check runs at any clone depth.<br>• `.github/workflows` is unchanged.<br>• **The spec is unchanged**: it does not carry the binding conditions, so there is nothing to mirror. | `d5513fd729a519cacc58a436473dc6f2e6c61e148d6d835ff67bb6bf969e1cb7` (unchanged) |
| R7 | 2026-10-20 | Heinrich's last #206 blocker, and the Owner decision of 2026-10-20 16:51 ET (relayed by EngMgr; [`OWNER_ACK_2026-10-20.md`](OWNER_ACK_2026-10-20.md) §6): "No: my waiver of a verifier check is enough for the web app to rely on it". Made before any gate result; no gate has run.<br>• W8 and §2 (**Owner-acked**): R5's fail-closed rule is reverted. An E-gate recorded `WAIVED, NOT PASSED` with a dated Owner decision counts toward the accepted P-ECO for W8, with no second decision. An E-gate neither PASSED nor Owner-waived blocks W8.<br>• §5 rule 6 (not covered): "fails isolation" is defined as exactly rule 2's INFRA test from M2 Round 7: ≥80% heartbeat-gap coverage, or a fork, balloon or steal overlap, or INFRA-VOID on a noise breach (M2's noise gates, including a heartbeat gap over 2 s and VM-fork/balloon kmsg records). No new trigger.<br>• Status block (not covered): FROZEN when #206 merges; R1-R7 are pre-merge content; the revision rule applies from the merge.<br>• `spec_freeze_test.go`: the source path and extension rule is case-insensitive; HFIR `.json` under verifier-source paths is source.<br>• §7 untouched. | `8a20258e80b18384842bca6b65b27869dce143669610e2236e3735b5b2f8782c` |

The §7 text hash and `SEALED.sha256` are unchanged by R1 and R3-R7. R2 changed the §7 text hash and left `SEALED.sha256` unchanged.

## 5. CI pins (`milestone3/spec_freeze_test.go`)

`go test ./...` runs the package `github.com/howlcipher/howlframe/milestone3`, which has test files only. Its pins are the
**post-R7 values**: the spec hash pinned is the value after revisions R1-R7 (R6 does not change the spec), not the `e6fdd48a`
value in §1.

| Pinned | sha256 |
|---|---|
| `milestone3/M3_ACCEPTANCE_SPEC.md` (post-R7) | `8a20258e80b18384842bca6b65b27869dce143669610e2236e3735b5b2f8782c` |
| `milestone3/heldout/SEALED.sha256` | `2bdb70870f55cbe029ecaab1889955ec878931671ab834aa5ce0518a0d3acf3b` |
| Spec §7 text span (`## 7.` up to `## 8.`) | `7dbeb891f72c35c8416ea59e67ce4d1fb26877c93ca870a2f5e1f7aabdb3bd09` |
| `milestone3/OWNER_ACK_2026-10-20.md` | `380370403e266310d43cc09abdbf67b27ac3617be2ff4a063a9cde8d9cbc082f` |
| `milestone3/FREEZE.md` (this file) | pinned in `spec_freeze_test.go` (`pinFreeze`) only. It is not listed here, because a file cannot record its own hash. |

This file's own sha256 is pinned in `milestone3/spec_freeze_test.go`, so any edit to it needs a re-pin there. Any change to a
pinned file needs a dated revision in §4 and a new pin in the same commit. R4 filled the former PENDING venue-path slot in `OWNER_ACK_2026-10-20.md` and re-pinned it.

The test also enforces §3.

**Verifier source** (paths under `milestone3/`):
* every file under `eco/src/` (spec §1.2, E2) or `verifier/`;
* every non-Markdown file under `eco/` outside `requirements/`, `tests/`, `evidence/` and `heldout/`;
* **every `.howl`, `.hfbc` or `.go` file anywhere under `milestone3/`**, including `eco/tests/`, `heldout/` and `web/`,
  unless it is on the source allowlist.
* HFIR `.json` files are covered by the first two rules: every file under `eco/src/` or `verifier/`, and every non-Markdown
  file under non-allowlisted `eco/` paths, is source, whatever its extension. `.json` under `eco/tests/`, `eco/evidence/`,
  `eco/requirements/` or `eco/heldout/` is data.
* **Matching is case-insensitive** (R7) for both the path prefixes and the extensions, so `.GO`, `.Howl` and `ECO/SRC/` all
  count.

**Source allowlist** (explicit and minimal; the same list as `sourceAllowlist` in the test):

| Path | Pin |
|---|---|
| `milestone3/spec_freeze_test.go` | none (it holds the pins and cannot pin itself) |

* Nothing else under `milestone3/` is a `.howl`, `.hfbc` or `.go` file today.
* Adding an entry needs a dated §4 revision and a sha256 pin in the test, made before any verifier code. The test checks
  every non-empty pin.
* **E-gate suite files.** The Auditor's suite lives under `eco/tests/`. Data files (JSON receipts, policies, `E5_ALLOWLIST`,
  expected outputs) are not verifier source.
  * A suite `.howl`, `.hfbc` or `.go` file (for example a fixture program) **is** verifier source, unless the Auditor has it
    allowlisted with a pin before verifier code exists.
  * This stops a builder from hiding verifier logic in `eco/tests/`. It also stops the suite from being edited after the
    fact.
  * An unpinned suite program counts as a verifier-source commit by its author, so it collides with condition 3 below.
* **P-WEB.** P-WEB `.howl` sources under `milestone3/web/` fall under the same rule. Before any P-WEB code exists, P-WEB's
  requirements freeze must add a dated revision that scopes them out of this rule.

* **Conditions 1-2.** The test fails if any verifier source exists unless all of these exist:
  * `milestone3/FREEZE.md`;
  * `milestone3/eco/requirements/FREEZE.md`;
  * a `sealed-v1-eco-bind` row in spec §7.3;
  * `milestone3/eco/tests/acceptance/`.
* **Condition 3: history checks, MANUAL-ONLY (R6, Owner decision 2026-10-20 16:48 ET).** These checks run over the full git
  history of `milestone3/`.
  * **What fails:**
    * any commit touching verifier source or `eco/tests/` that has **no** `Agent:` trailer;
    * any `Agent:` value on a verifier-source commit (a builder) that also appears on an `eco/tests/` commit (a suite author);
    * once `milestone3/eco/DECISION.md` exists, a reviewer who is a builder.
  * **The suite-author set must be disjoint from the builder set.** That is how the Auditor's suite coexists with the
    verifier: it sits in `eco/tests/` and is committed only by non-builders.
  * **Convention:** every commit touching these paths carries `Agent: <agent id>`. Product's freeze commits carry
    `Agent: bot-product-0001`. They touch only the allowlisted test file and docs, so they are not builder commits.
  * **How to run them (manual-only):**
    1. Get full history, with `git fetch --unshallow` or a fresh full clone.
    2. Check out the commit under test (`<head-sha>`).
    3. Run `go test -count=1 -v ./milestone3/...` from the repo root.
    4. Save the **full output** as `milestone3/eco/evidence/freeze_history_<head-sha>.txt`.
  * **Merge block:**
    * **Before any commit merges that touches `eco/tests/`, verifier source or `eco/DECISION.md`**, a **non-builder** checks
      that file for `<head-sha>`.
    * The file must show `TestM3VerifierReviewerNotBuilder` as `--- PASS`, with no `--- FAIL` and no `--- SKIP` for it.
    * **A missing evidence file, a failed or skipped run, or a file for another SHA blocks the merge.**
  * **In the test:**
    * On a full clone, the history test fails closed exactly as before.
    * If git history is unavailable, it fails.
    * On a shallow clone (CI's `actions/checkout@v4` default `fetch-depth: 1`), it calls `t.Skip` with "history checks are
      manual-only per Owner decision 2026-10-20 16:48 ET; run on a full clone and save evidence (see milestone3/FREEZE.md
      R6)".
    * The history test holds no non-history check.
    * Every non-history check runs and fails in CI at any clone depth: the pins, the allowlist pins, the verifier-source
      rule against `FREEZE.md`, the bind and the suite (conditions 1-2), and the `DECISION.md` `Reviewer:` line
      (`TestM3DecisionNamesReviewer`).
  * `.github/workflows` is unchanged (Owner decision).
