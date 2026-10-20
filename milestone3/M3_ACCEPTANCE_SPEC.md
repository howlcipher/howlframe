# Milestone 3 acceptance spec: full-stack web app and a genuine Howl ecosystem component

**Status:** DRAFT for review. It freezes once Assurance's held-out section (§7) lands and the independent review accepts it.
After the freeze, changes need a separately recorded, dated revision (directive §8, "Record decisions without retroactive
goalpost changes").
**Base:** `main` at `d03c1176` (Milestone 2 accepted by the Owner, `milestone2/MILESTONE2_DECISION.md:3-12`).
**Authority:** `HOWLFRAME_PRODUCT_DIRECTIVE.md` §6, Milestone 3 (lines 178-185), and nothing more. The directive sets **no
extra numeric M3 gates, no required component, and no mandatory parallel execution** (line 185). This spec adds none. Every
gate below is a pass/fail observable chosen for that project.
**Author:** Product (requirements and acceptance only; Product builds none of it).

Citation key: `DIR` = `HOWLFRAME_PRODUCT_DIRECTIVE.md`, `M1D` = `milestone1/MILESTONE1_DECISION.md`,
`M2P` = `milestone2/PLAN.md`, `M2D` = `milestone2/MILESTONE2_DECISION.md`, `M2R` = `milestone2/ASSURANCE_M2_FINAL_REVIEW.md`.

---

## 1. The two projects

What the directive requires (DIR:180-185):
* **P-WEB.** A complete full-stack web application with meaningful frontend and backend behaviour.
* **P-ECO.** A useful implementation or replacement of a genuine Howl ecosystem component, built with HowlFrame and not an
  artificial integration.

Rules that apply to both, carried forward from M1/M2 practice:
* Requirements and the acceptance suite are frozen **before implementation**. They are written by a party that does not
  implement (DIR:185, DIR:242; M2P:74-77).
* Clean-environment build and run, with no generated application in another language (DIR:148, DIR:171).
* Narrow runtime fixes are made only for a demonstrated blocker, each with a regression test (DIR:198, DIR:215-216; M2P:63-70).
* App frameworks stay in separate packages (DIR:99, DIR:185).

### 1.1 Independent reviewer (both projects)

For each project, the final reviewer is an **Assurance-role session** that wrote none of that project's code, its runtime fixes,
or its scripts, and took no part in an earlier review round of it. This is the same independence statement used in `M2R:3-6`.
The reviewer reruns every gate rather than trusting journals (M2R:14). The reviewer records a per-gate PASS/FAIL table and a
decision file: `milestone3/<project>/DECISION.md`.
* A self-reported success message is not evidence (DIR:158).
* Commits go through one shared account, so git authorship cannot show independence (M2R:5). The written assignment and the
  reviewer's statement are the record.

### 1.2 P-ECO gates: receipt and grant-audit verifier (the top-ranked candidate, §3)

A HowlFrame program (`.howl` built to `.hfbc` and run on the standalone VM). It takes a `howlframe.receipt/v0` file
(`docs/cli.md:37-68`), the expected artifact SHA-256, and a declared grant policy. It reports, in machine-readable form,
whether the run stayed within that policy. If the evidence later moves the ranking, §3 picks a different candidate, and that
candidate gets gates in this same shape by a recorded revision before it starts.

| # | Gate | PASS when (objective observable) |
|---|---|---|
| E1 | Frozen contract | `milestone3/eco/requirements/` defines the input formats (receipt v0 fields as in `docs/cli.md:42-61`; the policy format), every verdict and exit code, and the stderr shape. It is committed before the first implementation commit. The freeze-integrity `git diff <freeze-sha> HEAD -- milestone3/eco/requirements milestone3/eco/tests/acceptance` prints nothing at review time. |
| E2 | Built with HowlFrame, standalone | `howlframe build milestone3/eco/src/<tool>.howl -o <tool>.hfbc`, then `howlframe run [--allow-caps …] <tool>.hfbc …`. The only artifacts are `howlframe` and the `.hfbc`, with no generated Go/Python tool (same check as M2R:183). |
| E3 | Genuine receipts verify | On receipts the reviewer produces live (one granted and one denied run of each M1 app and of ticketdesk, with the documented grants), the tool reports `PASS` with exit 0, at 100%. |
| E4 | Deviations detected | On an independently authored corpus of altered receipts and policies, every case gets its frozen `FAIL` code and nonzero exit, at 100%. The corpus covers: artifact hash mismatch; a grant missing from the policy; a broader grant than the policy (for example coarse `network` vs `network:fetch=host:port`, as in M1 `02-healthwatch/REVIEW.md:28`); an allowed decision on a capability the policy denies; `effects_truncated: true`; limits that differ from the policy; a malformed or unknown-schema receipt. No false PASS. |
| E5 | Honest claims | The README states that receipts have **no signature or attestation** (`docs/cli.md:66`), so the tool checks consistency against a policy and does not prove authenticity. The reviewer's doc scan finds no tamper-proof or attestation claim. |
| E6 | Capability discipline | The tool runs with read-only filesystem grants for its inputs. Each removed grant gives the frozen denial exit, and the tool needs no network, process or environment grant. Checked as in the M2 startup matrix (M2R:169-176). |
| E7 | Clean environment | A fresh `git clone --no-local` under `env -i`, with empty `GOCACHE`/`GOMODCACHE`, `GOPROXY=off` and `GOTOOLCHAIN=local`, runs the documented script with exit 0 and all suites passing. The README steps also work from a `git archive` copy with no `.git` (the m7 lesson, M2R:67, M2R:221). |
| E8 | Real use, not artificial | P-WEB gate W8 runs the tool on the receipts of P-WEB's own runs as an acceptance step, and the reviewer records that run. |
| E9 | Held-out checks | Every check in §7 passes. |
| E10 | Independent review | A §1.1 reviewer records PASS on E1-E9 in `milestone3/eco/DECISION.md`. |

### 1.3 P-WEB gates: full-stack web application

The application domain is **selected or approved by an independent evaluator** and frozen before the build, as in M1
(`milestone1/SELECTION.md:3`; DIR:136). Extending ticketdesk with a real frontend is one allowed option. If it is chosen, the
§4 notes on m2/m3/m4 apply.

| # | Gate | PASS when (objective observable) |
|---|---|---|
| W1 | Frozen requirements and suites | `milestone3/web/requirements/` and the black-box API and browser suites are committed before implementation. The freeze-integrity diff is empty at review time (as in M2R:50). |
| W2 | Backend API | The independent black-box API suite passes at 100% against `howlframe run <server>.hfbc` started with the README command and **default runner ceilings**. The ceilings rule exists because the M2 suite once masked B1 with raised limits (M2D:756-758). |
| W3 | Meaningful frontend | The frontend source is HowlFrame, built with a HowlFrame web target and served by the P-WEB backend. An independent headless-browser end-to-end suite drives the frozen user workflows (create, read, update and delete through the UI, plus auth flows) and passes at 100%. |
| W4 | Frontend output safety | An XSS probe set (for example `<script>…`, `<img src=x onerror=…>` and attribute-break payloads in every user-editable field) renders as inert text, and no script runs in the browser suite. A source check confirms that no untrusted value reaches `set_html` without `html_escape`. The basis for this gate is `docs/howlboard_xss_set_html_issue.md`. |
| W5 | Auth enforced server-side | Every role and ownership rule in the frozen requirements is checked by direct API calls that bypass the UI (401/403 as specified). No secret appears in server logs or stored state (the same scan as M2R:134-135). |
| W6 | Reliable errors | Every non-2xx response from a request that reaches routing uses the frozen error envelope, and the UI shows a defined message for each one. The scope of this rule is set in the frozen requirements, using the m3/m17 lessons in §4. |
| W7 | Framework packaging | All web-framework or UI-helper code is in a separately packaged module, not in `internal/` core. Any core change is a demonstrated-blocker fix with a journal entry and a regression test (DIR:99, DIR:185, DIR:198). The reviewer lists core diffs since the freeze. |
| W8 | Capability evidence | The server runs with the minimum grants listed in the requirements, and each dropped grant gives the frozen denial exit. A `--receipt` is captured for the reviewer's runs and verified with the P-ECO tool (E8), with a `PASS` recorded. |
| W9 | Clean environment | Same as E7 (fresh clone and non-git copy). |
| W10 | Startup contract | Port-in-use (including the lost-race case) and bad-config startups give the frozen exit code and a prefixed stderr line, with no raw runtime object. See m12 in §4. |
| W11 | Held-out checks | Every check in §7 passes. |
| W12 | Independent review | A §1.1 reviewer records PASS on W1-W11 in `milestone3/web/DECISION.md`, with known limitations documented (DIR:153). |

---

## 2. Order: P-ECO first, then P-WEB

**Decision: P-ECO (the receipt and grant-audit verifier) first, then P-WEB.**

**Why (evidence):**
1. **Authority verification was manual in both milestones.**
   * M1 reviewers checked capability and limits by hand (`milestone1/01-ledgerlens/REVIEW.md:52`,
     `milestone1/02-healthwatch/REVIEW.md:30`, `milestone1/03-taskflow/REVIEW.md:32`; about 60/90/120 adversarial probes,
     M1D:50).
   * In M2 a grant-check mutation (`internal/vm/crypto.go:51`) left `go test ./...` at 76 ok and was caught only by a
     reviewer's hand probe (m9, M2R:223, M2R:375).
2. **Receipts were asked for but none is on record.** Every M1 `requirements/ACCEPTANCE.md` asks to "preserve … a
   `--receipt` of at least one denied run per application" (`milestone1/01-ledgerlens/requirements/ACCEPTANCE.md:84`,
   `02-healthwatch/…:74`, `03-taskflow/…:78`). `rg -l 'howlframe.receipt' milestone1 milestone2` finds no file, so no
   receipt is on record in either milestone tree. Receipts exist, but nothing checks them (`docs/cli.md:37-68`).
