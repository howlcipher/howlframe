# Milestone 3 acceptance spec: full-stack web app and a genuine Howl ecosystem component

**Status:** DRAFT for review (revision 2, addressing Motoko's review of `c85cf2fa`,
[#205 review 5587744378](https://github.com/howlcipher/howlframe/pull/205#pullrequestreview-5587744378)). It freezes once
Assurance's held-out section (§7) lands and the independent review accepts it. After the freeze, changes need a separately
recorded, dated revision (directive §8, "Record decisions without retroactive goalpost changes").
**Base:** `main` at `d03c1176` (Milestone 2 accepted by the Owner, `milestone2/MILESTONE2_DECISION.md:3-12`).
**Authority:** `HOWLFRAME_PRODUCT_DIRECTIVE.md` §6, Milestone 3 (lines 178-185), and nothing more. The directive sets **no
extra numeric M3 gates, no required component, and no mandatory parallel execution** (line 185). This spec adds none. Every
gate below is a pass/fail observable chosen for that project.
**Author:** Product (requirements and acceptance only; Product builds none of it).
**Owner acknowledgement needed at spec acceptance:** the W3 frontend exception in §1.3.1.

Citation key: `DIR` = `HOWLFRAME_PRODUCT_DIRECTIVE.md`, `M1D` = `milestone1/MILESTONE1_DECISION.md`,
`M2P` = `milestone2/PLAN.md`, `M2D` = `milestone2/MILESTONE2_DECISION.md`, `M2R` = `milestone2/ASSURANCE_M2_FINAL_REVIEW.md`.

---

## 1. The two projects

What the directive requires (DIR:180-185):
* **P-ECO.** A useful implementation or replacement of a genuine Howl ecosystem component, built with HowlFrame and not an
  artificial integration.
* **P-WEB.** A complete full-stack web application with meaningful frontend and backend behaviour.

Rules that apply to both, carried forward from M1/M2 practice:
* Requirements and the acceptance suites are frozen **before implementation**. They are written by a party that does not
  implement (DIR:185, DIR:242; M2P:74-77).
* Clean-environment build and run, with no generated application in another language (DIR:148, DIR:171). P-WEB's browser side
  has one narrow, recorded exception (§1.3.1).
* Narrow runtime fixes are made only for a demonstrated blocker, each with a regression test (DIR:198, DIR:215-216; M2P:63-70).
* App frameworks stay in separate packages (DIR:99, DIR:185).

### 1.1 Independent reviewer, suite authors and gate format

* **Final reviewer (each project).** An **Assurance-role session** that wrote none of that project's code, its runtime fixes,
  or its scripts, and took no part in an earlier review round of it. This is the same independence statement used in
  `M2R:3-6`. The reviewer reruns every gate rather than trusting journals (M2R:14). Commits go through one shared account, so
  git authorship cannot show independence (M2R:5). The written assignment and the reviewer's statement are the record.
* **Suite and corpus authors.** Each gate names who writes its suite, corpus or fixtures, by role:
  * Assurance (Lain) writes the held-out checks (§7).
  * Every other independent suite is written by the **Dev Lead** or the **Auditor**, whichever is not a builder of that
    project.
  * The builder never authors a suite that grades their own work.
* **Every gate states:**
  * **Command.** The exact command, or the exact contract a command must satisfy. A contract is marked **frozen at requirements
    freeze** when its command can only be named once the requirements are frozen. No placeholder command is given.
  * **Evidence.** The artifact path, `milestone3/evidence/<gate-id>/`, which holds the command line, stdout/stderr, the exit
    code, the pins (§5 rule 9) and a `RESULT` file with the §5 outcome.
* **Freeze SHA.** For each project, the freeze commit's SHA is recorded in `milestone3/<eco|web>/requirements/FREEZE.md` by a
  follow-up commit that changes only that line. This mirrors the M2 pin-line pattern (M2D:383-391). `<freeze-sha>` below
  means that value.
* **Decision file.** The reviewer records a per-gate outcome table in `milestone3/<eco|web>/DECISION.md`.

### 1.2 P-ECO gates: receipt and grant-audit verifier (the top-ranked candidate, §3)

A HowlFrame program (`.howl` built to `.hfbc` and run on the standalone VM). It takes a `howlframe.receipt/v0` file
(`docs/cli.md:37-68`), the expected artifact SHA-256, and a declared grant policy. It reports, in machine-readable form,
whether the run stayed within that policy. **P-ECO is accepted on its own gates (E1-E10), with no dependency on P-WEB.**

**E1: Frozen contract.**
* **PASS when:**
  * `milestone3/eco/requirements/` defines the input formats (receipt v0 fields as in `docs/cli.md:42-61`; the policy format),
    every verdict and exit code, the stderr shape, and the verdict for `effects_truncated: true`.
  * It is committed before the first implementation commit, and `FREEZE.md` holds `<freeze-sha>`.
* **Command:** `git diff --name-status <freeze-sha> HEAD -- milestone3/eco/requirements milestone3/eco/tests` prints nothing.
* **Evidence:** `milestone3/evidence/E1/`
* **Suite author:** Auditor or Dev Lead (non-builder).

**E2: Built with HowlFrame, standalone.**
* **PASS when:** the only artifacts are `howlframe` and the `.hfbc`, with no generated Go/Python tool (same check as M2R:183).
* **Command:**
  * `go build -o howlframe .`
  * `./howlframe build milestone3/eco/src/<tool>.howl -o <tmp>/<tool>.hfbc`
  * `./howlframe run [--allow-caps …] <tmp>/<tool>.hfbc …`
  * The source name is frozen at requirements freeze.
* **Evidence:** `milestone3/evidence/E2/`
* **Suite author:** the reviewer.

**E3: Genuine receipts verify.**
* **PASS when:** on receipts the reviewer produces live, the tool reports `PASS` with exit 0, at 100%. The receipts come from
  one granted and one denied run of each M1 app (CLI `run --receipt`).
* **Command:**
  * Contract, frozen at requirements freeze: for each app, `howlframe run --receipt <tmp>/r.json …` with the documented grants,
    then the tool on that receipt plus the app's policy.
  * The script path and its sha256 are pinned.
* **Evidence:** `milestone3/evidence/E3/` (the receipts, policies and verdicts)
* **Suite author:** Auditor (policies) and the reviewer (live runs).

**E4: Deviations detected.**
* **PASS when:** on an independently authored corpus of altered receipts and policies, every case gets its frozen `FAIL` code
  and a nonzero exit, at 100%, with no false PASS. The corpus covers:
  * an artifact hash mismatch;
  * a grant missing from the policy;
  * a broader grant than the policy (for example coarse `network` vs `network:fetch=host:port`, as in M1
    `02-healthwatch/REVIEW.md:28`);
  * an allowed decision on a capability the policy denies;
  * the m9 grant-scope case, if the receipt carries it (§8);
  * `effects_truncated: true` (receipts cap effects at 10,000, `internal/vm/receipt.go:70`);
  * limits that differ from the policy;
  * a malformed or unknown-schema receipt.
* **Command:**
  * Contract, frozen at requirements freeze: one runner script iterates the corpus directory and compares each verdict and
    exit with the corpus's `expected` file.
  * The corpus and the script are sha256-pinned.
* **Evidence:** `milestone3/evidence/E4/`
* **Suite author:** **Auditor** (corpus), not the builder.

**E5: Honest claims.**
* **PASS when:** the README states that receipts have **no signature or attestation** (`docs/cli.md:66`), so the tool checks
  consistency against a policy and does not prove authenticity. No tamper-proof or attestation claim appears.
* **Command:** `rg -n -i 'tamper|attest|authentic|signed|proof' milestone3/eco/` returns only the disclaimer lines. The
  reviewer records each hit.
* **Evidence:** `milestone3/evidence/E5/`
* **Suite author:** the reviewer.

**E6: Capability discipline.**
* **PASS when:**
  * the tool runs with read-only filesystem grants for its inputs only;
  * each removed grant gives the frozen denial exit;
  * the tool needs no network, process or environment grant;
  * all of this is checked as in the M2 startup matrix (M2R:169-176).
* **Command:**
  * Contract, frozen at requirements freeze: a grant matrix script runs the tool once per dropped or extra grant and asserts
    the frozen exit codes and stderr.
  * The script is sha256-pinned.
* **Evidence:** `milestone3/evidence/E6/`
* **Suite author:** Auditor.

**E7: Clean environment.**
* **PASS when:**
  * the documented script exits 0 with all suites passing from a fresh `git clone --no-local`;
  * the README steps also work from a `git archive` copy with no `.git` (the m7 lesson, M2R:67, M2R:221).
* **Command:**
  * `env -i PATH=… GOCACHE=<empty> GOMODCACHE=<empty> GOPROXY=off GOTOOLCHAIN=local bash milestone3/eco/scripts/clean_repro.sh`
    (as in M2R:65-66);
  * then the README steps by hand in a `git archive <sha> | tar -x` copy.
* **Evidence:** `milestone3/evidence/E7/`
* **Suite author:** the reviewer.

**E8: Standalone usefulness on real records.**
* **PASS when:** following only the README, the reviewer audits the receipts from E3 against the policies written from each
  M1 app's documented grants. The reviewer gets a correct verdict for each and records the workflow, as M1 gate F did
  (M1D:44).
* **Command:**
  * the README workflow commands, verbatim;
  * the contract is frozen at requirements freeze.
* **Evidence:** `milestone3/evidence/E8/`
* **Suite author:** the reviewer.
* **Note:** use against P-WEB is **not** part of P-ECO; it is P-WEB's integration gate W8.

**E9: Held-out checks.**
* **PASS when:** every check in §7 (P-ECO) passes.
* **Command:** as written in §7.
* **Evidence:** `milestone3/evidence/E9/`
* **Suite author:** Assurance (Lain).

**E10: Independent review.**
* **PASS when:** a §1.1 reviewer records PASS on E1-E9 in `milestone3/eco/DECISION.md`.
* **Command:** n/a (the decision record).
* **Evidence:** `milestone3/eco/DECISION.md`
* **Suite author:** the reviewer.

#### 1.2.1 Intended external users of the verifier (hypothesis, not evidence)

* **No evidence on record** of anyone outside the HowlFutureWorks team (reviewers, Assurance, the EM) wanting or using a
  receipt verifier.
* **Hypothesis to validate:** an operator who runs other people's `.hfbc` artifacts under explicit grants would want a
  policy check of what the run actually did. That operator could be another AI tool calling HowlFrame directly (DIR:102), or
  a developer reviewing AI-generated changes (DIR:88).
* **The hypothesis does not gate M3.** It must not be stated as demand in the README or the decision record unless real
  usage is recorded later.

### 1.3 P-WEB gates: full-stack web application

The application domain is **selected or approved by an independent evaluator** and frozen before the build, as in M1
(`milestone1/SELECTION.md:3`; DIR:136). Extending ticketdesk with a real frontend is one allowed option. If it is chosen, the
§4 notes on m2/m3/m4 apply.

#### 1.3.1 Frontend exception: JS as compiler output only (requires Owner acknowledgement at spec acceptance)

The only HowlFrame web route generates JavaScript. `howlframe build` rejects a `web_app` root, and
`howlframe -o build <frontend>.howl` writes `build/app.js` as "a compatibility backend workflow, not an HFBC build or a browser
runtime" (`docs/cli.md:96-107`). The no-other-language rule (DIR:148, DIR:171) therefore gets a narrow exception for the
browser side only:
* **All app source is `.howl` or HFIR.** The backend is HFBC on the standalone VM (`howlframe run`).
* **The browser runs only unmodified HowlFrame compiler output:** `build/app.js` from `howlframe -o build <frontend>.howl`.
  The only other allowed file is one pinned, minimal static HTML loader whose only script is `<script src="app.js">`.
* **Hand-written or third-party JS app logic is a FAIL** (gate W13). Browser-harness files under `milestone3/web/tests/browser/`
  are test code, not app code, and are pinned separately.
* **The exception covers M3 P-WEB only.** It is recorded as an exception and does not reinterpret DIR:148.

#### 1.3.2 P-WEB entry criteria (all must be met and recorded before the W1 freeze)

**WE-1: m12 fixed and recorded (§4).**
* **PASS when:**
  * after the runtime fix, a manual (not CI) run reports `XPASS` for `m10_port_lost_race_stderr_shape`;
  * its `XFAIL` entry is then removed;
  * the rerun reports `5 tests, 0 failed` with no expected failures (M2R:318-323).
* **Command:**
  * `go build -o howlframe . && ./howlframe build milestone2/ticketdesk/src/ticketdesk.howl -o /tmp/ticketdesk.hfbc`
  * `HF_BIN=$PWD/howlframe APP_ARTIFACT=/tmp/ticketdesk.hfbc python3 milestone2/ticketdesk/tests/regression/assurance_m10.py milestone2/ticketdesk/tests/acceptance/launcher.example.sh`
    (M2R:345)
* **Evidence:** `milestone3/web/evidence/entry_m12/` (both runs, pins, `RESULT`)
* **Run by:** a non-builder of the fix (Auditor or Assurance).

**WE-2: Pinned browser harness.**
* **PASS when:**
  * A harness at `milestone3/web/tests/browser/` drives a **real Chrome** (the box has `google-chrome` 155 and `node` v20.19.2,
    with no driver yet) in headless mode with a real DOM.
  * The frozen requirements record its tool name, version, path, exact command and sha256.
  * One recorded smoke run on the box loads `examples/web_app_hello.howl`, built with `howlframe -o build`, and asserts a DOM
    observable.
* **Command:**
  * Contract, frozen at requirements freeze: one command starts the server and Chrome, runs the suite, and exits 0 only on all
    PASS.
  * It prints the Chrome and harness versions and the harness sha256.
* **Evidence:** `milestone3/web/evidence/entry_browser_harness/`
* **Author:** Dev Lead or Auditor (non-builder).
* **Runs manually on the box, not in CI.** CI has no browser step, and adding one needs `workflow` scope (§6, M2R:330-334).
* **Does not count as browser evidence:** the existing Node `vm` tests with stubbed browser globals (`internal/backend/javascript/browser_fetch_test.go`).

**WE-3: Server receipt coverage.**
* **PASS when:**
  * A recorded run shows a bytecode HTTP server writing a `howlframe.receipt/v0` on clean shutdown.
  * That receipt's effects count stays under the 10,000 cap (`internal/vm/receipt.go:70`) for the W8 workload, with
    `effects_truncated: false`.
  * Today, receipts are written when `RunBytecodeWithReceipt` returns (`internal/vm/receipt.go:143`), and there is no evidence
    on record of a shutdown receipt (§8).
* **If this can't be met:**
  * either a narrow, demonstrated-blocker runtime fix with a regression test (DIR:198) makes it work;
  * or the W1 freeze resizes W8 to a bounded workload (a single-process run that returns), recorded by revision before any W8
    run.
* **Command:**
  * Contract, frozen at requirements freeze: start the server with `--receipt`, drive the W8 workload, stop it cleanly, then
    `jq '.effects | length, .effects_truncated'` on the receipt.
* **Evidence:** `milestone3/web/evidence/entry_receipt/`
* **Run by:** Auditor.

#### 1.3.3 P-WEB gates

**W1: Frozen requirements and suites.**
* **PASS when:** `milestone3/web/requirements/` and the API, browser and XSS suites are committed before implementation, and
  `FREEZE.md` holds `<freeze-sha>`.
* **Command:** `git diff --name-status <freeze-sha> HEAD -- milestone3/web/requirements milestone3/web/tests` prints nothing (as
  in M2R:50).
* **Evidence:** `milestone3/evidence/W1/`
* **Suite author:** Dev Lead or Auditor (non-builder).

**W2: Backend API.**
* **PASS when:** the independent black-box API suite passes at 100% against `howlframe run <server>.hfbc`, started with the
  README command and **default runner ceilings**. The ceilings rule exists because the M2 suite once masked B1 with raised
  limits (M2D:756-758).
* **Command:**
  * Contract, frozen at requirements freeze: `python3 milestone3/web/tests/acceptance/run_acceptance.py <launcher>`, printing
    `N tests, N passed, 0 failed`.
  * The suite is sha256-pinned.
* **Evidence:** `milestone3/evidence/W2/`
* **Suite author:** Dev Lead or Auditor.

**W3: Meaningful frontend in a real browser.**
* **PASS when:**
  * the frontend is `.howl` built by `howlframe -o build <frontend>.howl` into `build/app.js` (§1.3.1), served by the P-WEB
    backend;
  * the WE-2 harness drives the frozen user workflows (create, read, update and delete through the UI, plus auth flows) and
    passes at 100%;
  * it runs manually on the box.
* **Command:** the WE-2 command against the P-WEB server.
* **Evidence:** `milestone3/evidence/W3/`
* **Suite author:** Dev Lead or Auditor.

**W4: Frontend output safety.**
* **PASS when:**
  * an XSS probe set renders as inert text in every user-editable field, and no script runs in the WE-2 browser. The set
    includes `<script>…`, `<img src=x onerror=…>` and attribute-break payloads;
  * a source check confirms that no untrusted value reaches `set_html` without `html_escape`. The basis for this gate is
    `docs/howlboard_xss_set_html_issue.md`.
* **Command:**
  * Contract, frozen at requirements freeze: the XSS suite runs on the WE-2 harness, plus `rg -n 'set_html' milestone3/web/src`
    with each hit justified in the evidence.
* **Evidence:** `milestone3/evidence/W4/`
* **Suite author:** **Auditor** (XSS set).

**W5: Auth enforced server-side.**
* **PASS when:**
  * every role and ownership rule in the frozen requirements is checked by direct API calls that bypass the UI (401/403 as
    specified);
  * no secret appears in server logs or stored state (the same scan as M2R:134-135);
  * if bearer auth is used, the frozen auth requirement explicitly excludes RFC 9110 trailing OWS (m2, §4).
* **Command:** the W2 suite's auth section, plus the secret scan script. Both are frozen at requirements freeze and
  sha256-pinned.
* **Evidence:** `milestone3/evidence/W5/`
* **Suite author:** Auditor.

**W6: Reliable errors.**
* **PASS when:** every non-2xx response from a request that reaches routing uses the frozen error envelope, and the UI shows a
  defined message for each one. The scope of this rule is set in the frozen requirements, using the m3/m17 lessons in §4.
* **Command:** the W2 validation section, plus a W3 error-display case.
* **Evidence:** `milestone3/evidence/W6/`
* **Suite author:** Dev Lead or Auditor.

**W7: Framework packaging.**
* **PASS when:**
  * all web-framework or UI-helper code is in a separately packaged module, not in `internal/` core;
  * any core change is a demonstrated-blocker fix with a journal entry and a regression test (DIR:99, DIR:185, DIR:198).
* **Command:** `git diff --stat <freeze-sha> HEAD -- internal/ howlframe.go`. Each listed file is mapped to a journal entry and
  a test.
* **Evidence:** `milestone3/evidence/W7/`
* **Suite author:** the reviewer.

**W8: Integration with P-ECO (consumes the accepted verifier).**
* **PASS when:**
  * the server runs with the minimum grants listed in the requirements, and each dropped grant gives the frozen denial exit;
  * the WE-3 receipt from the reviewer's W8 workload run is checked by the **accepted P-ECO tool** against the P-WEB grant
    policy, with `PASS` recorded.
* **Command:**
  * the grant matrix script (frozen at requirements freeze);
  * then the P-ECO tool on the WE-3 receipt plus `milestone3/web/requirements/policy.*`.
* **Evidence:** `milestone3/evidence/W8/`
* **Suite author:** Auditor.

**W9: Clean environment.**
* **PASS when:** the same as E7 (fresh clone and non-git copy), for `milestone3/web/scripts/clean_repro.sh`.
* **Command:** the E7 command, with the web script.
* **Evidence:** `milestone3/evidence/W9/`
* **Suite author:** the reviewer.

**W10: Startup contract.**
* **PASS when:** port-in-use (including the lost-race case, WE-1) and bad-config startups give the frozen exit code and a
  prefixed stderr line, with no raw runtime object.
* **Command:** a startup-matrix script, frozen at requirements freeze (shape as in M2R:169-179).
* **Evidence:** `milestone3/evidence/W10/`
* **Suite author:** Auditor.

**W11: Held-out checks.**
* **PASS when:** every check in §7 (P-WEB) passes.
* **Command:** as written in §7.
* **Evidence:** `milestone3/evidence/W11/`
* **Suite author:** Assurance (Lain).

**W12: Independent review.**
* **PASS when:** a §1.1 reviewer records PASS on W1-W11 and W13 in `milestone3/web/DECISION.md`, with known limitations
  documented (DIR:153).
* **Command:** n/a (the decision record).
* **Evidence:** `milestone3/web/DECISION.md`
* **Suite author:** the reviewer.

**W13: JS provenance (enforces §1.3.1).**
* **PASS when:**
  * in the clean repro, `build/app.js` regenerates from source **byte-identically** across two independent builds, and the
    `app.js` the server serves has the same sha256;
  * no other `.js` file is in the app tree;
  * the HTML loader matches its pinned sha256 and has exactly one `<script>` element, which has a `src`, no inline body and no
    `on*` attributes.
* **Command:**
  * `howlframe -o build <frontend>.howl` twice into separate empty directories, then `sha256sum` both and the served file;
  * `git ls-files milestone3/web | rg '\.js$'` lists only `milestone3/web/tests/browser/` files;
  * `sha256sum <loader>.html`, plus an `rg -c '<script'` count of 1.
* **Evidence:** `milestone3/evidence/W13/`
* **Suite author:** Auditor.

---

## 2. Order: P-ECO first, then P-WEB

**Decision: P-ECO (the receipt and grant-audit verifier) first, and accepted on its own gates. Then P-WEB, which consumes the
accepted verifier in its integration gate W8.**

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
3. **P-WEB enlarges the authority surface, and evidence quality decided M2.**
   * P-WEB adds a browser target, a server and more grants.
   * B6 took seven rounds and was never passed (M2D:14-50, M2D:440-446).
   * An accepted verifier gives P-WEB an objective capability gate (W8) instead of another round of hand probes. The
     dependency runs **one way only**: P-ECO's acceptance never waits on P-WEB (E8).
4. **P-ECO is bounded and has a proven shape.**
   * It is a standalone CLI on the standalone VM, which M1 showed works three times (M1D:5-11).
   * P-WEB needs a JS-only frontend route (`docs/cli.md:96-107`), a real-browser harness that does not exist yet (WE-2), and
     server receipts that are not shown yet (WE-3). That makes it the riskier project.

**What would reverse the order:** the P-ECO freeze review finds that receipt v0 cannot express the E4 checks without a
runtime change larger than a narrow fix (DIR:198). In that case P-WEB goes first, with W8 reduced by a recorded revision to
hand probes plus a captured receipt.

**Not parallel.** The directive does not require unconditional parallel work (DIR:185). P-WEB's entry criteria (§1.3.2) may
be worked on while P-ECO is under review, because they are preconditions, not P-WEB implementation. The milestone stop rule
is applied per project (DIR:162, DIR:220).

---

## 3. Ranked ecosystem-component candidates

Ranked by strength of M1/M2 evidence. The owner component follows DIR:92-100.

| Rank | Candidate (owner) | M1/M2 friction it addresses | Evidence strength |
|---|---|---|---|
| 1 | **Receipt and grant-audit verifier** (HowlFrame: capability and execution authority, tooling). Hange's candidate. | Hand-run H/authority checks (M1 REVIEW H rows; M1D:50). The m9 mutation is undetected by the test suite (M2R:223). Receipts were required by M1 ACCEPTANCE but none is on record (§2.2). Round 1 asked for stronger tests on `inherit:filesystem` and `network:fetch=` grants (M2D:778-779). Receipts are unsigned and have no checker (`docs/cli.md:66`). | **Strong** for the team's own friction. **No evidence on record** of users outside the team; that is a hypothesis to validate (§1.2.1). |
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
| **m2** | A bearer secret with one trailing space still authenticates. Go `net/http` strips trailing OWS (RFC 9110 §5.5) before the app sees it. No security impact (M2R:218, M2R:233-243). | **NOT** | It blocks neither project. If P-WEB uses bearer auth, W5 requires its frozen auth requirement to exclude RFC 9110 trailing OWS (fix option (b), M2R:241). |
| **m3** | Transport-level rejections (malformed `%zz`, a 431 for an oversize header, `OPTIONS *`) are plain text or empty and have no `X-Request-Id`. `net/http` answers before routing (M2R:219, M2R:245-264). | **NOT** | Only syntactically invalid HTTP is affected, and P-ECO is not a server. P-WEB's W6 scopes the uniform envelope to requests that reach routing (option (b), M2R:263). The guide and README must document it. |
| **m4** | Semantically corrupt state passes the shallow startup check, starts, then answers 500. This is app code (`ticketdesk.howl:49-58`) (M2R:220, M2R:297-308). | **NOT** | It is a ticketdesk application defect, not a runtime one. If P-WEB extends ticketdesk, deep validation on load becomes a P-WEB requirement and is not a precondition for starting. |
| **m12** | A port-probe race: if the port is taken between the probe (`vm.go:3532`) and `ListenAndServe` (`vm.go:3045-3051`), the raw `{"phase":"runtime","code":"IO_ERROR",…}` object reaches stderr. Deterministically reproduced (M2R:224, M2R:282-295). | **ENTRY CRITERION for P-WEB (WE-1)**, not for P-ECO | It is a HowlFrame runtime defect that any HowlFrame HTTP server inherits, including W10. The fix direction is to bind once and use `server.Serve(ln)` (M2R:292). The run is manual, not CI, and is recorded with its command at `milestone3/web/evidence/entry_m12/` by a non-builder (WE-1). |
| **m17** | When the declared Content-Length is not delivered, the server returns a 400 that is not in the uniform shape and has no `X-Request-Id` header, at `vm.go:2957-2963`. The upload is never executed (M2R:225, M2R:266-280). | **NOT** | Only clients that break HTTP framing see it, and browsers driving P-WEB do not. Fixing it needs an agreed error code (M2R:277-278). Record the choice (fix (a) or drop the connection (b)) in P-WEB's frozen requirements. |

Not in the list but relevant: **m9** (documented-accepted, a grant-coverage test gap, M2R:223). P-ECO's E4 corpus includes
the grant-scope case it describes, if the receipt carries it (§8).

---

## 5. Venue, outcome and noise rules (settled now, before any run)

These are based on what M2 actually did and learned.

1. **Deterministic functional gates run on the shared box.** This covers acceptance and browser suites, regressions, mutation
   checks and clean repro. M2's final review ran entirely there, on Linux with Go 1.27.2 and the Python 3 stdlib (M2R:11).
   Each gate gets one reviewer run plus one clean-repro run, with 0 failures required.

2. **Outcome classes for every gate run.** INFRA is a class of its own, separate from PASS and FAIL.
   * **PASS:** 0 failures and no noise breach.
   * **FAIL (product):** any failure that does not meet the INFRA test below. **Ambiguity counts as FAIL** (M2D:409).
   * **INFRA:** applies when either:
     * a failure whose request or step is overlapped by a recorded heartbeat gap of at least 1 s, or by a
       `virtual machine fork`/`virtio_balloon` kmsg record, or by a steal breach (the M2 classification, M2D:404-408); or
     * 0 failures with a noise breach (INFRA-VOID, M2D:411).
     * The infra cause must be documented in the gate's evidence (the heartbeat, kmsg and noise excerpts).
   * **Rerun budget: at most 1 rerun per gate, with the same pins, used only after a documented INFRA outcome.** A rerun can
     never turn a FAIL into a PASS. A FAIL is final for that pin (the M2 "no restart" rule, M2D:412-414). The rerun only
     replaces an INFRA run.
   * **A second INFRA** on the same venue makes the gate **BLOCKED-VENUE** (rule 6).

3. **Monotonic clock for all timing.**
   * Every timing bound is measured on a monotonic clock: Python `time.monotonic()`, or Go `time.Since` with its monotonic
     reading.
   * Wall time is also logged and disclosed, but is never used to classify or reclassify.
   * The M2 lesson: three B6 requests passed under the pinned monotonic 10 s deadline but would have failed on wall time
     across box freezes (M2D:42-45, M2D:157-160).
   * Timing bounds are at least 50× the measured worst case, following m10's 3 s bound against a measured 0.01-0.04 s
     (M2R:347-349).

4. **Heartbeat sidecar required** for every timing-sensitive run, including the W2/W3 suites, as in Round 7 (M2D:336-338):
   * a 50 ms loop that logs gaps over 250 ms with monotonic, realtime and boottime deltas;
   * plus the kmsg capture.
   Without it, a failure cannot be classified INFRA and counts as FAIL.

5. **Stress, durability or kill-storm gates are pre-registered before any code they test exists.**
   * The terms include the run count with its flake-bound arithmetic (M2: 0/75 lets a 4% flake pass about 5% of the time;
     0/225 puts the 95% upper bound under 1.4%, M2D:394-395, M2D:520).
   * They also include the Round 7 idle gate and noise gates verbatim (M2D:360-379) and the mechanical decision rule
     (M2D:397-411).
   * Such a gate may be dropped only by a dated revision **before its first run**, never waived after a result.

6. **BLOCKED-VENUE: an exit for an unavailable venue.**
   * **Why it exists:** the box is a ballooned, fork-prone VM. Two B6 runs there were INFRA-VOID (M2D:83-84, M2D:118-122), the
     planned PC run never happened (M2D:10-12, M2D:184-185), and an Owner waiver became the only way out (M2D:14-19).
   * **When it applies:**
     * a gate's second INFRA outcome; or
     * its pre-registered venue cannot be obtained.
   * **What is recorded:** BLOCKED-VENUE, with the venue evidence. The project's acceptance stays **pending**, never PASS.
   * **The pre-registered alternate venue,** named in the frozen requirements for any gate that can hit this:
     * William's PC, under the venue-evidence terms in M2D:178-180: bare metal vs VM/WSL2, balloon, sleep disabled, other
       workloads stopped, `/dev/kmsg` readable; or
     * another declared idle, non-ballooned machine.
   * **How the alternate is tried:** one attempt, under the same pins and the same outcome rules, including one INFRA rerun.
   * **If it is also unavailable or INFRA:** the gate stays BLOCKED-VENUE and the decision passes to the Owner as a recorded
     revision or waiver. An Owner waiver is therefore the last path, not the only one.

7. **On the shared box, a declared quiet window is required** for any timing-sensitive run: no other agent's builds, tests,
   storms or browser work (M2D:380-382). M2 had no artifact showing the box was idle (M2D:125-126), so M3 records one: a
   process listing at the start and end, kept in the gate's evidence.

8. **Toolchain recorded.** Each decision records the Go version, the Chrome and harness versions (P-WEB), the OS and the
   kernel. CI's Go 1.21 differs from the box (M2R:346), so a gate that CI is meant to cover must also pass on CI's version, or
   the gap is recorded.

9. **Pins and fixed inputs.**
   * The reviewer records sha256 or commit pins for the **code, every harness, corpus, fixture and script** used by a gate.
     The reviewer also shows that the reviewed tree differs from the code pin only in docs, or reruns (M2R:39-53).
   * Every randomized input uses a seed recorded in the frozen requirements (M2 used `20261019`, M2D:331).
   * Secrets are generated at runtime and never printed or written (M2R:69-71).

---

## 6. Out of scope

* New language features (paused by the Owner). The token benchmarks stay a regression guard only.
* The later competitive-validation program (DIR:249).
* A production HFIR or `-compile-bc` flip.
* OS-level sandboxing claims (DIR:244).
* Performance work beyond functional smoke checks.
* Desktop, mobile or game targets (DIR:189).
* Re-running M2's B6.
* Edits to `.github/workflows/` (the token lacks `workflow` scope, M2R:330-334). Wiring CI for the m10 tests or the browser
  harness is the Owner's call, so both stay manual-only.
* Any fixed number of endpoints, users or pass rates beyond the per-gate 100% rules above (DIR:185).

## 7. Held-out checks (Assurance; public part, check contents sealed)

**Author:** Assurance (Lain seat, `bot-assurance-0001`), a non-builder, 2026-10-20. Written against this spec at rev 2
(`f90d0093`). Product's text in §§1-6 and §8 is unchanged. Anything in it that is untestable or ambiguous is listed in §7.5
instead of being edited.

The full checks are **sealed**: ids, the requirement each maps to, exact inputs, pass conditions, category and mutation plan.
They are not in this repository, and no commit carries them. This section publishes only the protocol, a coverage map at the
level of behaviour, the hash commitments and the M2-carryover entry criteria. Builders must not see the sealed contents before
their project is scored.

### 7.1 Held-out protocol

1. **Sealed set v0 (now).** Assurance wrote `verifier.md` (P-ECO), `webapp.md` (P-WEB) and a `MANIFEST` of their sha256. They
   are kept outside the repository, in the Assurance agent store (`m3-heldout/`), with a working copy on the shared box. Their
   hashes are committed in §7.3 and in `milestone3/heldout/SEALED.sha256`.
2. **Sealed amendment v1 (per project).** The P-WEB domain and both frozen contracts do not exist yet: E1's policy format and
   FAIL codes, and W1's routes, roles, fields, session mechanism, messages, loader and policy. Some checks therefore refer to
   "the frozen code/route/field". Assurance writes a sealed v1 amendment that binds them to concrete values.
   * It is written after that project's requirements freeze and **before its first implementation commit**. Its hashes are
     committed in a dated revision of §7.3.
   * It may add checks or make inputs more concrete. It may never remove a check or weaken a pass condition. The v0→v1 diff is
     shown at the reveal.
   * Whether a conditional check applies (CSRF vs bearer, CSP, a robustness clause) is decided in v1 from the frozen
     requirements, never at scoring time.
3. **No builder access.** No builder of either project reads, writes or influences the sealed files. If the set leaks before
   scoring, it is void: Assurance writes a fresh set, records the incident and commits the new hashes before scoring.
4. **Scoring at a pin.** The §1.1 reviewer, a non-builder, scores at a stated pinned SHA and records the §5 rule 9 pins.
   * The reveal commit adds the sealed files under `milestone3/<eco|web>/heldout/`, together with a freeze test in the style of
     `experiments/token-efficiency-heldout-v3/spec_freeze_test.go`.
   * That test asserts that each file's sha256 equals the value committed here (or in the dated v1 revision).
   * The reviewer also runs `sha256sum -c` against `MANIFEST`. **Any hash mismatch voids the score.**
   * Evidence goes to `milestone3/evidence/E9/` (P-ECO) and `milestone3/evidence/W11/` (P-WEB), in the §1.1 layout with a
     `RESULT` file.
5. **Outcomes and reruns follow §5.** §5.2 sets the outcome classes, the single rerun allowed only after a documented INFRA,
   and BLOCKED-VENUE. A FAIL is never rerun to a PASS. Timing-bound checks use the §5.3 monotonic clock, the §5.4 heartbeat
   sidecar and the §5.7 quiet window. Browser checks run on the WE-2 harness, and the Chrome and harness versions are recorded.
6. **Mutation proof.** Every negative check must be shown to fail under a recorded mutation of the code under test. The sealed
   mutation plan names one or more mutations per check.
   * The proof records the file, the change and the failing excerpt, then reverts the change.
   * Before recording the decision, the reviewer shows that `git status --porcelain` is empty.
   * A negative check with no biting mutation does not count as passed.
7. **Recording.** `milestone3/<eco|web>/DECISION.md` lists every sealed check id with PASS/FAIL and its evidence. QA and
   Security are kept in separate sections, as in M2R §3.
8. **No secrets.** The sealed files use placeholders only. Test secrets are generated at runtime and never printed or written.

### 7.2 Coverage map (behaviour categories only, with counts of sealed checks)

**P-ECO, the receipt and grant-audit verifier** (ids `M3-E-H01` to `M3-E-H25`): 25 checks, 15 QA and 10 Security. None of them
depends on P-WEB (§1.2).

| Category | QA | Security | Gates |
|---|---|---|---|
| Genuine M1 receipts verify (granted, denied, large) | 3 | 0 | E3 |
| Real-record workflow from the README alone | 1 | 0 | E8 |
| Deviation detection: hash, missing grant, truncation, limits, malformed or unknown schema | 5 | 0 | E4 |
| Grant-scope bypass attempts (coarse vs scoped, path and port tricks, normalisation) | 0 | 3 | E4 |
| Detection of a decision the policy denies | 0 | 1 | E4 |
| Policy-file grammar | 0 | 1 | E4 |
| Output contract and determinism | 2 | 0 | E1 |
| Output injection safety | 0 | 1 | E1 |
| Input and startup errors (frozen exit, stderr shape) | 1 | 0 | E1, E6 |
| Hostile or oversized input (bounded, no hang) | 0 | 1 | E4 |
| Capability discipline and path traversal | 0 | 2 | E6 |
| Honest-claims scan, wider than the E5 command | 0 | 1 | E5 |
| Clean install (fresh clone; non-git copy) | 2 | 0 | E7 |
| DX: documented commands work verbatim | 1 | 0 | E1 |
| **Total** | **15** | **10** | |

**P-WEB, the full-stack web application** (ids `M3-W-H01` to `M3-W-H28`): 28 checks, 14 QA and 14 Security. Three Security
checks depend on the frozen W1 design (§7.1 step 2): one each for CSRF/same-origin/CORS, CSP and slow-client robustness.

| Category | QA | Security | Gates |
|---|---|---|---|
| Functional happy paths in the WE-2 browser (UI and API agree; repeatability) | 3 | 0 | W2, W3 |
| Error shapes (envelope, UI messages, transport-level and truncated-upload scope) | 3 | 0 | W6 |
| Input validation boundaries | 1 | 0 | W2, W6 |
| Durability and corrupt-state handling | 2 | 0 | W2, W10 |
| Startup contract, including the lost-race case | 1 | 0 | W10 |
| Clean install (fresh clone; non-git copy) | 1 | 0 | W9 |
| DX: documented commands and logs | 1 | 0 | W12 |
| Framework packaging | 1 | 0 | W7 |
| Capability evidence via a WE-3 receipt checked by the accepted P-ECO tool | 1 | 0 | W8 |
| Frontend output safety: XSS escaping, source audit, content type of echoed errors | 0 | 3 | W4, W6 |
| Served-asset provenance (only compiler-output JS; no inline script in any served HTML) | 0 | 1 | W13, W4 |
| Server-side authn/authz, including direct-API and header edge cases | 0 | 2 | W5 |
| Bypass via path aliasing (double slashes, dot segments, `%2F` variants) | 0 | 1 | W5, W6 |
| Secrets in served assets, logs, state and browser storage; static-file traversal | 0 | 2 | W5 |
| CSRF / same-origin / CORS (conditional) | 0 | 1 | W5 |
| CSP (conditional; only if W1 requires it) | 0 | 1 | W4 |
| Robustness against slow clients (conditional on a W1 clause) | 0 | 1 | W2 |
| Capability discipline (minimum grants) | 0 | 1 | W8 |
| Integration negative: altered P-WEB receipts must FAIL in the accepted P-ECO tool | 0 | 1 | W8 |
| **Total** | **14** | **14** | |

### 7.3 Hash commitments (sealed-v0, against rev 2 `f90d0093`)

| File | sha256 |
|---|---|
| `verifier.md` | `c0d989952b2b200805e07845b1d8e8f1197c077fc07757ca1c264893ca1502ea` |
| `webapp.md` | `2c289dbba6d6e0208c77c55aa84bd9894748858baf458fea47c906bcea78b2cc` |
| `MANIFEST` | `a45bbb88b1bb18ef8a0ad8c25806cb5c318c22fc46f2dcb4ffc050b0127d6857` |

The same values are in `milestone3/heldout/SEALED.sha256`. An earlier draft, written against `c85cf2fa`, was never committed or
pushed and is superseded.

### 7.4 Entry criteria for the M2-waived items (Assurance recommendation)

This table is Assurance's recommendation. Where it differs from Product's §4 table, the difference is listed for decision in
§7.5 (OQ-1), and §4 is left as written.
* "ENTRY" means fixed, or resolved in the frozen requirements, before the W1 freeze (the §1.3.2 timing).
* "EXIT" means a gate checked at that project's final review.
* P-ECO is not an HTTP server, so m2, m3, m12 and m17 do not apply to it.

| Id | Requirement (M2) | P-ECO | P-WEB | One-line reason | Verifying check |
|---|---|---|---|---|---|
| **m2** | bearer trailing OWS (R9) | n/a | **ENTRY at the requirements level if P-WEB uses bearer auth; otherwise n/a.** This matches W5 in effect. | The frozen W1 auth rule must state the RFC 9110 exclusion (or require a runtime fix) before the build, because a silent deviation cannot be scored later. | `M3-W-H22` |
| **m3** | transport-level error shape (R8.1/R3.3) | n/a | **EXIT**. §4 says NOT. | W6 scopes the envelope to requests that reach routing. The frontend must encode user input so the UI never triggers m3, and must show a defined message if it happens anyway. No runtime fix is a precondition. | `M3-W-H04` (with `M3-W-H03`) |
| **m17** | truncated-upload 400 shape (R8.1/R3.3) | n/a | **ENTRY**. §4 says NOT. | A real but slow browser upload that runs past the runtime's 30 s ReadTimeout gets the off-envelope 400 (a stalled upload was answered at 30.0 s, M2R §4). So the premise "browsers do not see it" does not hold. The runtime fix, or the frozen "drop the connection" choice, must exist before the UI error mapping is built. | `M3-W-H05` |
| **m4** | semantic state validation (R1.3) | n/a (malformed inputs are covered by E4: `M3-E-H07`, `M3-E-H24`) | **ENTRY if P-WEB extends ticketdesk; otherwise EXIT**. §4 says NOT. | Extending ticketdesk builds directly on its shallow loader. A new app must meet the frozen corrupt-state rule at review. | `M3-W-H08` |
| **m12** | port-probe race raw error (R1.3) | n/a | **ENTRY**. This agrees with §4 and WE-1. | It is a runtime defect that every HowlFrame server inherits, and W10 names the lost-race case. It is verified when `assurance_m10.py` reports XPASS, the XFAIL entry is removed, and the suite reruns clean (WE-1). | WE-1; `M3-W-H09` |

### 7.5 Open questions for Product

* **OQ-1 (§4 vs §7.4).** Assurance recommends m17 as ENTRY for P-WEB; §4 says NOT. If adopted, it would become a WE-4 under
  §1.3.2. Assurance also recommends m3 as EXIT with W6 scoping, and m4 as ENTRY when extending ticketdesk. Which table governs?
* **OQ-2 (E3, denied runs).** What verdict does a receipt with a `denied` decision get when the policy equals the run's grants?
  A denial is within policy, but the spec does not say so.
* **OQ-3 (E4, limits).** For "limits that differ from the policy": is any difference a FAIL, or only a looser limit (for example
  a higher `max_instructions`)?
* **OQ-4 (E4, grant semantics).** E4 needs a frozen subsumption rule for scoped grants: path-aware filesystem roots (trailing
  slash, `..`, prefix-sibling paths), `network` vs `network:fetch=host[:port]`, `inherit:filesystem`, and duplicate or unsorted
  `grant[]`. Without one, "broader grant" cannot be decided.
* **OQ-5 (E5 command scope).** `rg -n -i 'tamper|attest|authentic|signed|proof' milestone3/eco/` also scans
  `milestone3/eco/tests` and `requirements`. The E4 corpus of altered receipts may legitimately be named "tampered…". Do such
  hits count as "only the disclaimer lines", or should the scan be limited to README, docs and source?
* **OQ-6 (W13 loader check).** `rg -c '<script'` counts matching lines, not elements, and is case-sensitive. A `<SCRIPT>` tag or
  two tags on one line passes, and a `<script` inside a comment fails. Should the check parse the HTML instead, or add `-i`
  plus a one-element-per-line rule?
* **OQ-7 (WE-2/W9 dependencies).** The WE-2 harness needs a Chrome driver: CDP over node, or a library. Is a vendored or
  npm-installed driver allowed under W9's clean environment (`GOPROXY=off`, no network)? Should W9 run the browser suites at
  all, or only the API suite?
* **OQ-8 (WE-3 vs W8 resizing).** If WE-3 falls back to "a bounded workload (a single-process run that returns)", does W8 still
  run the server on HFBC with `howlframe run`? Which workload does the frozen requirement name? M3-W-H14 uses a different
  reviewer-chosen workload of the same kind.
* **OQ-W1 (W4).** Is `set_html` allowed at all, or should W4 prefer `set_text`? URL contexts (`href`, `src`) are not covered by
  `html_escape`. Does W1 freeze a scheme allow-list?
* **OQ-W2 (W5).** The session mechanism (a cookie, or a bearer token held in JS) is unspecified. It decides whether CSRF and
  SameSite apply, and how XSS would expose tokens. Please freeze it in W1.
* **OQ-W3 (W4).** No part of the spec requires a CSP. Is one intended? If not, the conditional CSP check stays inactive.
* **OQ-W4 (W2).** P-WEB has no robustness or slow-client clause (M2 had R17, and the B3 lesson). Is one intended? If not, the
  conditional robustness check stays inactive.
* **OQ-W5 (W6).** "The UI shows a defined message for each one" needs a frozen code-to-message table in W1.
* **OQ-9 (§5.3).** "At least 50× the measured worst case" depends on a measurement that can only exist after implementation.
  Which measurement, and when is the bound frozen?
* **OQ-10 (sealing venue).** The shared box is readable by every agent, including future builders, so the box copy of the
  sealed set is weaker than the agent-store copy. Should the box copy be removed once the hashes are committed?

## 8. Evidence gaps (stated, not guessed)

* **The web target is JS-only.**
  * `howlframe build` rejects a `web_app` root. The route is `howlframe -o build` into `build/app.js`, "a compatibility backend
    workflow, not an HFBC build or a browser runtime" (`docs/cli.md:96-107`).
  * There is no full web app on record, only `examples/web_app_hello.howl` and the dogfood journals.
  * **Handled by:** the §1.3.1 exception (Owner acknowledgement) and W13.
* **There is no browser harness.**
  * The repo has no Playwright, Puppeteer or Selenium, and no `package.json`.
  * `internal/backend/javascript/browser_fetch_test.go` runs generated JS in Node's `vm` with stubbed globals and no real DOM, so it is not browser
    evidence.
  * CI has no browser step.
  * **Handled by:** entry criterion WE-2 (manual on the box).
* **Server receipts are written at return and capped at 10,000 effects.**
  * `internal/vm/receipt.go:143` writes the receipt when `RunBytecodeWithReceipt` returns, and `:70` caps effects at
    `maxReceiptEffects = 10000`, then sets `effects_truncated`.
  * There is no evidence on record of a receipt from a server at shutdown, and a long-running server would probably hit the
    cap.
  * **Handled by:** entry criterion WE-3, before P-WEB's freeze. The E4 `effects_truncated` verdict is fixed in E1. W8 uses
    only a WE-3-qualified receipt.
* **Users of the verifier outside the team:** no evidence on record. It is a hypothesis (§1.2.1).
* **Whether m9's mutation shows up in a receipt:** not established. E4 includes the case only if the frozen contract shows the
  receipt carries it.

## Sources

* `HOWLFRAME_PRODUCT_DIRECTIVE.md` §§3.4, 4, 5, 6 (M3: lines 178-185), 7.2-7.3, 8.
* `milestone1/MILESTONE1_DECISION.md`; `milestone1/SELECTION.md`; `milestone1/0{1,2,3}-*/REVIEW.md`;
  `milestone1/0{1,2,3}-*/requirements/ACCEPTANCE.md`.
* `milestone2/PLAN.md`; `milestone2/MILESTONE2_DECISION.md` (acceptance `d03c1176`; Round 7 record #201 `e41b6830`;
  isolation rerun record #202 `883f26de`; pre-registration terms); `milestone2/ASSURANCE_M2_FINAL_REVIEW.md` (#203, `dd8f2728`);
  `milestone2/ticketdesk/README.md` "Known limitations".
* `docs/cli.md` (`--receipt`, lines 37-68; web route, lines 96-107); `internal/vm/receipt.go` (lines 70, 143);
  `docs/howlboard_xss_set_html_issue.md`; `docs/journals/2026-08-14_dogfood_runtime_integrity_gate.md`.
* Motoko's review of `c85cf2fa`: #205 review `5587744378`.