3. **P-WEB enlarges the authority surface.** It adds a browser target, a server and more grants, and M2's lesson is that
   evidence quality decides acceptance. B6 took seven rounds and was never passed (M2D:14-50, M2D:440-446). Building the
   verifier first gives P-WEB an objective capability gate (W8) instead of another round of hand probes.
4. **P-ECO is bounded and has a proven shape.** It is a standalone CLI on the standalone VM, which M1 showed works three
   times (M1D:5-11). P-WEB needs a frontend target and a browser test harness. Neither was exercised in M1/M2: a web UI was
   out of scope in M2 (M2P:83-85), so there is no evidence on record for that path, and it is the riskier project.

**What would reverse the order:** the P-ECO freeze review finds that receipt v0 cannot express the policy checks in E4
without a runtime change larger than a narrow fix (DIR:198). In that case P-WEB goes first, with W8 reduced by a recorded
revision to hand probes plus a captured receipt.

**Not parallel.** The directive does not require unconditional parallel work (DIR:185). The milestone stop rule is applied
per project (DIR:162, DIR:220).

---

## 3. Ranked ecosystem-component candidates

Ranked by strength of M1/M2 evidence. The owner component follows DIR:92-100.

| Rank | Candidate (owner) | M1/M2 friction it addresses | Evidence strength |
|---|---|---|---|
| 1 | **Receipt and grant-audit verifier** (HowlFrame: capability and execution authority, tooling). Hange's candidate. | Hand-run H/authority checks (M1 REVIEW H rows; M1D:50). The m9 mutation is undetected by the test suite (M2R:223). Receipts were required by M1 ACCEPTANCE but none is on record (§2.2). Round 1 asked for stronger tests on `inherit:filesystem` and `network:fetch=` grants (M2D:778-779). Receipts are unsigned and have no checker (`docs/cli.md:66`). | **Strong.** The friction recurs in both milestones, and HowlFrame owns this area outright. |
| 2 | **Pre-registered run-protocol and evidence checker** (HowlCreate: the "checking … execution" stage, DIR:96). It verifies script sha, `--expect-head` pin rules, the idle gate and noise gates, and whether the evidence bundle is complete before a reviewer starts. | B6 needed seven rounds, hand-written pin rules (`TestRound6/7ExpectHeadPinRules`, M2D:383-390, M2D:491-504), two INFRA-VOID runs and a venue waiver (M2D:14-50). The Round 7 exit status was inferred, not captured (M2D:231). | **Moderate.** The friction is heavy but concentrated in one gate (B6). Must not duplicate HowlPlane orchestration (DIR:102). |
| 3 | **Clean-room build and repro runner** (HowlCreate: build and execution). | `clean_repro.sh` reuses the host toolchain and module cache (M1D:89-90). It fails from a non-git copy (m7, M2R:67). Two separate scripts exist (`milestone1/scripts/`, `milestone2/scripts/`). CI uses Go 1.21 while the box uses 1.27.2 (M2R:346). | **Moderate.** The friction is real but tolerated in both milestones (documented-accepted). |
| 4 | **HowlBoard reimplementation** (HowlBoard: approvals and progress visibility). | M2 Owner decisions were transcribed from chat into records (M2R:19-37, M2D:7-12). An earlier HowlBoard frontend had a `set_html` XSS (`docs/howlboard_xss_set_html_issue.md`; `docs/journals/2026-08-14_dogfood_runtime_integrity_gate.md:41`). | **Weak.** No M1/M2 record shows a dashboard blocking work. The evidence predates the directive (August journals). It also overlaps P-WEB and could merge the two projects into one artificial integration. |
| — | HowlPlane piece (provider failover, orchestration). | The cloud VM launch failed for lack of usage (M2D:113). | **Weak and excluded.** One incident only, and DIR:102 and DIR:123 warn against duplicating HowlPlane. |

---

## 4. M2-waived items: are they M3 entry criteria?

All five were waived **for M2 only** (M2R:29-32, M2D:61-62). They do not carry over automatically, so if a project inherits
one it needs a fix, a requirements revision, or a new waiver.

| Id | What it is (from M2R §4) | M3 entry criterion? | Reason |
|---|---|---|---|
| **m2** | A bearer secret with one trailing space still authenticates. Go `net/http` strips trailing OWS (RFC 9110 §5.5) before the app sees it. No security impact (M2R:218, M2R:233-243). | **NOT** | It blocks neither project. If P-WEB reuses bearer auth, its frozen auth requirement must exclude RFC 9110 trailing OWS from the start (fix option (b), M2R:241). |
| **m3** | Transport-level rejections (malformed `%zz`, a 431 for an oversize header, `OPTIONS *`) are plain text or empty and have no `X-Request-Id`. `net/http` answers before routing (M2R:219, M2R:245-264). | **NOT** | Only syntactically invalid HTTP is affected, and P-ECO is not a server. P-WEB's W6 scopes the uniform envelope to requests that reach routing (option (b), M2R:263). The guide and README must document it. |
| **m4** | Semantically corrupt state passes the shallow startup check, starts, then answers 500. This is app code (`ticketdesk.howl:49-58`) (M2R:220, M2R:297-308). | **NOT** | It is a ticketdesk application defect, not a runtime one. If P-WEB extends ticketdesk, deep validation on load becomes a P-WEB requirement and is not a precondition for starting. |
| **m12** | A port-probe race: if the port is taken between the probe (`vm.go:3532`) and `ListenAndServe` (`vm.go:3045-3051`), the raw `{"phase":"runtime","code":"IO_ERROR",…}` object reaches stderr. Deterministically reproduced (M2R:224, M2R:282-295). | **ENTRY CRITERION for P-WEB** (not for P-ECO) | It is a HowlFrame runtime defect that any HowlFrame HTTP server inherits, including P-WEB's W10 startup contract. A strict XFAIL test already gives the objective gate: after a fix (bind once, `server.Serve(ln)`, M2R:292), `assurance_m10.py` must report XPASS, then the XFAIL entry is removed, then the run reports `5 tests, 0 failed` with no expected failures (M2R:318-323). |
| **m17** | When the declared Content-Length is not delivered, the server returns a 400 that is not in the uniform shape and has no `X-Request-Id` header, at `vm.go:2957-2963`. The upload is never executed (M2R:225, M2R:266-280). | **NOT** | Only clients that break HTTP framing see it, and browsers driving P-WEB do not. Fixing it needs an agreed error code (M2R:277-278). Record the choice (fix (a) or drop the connection (b)) in P-WEB's frozen requirements. |

Not in the list but relevant: **m9** (documented-accepted, a grant-coverage test gap, M2R:223). P-ECO's E4 corpus should
include the grant-scope case it describes.

---

## 5. Venue and noise rules (settled now, before any run)

These are based on what M2 actually did and learned:

1. **Deterministic functional gates run on the shared box.** This covers acceptance and browser suites, regressions, mutation
   checks and clean repro. M2's final review ran entirely there, on Linux with Go 1.27.2 and the Python 3 stdlib (M2R:11).
   Each gate gets **one reviewer run plus one clean-repro run, with 0 failures required**. A failure is never rerun to a pass
   (M2D:412-414, the "no restart" rule).
2. **Toolchain recorded.** Each decision records the Go version, OS and kernel. CI's Go 1.21 differs from the box (M2R:346),
   so a gate that CI is meant to cover must also pass on CI's version, or the gap is recorded.
3. **Timing-bound tests use wide margins.** The bound is at least 50× the measured worst case, which follows m10's 3 s bound
   against a measured 0.01-0.04 s (M2R:347-349). A timing failure counts as a failure unless a pre-registered noise gate was
   breached during it.
4. **Stress, durability or kill-storm gates are pre-registered before any code they test exists.** The terms include the run
   count with its flake-bound arithmetic (M2: 0/75 lets a 4% flake pass about 5% of the time; 0/225 puts the 95% upper bound
   under 1.4%, M2D:394-395, M2D:520). They also include the Round 7 idle gate and noise gates verbatim (M2D:360-379) and the
   mechanical decision rule (M2D:397-411).
5. **No isolated-venue gate unless an isolated venue is confirmed before adoption.** The box is a ballooned, fork-prone VM.
   Two B6 runs there were INFRA-VOID (M2D:83-84, M2D:118-122), and the planned PC run on William's machine never happened
   (M2D:10-12, M2D:184-185). A gate that needs isolation is adopted only after venue evidence is recorded: bare metal vs
   VM/WSL2, balloon, sleep, other workloads, `/dev/kmsg` (M2D:178-180). Otherwise it is not an M3 gate.
6. **On the shared box, a declared quiet window is required** for any timing-sensitive run: no other agent's builds, tests,
   storms or browser work (M2D:380-382). M2 had no artifact showing the box was idle (M2D:125-126), so M3 records one,
   for example a process listing at start and end.
7. **Fixed seeds and inputs.** Every randomized input uses a seed recorded in the frozen requirements (M2 used `20261019`,
   M2D:331). Fixtures are committed. Secrets are generated at runtime and never printed or written (M2R:69-71).
8. **Pins.** The reviewer states the code pin and shows that the reviewed tree differs from it only in docs, or reruns
   (M2R:39-53).

---

## 6. Out of scope

* New language features (paused by the Owner). The token benchmarks stay a regression guard only.
* The later competitive-validation program (DIR:249).
* A production HFIR or `-compile-bc` flip.
* OS-level sandboxing claims (DIR:244).
* Performance work beyond functional smoke checks.
* Desktop, mobile or game targets (DIR:189).
* Re-running M2's B6.
* Edits to `.github/workflows/` (the token lacks `workflow` scope, M2R:330-334). Wiring CI for the m10 tests is the Owner's
  call.
* Any fixed number of endpoints, users or pass rates beyond the per-gate 100% rules above (DIR:185).

## 7. Held-out checks (PLACEHOLDER: authored by Assurance)

> **TODO, authored by Lain (Assurance, non-builder).** Builders of P-ECO or P-WEB must not see, author or influence this
> section or its fixtures before their project's final review. This is the held-out discipline used for the frozen benchmark
> sets. Product leaves it empty on purpose.

* P-ECO held-out checks: _TODO (Assurance)_
* P-WEB held-out checks: _TODO (Assurance)_

## 8. Evidence gaps (stated, not guessed)

* **Browser E2E on the box:** no evidence on record of a headless-browser test harness in M1/M2. W3 depends on one.
* **HowlFrame web-target maturity for a full frontend:** no M1/M2 evidence. The latest evidence is the August HowlBoard
  dogfood journals.
* **Receipts in HTTP-server mode:** `docs/cli.md:37-41` documents `--receipt` for `run --target bytecode`, but no record shows a
  receipt from a long-running server. E3 and W8 assume one can be written at shutdown, and the P-ECO freeze must confirm it.
* **Whether m9's mutation shows up in a receipt:** not established. E4 includes the case only if the frozen contract shows
  the receipt carries it.

## Sources

* `HOWLFRAME_PRODUCT_DIRECTIVE.md` §§4, 5, 6 (M3: lines 178-185), 7.2-7.3, 8.
* `milestone1/MILESTONE1_DECISION.md`; `milestone1/SELECTION.md`; `milestone1/0{1,2,3}-*/REVIEW.md`;
  `milestone1/0{1,2,3}-*/requirements/ACCEPTANCE.md`.
* `milestone2/PLAN.md`; `milestone2/MILESTONE2_DECISION.md` (acceptance `d03c1176`; Round 7 record #201 `e41b6830`;
  isolation rerun record #202 `883f26de`; pre-registration terms); `milestone2/ASSURANCE_M2_FINAL_REVIEW.md` (#203, `dd8f2728`);
  `milestone2/ticketdesk/README.md` "Known limitations".
* `docs/cli.md` (`--receipt`, lines 37-68); `docs/howlboard_xss_set_html_issue.md`;
  `docs/journals/2026-08-14_dogfood_runtime_integrity_gate.md`.
