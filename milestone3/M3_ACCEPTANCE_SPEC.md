# Milestone 3 acceptance spec: full-stack web app and a genuine Howl ecosystem component

**Status:** FROZEN when #206 merges to `main`: `e6fdd48a` + the pre-merge revisions R1–R7 (`milestone3/FREEZE.md` §1 and §4;
R1 is superseded by R4). Each revision is listed in `milestone3/OWNER_ACK_2026-10-20.md` §7 with its ack status. From that
merge on, the revision rule below applies.
* It addresses Motoko's reviews of `c85cf2fa`
  ([#205 review 5587744378](https://github.com/howlcipher/howlframe/pull/205#pullrequestreview-5587744378)) and `baeb0d03`
  ([#205 review 5587896730](https://github.com/howlcipher/howlframe/pull/205#pullrequestreview-5587896730)).
* The spec freezes once the independent review accepts it **and** the Owner decision below is recorded in each project's
  `FREEZE.md` (§1.1.1). A freeze without that record is not valid.
* From the #206 merge on, changes need a separately recorded, dated revision (directive §8, "Record decisions without
  retroactive goalpost changes"). **Revision rule** (`milestone3/FREEZE.md` §4): every change after that merge is a dated
  revision row with the spec sha256 after, carries an Owner ack recorded in an `OWNER_ACK_<date>.md` file, and is never made
  after any result of a gate that the revision affects is known. R1–R7 are pre-merge content, not post-freeze revisions.
  * **Owner-reserved decisions** *(R5; from EngMgr's determination, 2026-10-20 16:37 ET, recorded in `milestone3/FREEZE.md` §4)*. A revision that relaxes a gate, or
    that touches any of the following, needs its own Owner ack:
    * waivers, including any `WAIVED, NOT PASSED` record;
    * the W3/W13 JS exception;
    * manual, non-CI runs;
    * the entry criteria;
    * the venue (§5 rule 6).
**Base:** `main` at `d03c1176` (Milestone 2 accepted by the Owner, `milestone2/MILESTONE2_DECISION.md:3-12`).
**Authority:** `HOWLFRAME_PRODUCT_DIRECTIVE.md` §6, Milestone 3 (lines 178-185), and nothing more. The directive sets **no
extra numeric M3 gates, no required component, and no mandatory parallel execution** (line 185). This spec adds none. Every
gate below is a pass/fail observable chosen for that project.
**Author:** Product (requirements and acceptance only; Product builds none of it).
**Owner decision on the acknowledgement items.** Recorded 2026-10-20 15:23 ET. Source: relayed by EngMgr (Rintaro Okabe).
William Elias (Owner) selected the option "Approve, but my PC isn't a backup venue"; as recorded in chat: "Approve items 1,
2 and 4; drop my PC as the alternate venue". Record: `milestone3/OWNER_ACK_2026-10-20.md`.
1. **APPROVED: W3/W13 frontend exception (§1.3.1).** Browser-side JS is allowed only as unmodified HowlFrame compiler output.
   This is a narrow, recorded exception to DIR:148/DIR:171.
2. **APPROVED: manual-only, non-CI runs** of WE-1, WE-4, WE-5 and the WE-2 browser suites (W3, W4, W9). CI cannot gain a browser
   or m10 step without `workflow` scope (M2R:330-334), and wiring that is the Owner's call (§6).
3. **DECLINED and removed: an alternate venue on the Owner's own machine.** No alternate venue is pre-registered (§5 rule 6).
   On 2026-10-20 at 16:32 ET the Owner also dropped the Owner-named-venue path (R4, `milestone3/OWNER_ACK_2026-10-20.md`):
   no venue can be added later.
4. **APPROVED: runtime fixes made as entry criteria** (WE-1 m12, WE-4 m17, and WE-5 m2 if bearer auth is used). Each is a
   narrow demonstrated-blocker fix under DIR:198.

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

#### 1.1.1 `FREEZE.md` template (one per project; the freeze is not valid without every field)

```text
Freeze SHA:            <sha of the freeze commit>            # set by a follow-up commit changing only this line
Code pin:              <howlframe commit sha>
Compiler:              ./howlframe built from the code pin; `howlframe version` = <output>; sha256 = <hex>
Toolchain pins:        Go <ver>; OS/kernel <uname -sr>; CPU <model, count>; Chrome <ver> (P-WEB); driver <tool ver, sha256> (P-WEB)
Harness/corpus pins:   <path> sha256 <hex>   (one line each: suites, E4 corpus, E5_ALLOWLIST, loader.html, scripts)
Seeds:                 <name>=<value>        (one line each)
W8 bounded N:          <N, or "n/a: WE-3 met">                  # P-WEB only, set before the freeze
Timing bounds:         <gate>: <absolute monotonic bound>   (one line each; OQ-9)
Alternate venue:       none; no venue can be added (§5 rule 6, R4)
Owner ack:             who=William Elias (Owner)  date=2026-10-20 15:23 ET  source=relayed by EngMgr (Rintaro Okabe)
                       option="Approve, but my PC isn't a backup venue"  chat record="Approve items 1, 2 and 4; drop my PC as the alternate venue"
                       items: 1 APPROVED (W3/W13 JS exception); 2 APPROVED (manual non-CI runs); 3 DECLINED (Owner's machine as alternate venue; removed); 4 APPROVED (runtime fixes as entry criteria)
                       link=milestone3/OWNER_ACK_2026-10-20.md
```

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
* **PASS when:**
  * the README states that receipts have **no signature or attestation** (`docs/cli.md:66`), so the tool checks consistency
    against a policy and does not prove authenticity;
  * no tamper-proof or attestation claim appears anywhere in `milestone3/eco/`, apart from fixture files on a pinned allowlist.
* **Allowlist (OQ-5):**
  * `milestone3/eco/tests/fixtures/E5_ALLOWLIST` lists fixture paths, one per line. Every entry must sit under
    `milestone3/eco/tests/fixtures/`, and the file's sha256 is recorded in `FREEZE.md`.
  * Only allowlisted files are skipped. A file named `*tampered*` (or any other hit) outside the allowlist still fails the
    gate, unless the hit is one of the README disclaimer lines.
* **Command:**
  * `rg -n -i 'tamper|attest|authentic|signed|proof' milestone3/eco/ $(sed 's/^/--glob=!/' milestone3/eco/tests/fixtures/E5_ALLOWLIST)`
    returns only the disclaimer lines;
  * plus `sha256sum milestone3/eco/tests/fixtures/E5_ALLOWLIST`, which must match `FREEZE.md`.
  * The reviewer records every hit, and confirms that a control file `milestone3/eco/tampered_probe.txt` created outside the
    allowlist is reported (then deletes it).
* **Evidence:** `milestone3/evidence/E5/`
* **Suite author:** Auditor (allowlist); the reviewer runs the scan.

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
* **Driver allowed (OQ-7):** a pinned headless-Chrome driver (CDP over Node, or a driver library) may be part of the harness.
  * Its tool name, version and sha256 are recorded in the frozen requirements.
  * It is vendored, or provisioned from a hash-verified local copy, so it needs no network at run time.
  * It is **test tooling only**: it is not an app runtime dependency and must never be served or loaded by the app.
  * Chrome itself is host-provided like Go, with its version recorded (§5 rule 8).
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
  * or W8 falls back to a bounded run (OQ-8). The server is still HFBC under `howlframe run`, and it runs the **same frozen
    W8 workload, truncated to a pre-registered request count N**. N is chosen so the receipt stays under the 10,000-effect
    cap with margin (at most half the cap at the measured effects per request). N is recorded in `FREEZE.md` before the
    W1 freeze, and is never chosen after the fact.
* **Command:**
  * Contract, frozen at requirements freeze: start the server with `--receipt`, drive the W8 workload, stop it cleanly, then
    `jq '.effects | length, .effects_truncated'` on the receipt.
* **Evidence:** `milestone3/web/evidence/entry_receipt/`
* **Run by:** Auditor.

**WE-4: m17 fixed or resolved (§4, aligned with §7.4).**
* **PASS when:**
  * A slow, real-browser-style upload that runs past the runtime's 30 s read timeout gets either the uniform error envelope
    with an `X-Request-Id` header, or a dropped connection with no response. Which of the two is chosen is frozen in the W1
    requirements.
  * Today it gets the off-envelope 400: a stalled upload is answered at 30.0 s (M2R:225, M2R:270-273).
  * The upload is never executed.
* **Command:**
  * Contract, frozen at requirements freeze: a stdlib Python client sends a declared Content-Length, delivers part of the
    body, then stalls past 30 s;
  * it asserts the frozen response (or a dropped connection) and that state is unchanged.
  * It runs against ticketdesk at the fix pin, and against the P-WEB server once it exists.
* **Evidence:** `milestone3/web/evidence/entry_m17/` (command, output, pins, `RESULT`)
* **Run by:** Auditor (a non-builder of the fix). The run is manual, not CI.

**WE-5: m2 fixed, if P-WEB uses bearer auth (§4).**
* **PASS when:**
  * if bearer auth is used, `Authorization: Bearer <secret> ` (one trailing space) gives 401 with `WWW-Authenticate`, as
    M2 R9 requires (M2R:234-236);
  * this is shown by a **verified runtime fix before entry** (fix option (a), M2R:239-240), not only by a requirements
    exclusion;
  * if P-WEB uses no bearer auth, this criterion is n/a and the reason is recorded.
* **Command:**
  * Contract, frozen at requirements freeze: a raw-socket probe (so the client does not normalise the header) with trailing
    space, trailing tab, and the control case without them, asserting 401, 401 and 200.
* **Evidence:** `milestone3/web/evidence/entry_m2/`
* **Run by:** Auditor (a non-builder of the fix). The run is manual.

**WE-6: m4 resolved, if P-WEB extends ticketdesk (§4).**
* **PASS when:** if P-WEB extends ticketdesk, the hand-edited corrupt states from M2R:220 (for example bucket `{"ids":"x"}`,
  `title: 7`, `meta.next_ticket: -5`) each exit 65 at startup with a prefixed stderr line, before the W1 freeze. If P-WEB
  is a new app, m4 is an exit check instead (W14).
* **Command:** contract, frozen at requirements freeze: a corrupt-state startup matrix script asserting exit 65 and the stderr
  shape for each state.
* **Evidence:** `milestone3/web/evidence/entry_m4/`
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
* **Slow-client robustness clause (OQ-W4):** W1 includes an M2 R17-equivalent clause. Slow-header and slow-body clients must
  not delay other requests past the frozen bound, and a cut-off upload is never executed. This is the B3 lesson (M2D:760-761)
  and is shaped like `m10_slow_body_clients_bounded` (M2R:357).
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
  * **`set_text` is required for any value derived from user or server data.** `set_html` is allowed only with compile-time
    literal markup (OQ-W1). The basis for this gate is `docs/howlboard_xss_set_html_issue.md`;
  * every URL context (`href`, `src`) built from data passes a scheme allow-list frozen in W1 (relative, `http:`, `https:`
    only), so `javascript:` and `data:` are rejected;
  * every served HTML response carries the CSP frozen in W1, at least
    `default-src 'self'; script-src 'self'; object-src 'none'; base-uri 'none'`, and the browser suite shows no CSP
    violations in the normal workflows (OQ-W3).
* **Command:**
  * Contract, frozen at requirements freeze: the XSS and URL-scheme suites run on the WE-2 harness;
  * `rg -n 'set_html' milestone3/web/src`, with every hit shown to have a literal argument;
  * a header check of the CSP on every HTML route.
* **Evidence:** `milestone3/evidence/W4/`
* **Suite author:** **Auditor** (XSS set).

**W5: Auth enforced server-side.**
* **PASS when:**
  * every role and ownership rule in the frozen requirements is checked by direct API calls that bypass the UI (401/403 as
    specified);
  * no secret appears in server logs or stored state (the same scan as M2R:134-135);
  * if bearer auth is used, trailing whitespace after the secret gives 401, with m2 fixed and verified at entry (WE-5, §4).
    No requirements exclusion of RFC 9110 trailing OWS is allowed;
  * W1 freezes **one** session mechanism, a cookie or a bearer token held in JS (OQ-W2). Whichever is chosen:
    * a CSRF/same-origin check is active: state-changing requests from a foreign origin are rejected, and no permissive CORS is
      allowed;
    * a session cookie is `HttpOnly` and `SameSite=Strict`;
    * no token is stored in `localStorage` or `sessionStorage`.
* **Command:** the W2 suite's auth section, plus the secret scan script. Both are frozen at requirements freeze and
  sha256-pinned.
* **Evidence:** `milestone3/evidence/W5/`
* **Suite author:** Auditor.

**W6: Reliable errors.**
* **PASS when:** every non-2xx response from a request that reaches routing uses the frozen error envelope, and the UI shows a
  defined message for each one, taken from a **code-to-message table frozen in W1** (OQ-W5). The scope of this rule is set in
  the frozen requirements, using the m3/m17 lessons in §4.
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
    policy, with `PASS` recorded;
  * the server is HFBC under `howlframe run` in every case. Under the WE-3 fallback, the workload is the same frozen W8
    workload truncated to the pre-registered N in `FREEZE.md` (OQ-8);
  * any receipt used by W8, or by a held-out W8 check, with `effects_truncated: true` counts as FAIL, never as PASS.
* **What "accepted P-ECO tool" means** *(R7; Owner decision 2026-10-20 16:51 ET, `milestone3/OWNER_ACK_2026-10-20.md` §6)*:
  every E-gate is either PASSED or Owner-waived.
  * An Owner-waived gate is one recorded `WAIVED, NOT PASSED` with a dated Owner decision. It counts toward the accepted
    P-ECO that W8 relies on, with **no second Owner decision**.
  * An E-gate that is neither PASSED nor Owner-waived (FAIL, INFRA, BLOCKED-VENUE without a waiver, or not yet run)
    **blocks W8**.
* **Command:**
  * the grant matrix script (frozen at requirements freeze);
  * then the P-ECO tool on the WE-3 receipt plus `milestone3/web/requirements/policy.*`.
* **Evidence:** `milestone3/evidence/W8/`
* **Suite author:** Auditor.

**W9: Clean environment.**
* **PASS when:** the same as E7 (fresh clone and non-git copy), for `milestone3/web/scripts/clean_repro.sh`. The script runs
  the API suite **and** the WE-2 browser suites.
* **Pinned driver (OQ-7):** the pinned headless-Chrome driver from WE-2 is allowed in this clean environment. It is test
  tooling only, with its tool, version and sha256 recorded, and it is never an app runtime dependency. It must come from a
  vendored or hash-verified local copy, because `GOPROXY=off` and no network fetch is allowed during W9.
* **Command:** the E7 command, with the web script. The output records the driver and Chrome versions and the driver sha256.
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
* **PASS when:** a §1.1 reviewer records PASS on W1-W11, W13 and W14, plus WE-1 to WE-6, in `milestone3/web/DECISION.md`, with known limitations
  documented (DIR:153).
* **Command:** n/a (the decision record).
* **Evidence:** `milestone3/web/DECISION.md`
* **Suite author:** the reviewer.

**W13: JS provenance (enforces §1.3.1).**
* **PASS when:**
  * in the clean repro, `build/app.js` regenerates from source **byte-identically** across two independent builds, and the
    `app.js` the server serves has the same sha256;
  * the compiler for both builds is `./howlframe` built from the recorded code pin, and its `howlframe version` output and
    binary sha256 match `FREEZE.md` (§1.1.1);
  * no other `.js` file is in the app tree;
  * the HTML loader matches its pinned sha256, **and a pinned HTML parser** shows all of the following (OQ-6, superseding
    the earlier `rg` count):
    * exactly **one** `script` element (tag names matched case-insensitively), with `src="app.js"` and no inline content;
    * no other executable element: no other `script`, no `iframe`, `object`, `embed`, `base`, or `<meta http-equiv="refresh">`;
    * no `on*` event-handler attribute on any element;
    * no attribute value with a `javascript:` (or `data:`/`vbscript:`) URL, after trimming and case-folding.
* **Command:**
  * `howlframe -o build <frontend>.howl` twice into separate empty directories, then `sha256sum` both and the served file;
  * `sha256sum ./howlframe` and `./howlframe version`, compared with `FREEZE.md`;
  * `git ls-files milestone3/web | rg '\.js$'` lists only `milestone3/web/tests/browser/` files;
  * `sha256sum <loader>.html` must match the pin;
  * `python3 milestone3/web/tests/loader_check.py <loader>.html` exits 0. The script uses the Python stdlib `html.parser`, and
    the Python version plus the script's sha256 are recorded in the pins. Its exact assertions are the list above, frozen at
    requirements freeze.
* **Evidence:** `milestone3/evidence/W13/`
* **Suite author:** Auditor.

**W14: M2-carryover exit checks (§4, aligned with §7.4).**
* **PASS when:**
  * **m3 (always):** requests rejected at the transport level are documented in the guide and README. The frontend encodes
    all user input, so the normal UI workflows never produce them. If one happens anyway (malformed `%zz` path, oversize
    header, `OPTIONS *`, M2R:219), the UI shows the defined message from the W6 table. This is verified at P-WEB
    acceptance, not before entry.
  * **m4 (only if P-WEB is a new app, not a ticketdesk extension):** the frozen corrupt-state rule holds at review, meaning
    semantically corrupt state exits 65 at startup. For a ticketdesk extension, m4 is WE-6 instead.
* **Command:**
  * Contract, frozen at requirements freeze: raw-socket transport probes plus a WE-2 UI case for m3;
  * the WE-6 corrupt-state matrix for m4.
* **Evidence:** `milestone3/evidence/W14/`
* **Suite author:** Auditor.

---

## 2. Order: P-ECO first, then P-WEB

**Decision: P-ECO (the receipt and grant-audit verifier) first, and accepted on its own gates. Then P-WEB, which consumes the
accepted verifier in its integration gate W8.** "Accepted" here means every E-gate is PASSED or Owner-waived
(`WAIVED, NOT PASSED` with a dated Owner decision). A gate that is neither blocks W8 (W8, R7, Owner decision 16:51 ET).

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

## 4. M2-waived items: entry and exit criteria (aligned with §7.4, Lain's OQ-1)

All five were waived **for M2 only** (M2R:29-32, M2D:61-62). They do not carry over automatically. This table **adopts
Assurance's §7.4 classification** (Product's answer to OQ-1, §9), with one stricter change: for m2, entry requires a
verified fix, not only a requirements exclusion. P-ECO is not an HTTP server, so none of the five apply to it.
* "ENTRY" means fixed and recorded before the W1 freeze (§1.3.2).
* "EXIT" means checked at P-WEB's final review.

| Id | What it is (from M2R §4) | P-WEB classification | Reason | Criterion / gate |
|---|---|---|---|---|
| **m2** | A bearer secret with one trailing space still authenticates. Go `net/http` strips trailing OWS (RFC 9110 §5.5) before the app sees it (M2R:218, M2R:233-243). | **ENTRY if P-WEB uses bearer auth** (a fix verified before entry); otherwise n/a | A silent auth deviation cannot be scored later (§7.4). The stricter reading needs the M2 R9 behaviour to actually hold, not a requirements exclusion. | WE-5; W5 |
| **m3** | Transport-level rejections (malformed `%zz`, a 431 for an oversize header, `OPTIONS *`) are plain text or empty and have no `X-Request-Id`. `net/http` answers before routing (M2R:219, M2R:245-264). | **EXIT** | W6 scopes the envelope to routed requests. The UI must never trigger m3 in normal use and must show a defined message if it happens anyway. No runtime fix is a precondition (§7.4). | W14 (and W6) |
| **m4** | Semantically corrupt state passes the shallow startup check, starts, then answers 500. This is app code (`ticketdesk.howl:49-58`) (M2R:220, M2R:297-308). | **ENTRY if P-WEB extends ticketdesk; otherwise EXIT** | Extending ticketdesk builds directly on its shallow loader. A new app must meet the frozen corrupt-state rule at review (§7.4). | WE-6, or W14 |
| **m12** | A port-probe race: if the port is taken between the probe (`vm.go:3532`) and `ListenAndServe` (`vm.go:3045-3051`), the raw `{"phase":"runtime","code":"IO_ERROR",…}` object reaches stderr. Deterministically reproduced (M2R:224, M2R:282-295). | **ENTRY** | It is a runtime defect that every HowlFrame server inherits (W10). It is verified by XPASS, then removal of the XFAIL entry, then a clean rerun, recorded manually at `milestone3/web/evidence/entry_m12/`. | WE-1; W10 |
| **m17** | When the declared Content-Length is not delivered, the server returns a 400 that is not in the uniform shape and has no `X-Request-Id` header, at `vm.go:2957-2963`. The upload is never executed (M2R:225, M2R:266-280). | **ENTRY** | A real but slow browser upload past the 30 s read timeout gets the off-envelope 400 (a stalled upload was answered at 30.0 s, M2R:225), so a real frontend can trigger it. The fix, or the frozen drop-connection choice, must exist before the UI error mapping is built (§7.4). | WE-4; W6 |

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
   * **INFRA:** exactly M2's Round 7 rule (M2D:404-406, M2D:411). It applies only when one of these holds:
     * for a failure: heartbeat gaps **cover at least 80% of the failing request's (or step's) monotonic elapsed time**; or a
       recorded `virtual machine fork`/`virtio_balloon` kmsg record or a steal breach **overlaps** that request or step;
     * for a run with 0 failures and a noise breach: INFRA-VOID (M2D:411).
   * **Weaker overlap counts as product.** A failure overlapped by a heartbeat gap of any size that
     covers **less than 80%** of its monotonic elapsed time, with no overlapping fork, balloon or steal record, is ambiguous.
     It counts as **FAIL (product)** (M2D:407-410).
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
     (M2R:347-349). Each bound is an absolute monotonic value frozen at requirements freeze and recorded in `FREEZE.md`.
     The measurement is the **worst monotonic elapsed time in the reviewer's scoring run of that gate, at the scoring pin**.
     If it is above bound ÷ 50, the gate is FAIL. The bound is never raised after its first run (OQ-9, §9).

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
     * the venue the gate needs cannot be obtained.
   * **What is recorded:** BLOCKED-VENUE, with the evidence for its cause: either the two INFRA runs' heartbeat, kmsg and noise
     excerpts (rule 2), or a record that the venue could not be obtained or could not hold rule 7's quiet window. The
     project's acceptance stays **pending**, never PASS.
   * **"Fails isolation"** *(R7)* means **exactly** rule 2's INFRA test, carried over from M2 Round 7 (M2D:404-406, M2D:411).
     There is no other trigger. A run on the venue fails isolation when either:
     * a failure is INFRA under rule 2: heartbeat gaps cover at least 80% of its monotonic elapsed time, or a recorded
       `virtual machine fork`/`virtio_balloon` kmsg record or a steal breach overlaps it; or
     * a run with 0 failures has a noise breach (INFRA-VOID). The noise breach is M2's noise gates (M2D:371-379), which
       include a heartbeat gap over 2 s and any `virtual machine fork` or `virtio_balloon` kmsg record.
     The first such run uses rule 2's single rerun. A second makes the gate BLOCKED-VENUE.
   * **Consequence** *(R5; Owner-acked 2026-10-20 16:51 ET)*:
     * A gate whose venue fails isolation, or is BLOCKED-VENUE for any reason, **cannot be PASSED**. An INFRA or INFRA-VOID
       run never counts as a PASS.
     * While that gate is open, its project (P-ECO or P-WEB), and so M3, **cannot be ACCEPTED**.
     * Its **only outcome** is an Owner decision recorded as `WAIVED, NOT PASSED` (below).
   * **No alternate venue, and no venue can be added.** *(Revision R4, 2026-10-20; Owner decision 16:32 ET,
     `milestone3/OWNER_ACK_2026-10-20.md`.)*
     * The Owner declined his own PC as an alternate venue on 2026-10-20 at 15:23 ET.
     * At 16:32 ET he dropped the path for him to name a new venue later ("Drop the Owner-named venue path; blocked checks
       can only be waived, never passed", relayed by EngMgr).
     * No venue may be named or added for any gate.
   * **A BLOCKED-VENUE gate has exactly one outcome:** an Owner decision recorded as `WAIVED, NOT PASSED`. It is never
     PASS, and it is not resolved by moving the gate to another venue.
   * **An Owner decision on a BLOCKED-VENUE gate is recorded as `WAIVED, NOT PASSED`.**
     * It never counts as PASS in any acceptance tally. `DECISION.md` lists it separately from PASS.
     * Its record carries who, the date and time (ET), a verbatim quote and a link.
     * This follows M2's W-B6-venue record: "B6 has no ACCEPTED verdict … it was not passed" (M2D:23-24).

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

**Custody and access (binding; custody revision 2026-10-20, on top of `e36cc520`).**
* **Who may read the sealed set:** Assurance only (the Lain seat, `bot-assurance-0001`, and Assurance sessions it runs), until
  the reveal in step 5.
* **Who may not:**
  * every builder of P-ECO or P-WEB, including Motoko as Dev Lead;
  * any executor, subagent or cloud agent working on M3 code, suites, corpora or fixtures;
  * the Auditor and Dev Lead while they author M3 suites.
  Product and the EM need no access before the reveal and get none.
* **Where it lives:** only in the Assurance-only agent store (`m3-heldout/`). There is no other copy.
* **Never placed:**
  * on the shared box filesystem (`/workspace`, `/tmp`, `/home/box`, any worktree);
  * in Git (any branch, commit, stash or PR);
  * in chat, issue or review comments;
  * in any prompt or task text given to a builder.
  Assurance edits the files in the store, and any scratch file stays in the store and is deleted after use.
* **Reveal:** only at scoring, at the pinned SHA, with the hashes verified in front of the reviewer (step 5).
* **Stated limit:** the store mount ignores `chmod`; its files show mode 0666. Access control is the store's agent-level
  authorization, not file modes.

1. **Sealed set v1 (current).** The set is `verifier.md` (P-ECO), `webapp.md` (P-WEB) and a `MANIFEST` of their sha256. Their
   hashes are committed in §7.3 and in `milestone3/heldout/SEALED.sha256`.
   * **Per-file nonce:** each sealed file begins with `nonce: <64 hex chars>`, a separate 256-bit value read from
     `/dev/urandom`. Because of it, nobody can confirm a guessed check, or a guessed whole file, by hashing candidates against
     the published commitment. Short or formulaic checks therefore can't be brute-forced from their hash.
2. **Ids and counts are fixed at v1.**
   * P-ECO: `M3-E-H01` to `M3-E-H25`, 15 QA and 10 Security.
   * P-WEB: `M3-W-H01` to `M3-W-H28`, 14 QA and 14 Security.
   * The categories and requirement mapping in §7.2 are fixed too.
   * Every replacement (step 4) and every binding amendment (step 3) keeps the same ids, counts, categories and mapping.
3. **Binding amendment (per project).** The P-WEB domain and both frozen contracts do not exist yet: E1's policy format and
   FAIL codes, and W1's routes, roles, fields, session mechanism, messages, loader and policy. Some checks therefore refer to
   "the frozen code/route/field".
   * Assurance writes `sealed-v1-<eco|web>-bind` after that project's requirements freeze and **before its first
     implementation commit**. It binds those references to concrete values, with a fresh nonce, and its hashes are committed
     in a dated revision of §7.3.
   * It may make inputs more concrete and settle whether a conditional check applies (CSRF vs bearer, CSP, a robustness
     clause), decided from the frozen requirements and never at scoring time.
   * It may not add, remove or renumber checks, and it may never weaken a pass condition. The v1-to-bind diff is shown at the
     reveal.
4. **Leak or hash-mismatch procedure.**
   * **Triggers:**
     * any evidence that sealed content reached a party or place outside the custody rules above;
     * any sha256 mismatch between a sealed file and its commitment, at any time, including at the reveal.
   * **Scope:**
     * a leak whose scope is known voids, and gets regenerated, only the affected checks;
     * a leak of unknown scope, or any hash mismatch, voids the whole set.
   * **Replacement:**
     * Assurance issues a replacement with new concrete inputs, payloads and fixture names, keeping the ids and counts
       (step 2).
     * Every file gets a fresh nonce, so every commitment changes.
     * The new commitments land in a docs-only commit to §7.3 and `SEALED.sha256`, which records the date, the reason and
       the voided version.
   * **Rescoring:** if the trigger comes at or after scoring, the score is void. The reviewer rescores with the replacement at
     the same pinned code SHA. A replacement never reopens or re-pins the code.
5. **Reveal and scoring at a pin.** The §1.1 reviewer, a non-builder, scores at a stated pinned SHA and records the §5 rule 9
   pins.
   * Assurance hands over the sealed files only at the start of scoring.
   * Before any check runs, the reviewer runs `sha256sum -c` itself, against `MANIFEST` and against `SEALED.sha256`, to
     confirm each file's sha256 matches §7.3, and records the output in the evidence. Any mismatch triggers step 4.
   * The reveal commit adds the files under `milestone3/<eco|web>/heldout/`, together with a freeze test in the style of
     `experiments/token-efficiency-heldout-v3/spec_freeze_test.go` that asserts the committed hashes.
   * Evidence goes to `milestone3/evidence/E9/` (P-ECO) and `milestone3/evidence/W11/` (P-WEB), in the §1.1 layout with a
     `RESULT` file.
6. **Outcomes and reruns follow §5.** §5.2 sets the outcome classes, the single rerun allowed only after a documented INFRA,
   and BLOCKED-VENUE. A FAIL is never rerun to a PASS. Timing-bound checks use the §5.3 monotonic clock, the §5.4 heartbeat
   sidecar and the §5.7 quiet window. Browser checks run on the WE-2 harness, and the Chrome and harness versions are recorded.
7. **Mutation proof.** Every negative check must be shown to fail under a recorded mutation of the code under test. The sealed
   mutation plan names one or more mutations per check.
   * The proof records the file, the change and the failing excerpt, then reverts the change.
   * Before recording the decision, the reviewer shows that `git status --porcelain` is empty.
   * A negative check with no biting mutation does not count as passed.
8. **Recording.** `milestone3/<eco|web>/DECISION.md` lists every sealed check id with PASS/FAIL and its evidence. QA and
   Security are kept in separate sections, as in M2R §3.
9. **No secrets.** The sealed files use placeholders only. Test secrets are generated at runtime and never printed or written.

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
checks depend on the frozen W1 design (§7.1 step 3): one each for CSRF/same-origin/CORS, CSP and slow-client robustness.

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

### 7.3 Hash commitments (sealed-v1, current)

| File | sha256 |
|---|---|
| `verifier.md` | `567772fbe7b93c99a6c3f1088c51c569c22ed13c6d49426061ca4480d2786062` |
| `webapp.md` | `4ed2243640b479187fd954e6060ff06ed4feff4fb7be6bb5606aec7899c3d532` |
| `MANIFEST` | `754f422511a3e6aec3e045a5b81b79180d52de8c19c17886669f87425a26d96f` |

The same values are in `milestone3/heldout/SEALED.sha256`.

**History**

| Version | Date | Status | Reason |
|---|---|---|---|
| sealed-v0 (`verifier.md` `c0d98995…`, `webapp.md` `2c289dbb…`, `MANIFEST` `a45bbb88…`; committed in `baeb0d03`) | 2026-10-20 | **VOID** | Potentially exposed, so the whole set is void (§7.1 step 4, unknown scope). A working copy had been kept on the shared box under `/workspace`, which every agent can read, future builders included (#205 review, Lain's blocker 3). That copy was shredded on 2026-10-20. The box, `/tmp`, `/home/box`, the worktrees and their git objects, stash and reflog were searched, and no other copy was found. v0 had no nonces. |
| sealed-v1 | 2026-10-20 | **Current** | Replaces v0 with new concrete inputs, payloads and fixture names, and a per-file nonce. Ids and counts are unchanged and now fixed (§7.1 step 2). Held only in the Assurance-only agent store. |

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
  **Resolved by Assurance (2026-10-20):** yes, and stricter. The box copy was destroyed, sealed-v0 was voided and replaced by
  sealed-v1, and the custody rules in §7.1 now forbid any copy outside the Assurance-only store.

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

## 9. Product response to §7.5 (outside §7; §7 is unchanged)

Product owns requirements and acceptance, so it answers here. The stricter reading is taken unless cited evidence says it
does not apply. Each answer is applied in the section named.

| OQ | Product decision | Status / owner | Applied in |
|---|---|---|---|
| **OQ-1** | **§7.4 governs.** §4 now matches it: m17 ENTRY, m4 ENTRY if P-WEB extends ticketdesk (otherwise EXIT), m3 EXIT, m12 ENTRY. Stricter for m2: ENTRY if bearer auth is used, with a runtime fix **verified before entry**, not a requirements exclusion. Motoko's re-review agrees that m17 is ENTRY. | **Resolved here.** Motoko's re-review backs m17 ENTRY. | §4; WE-4, WE-5, WE-6; W5; W14 |
| **OQ-2** | A receipt whose `denied` decision falls under a policy equal to the run's grants is **PASS**: a denial is enforcement working, and is within policy. The output must still list every denial with its capability and target summary. An *allowed* decision on a capability the policy denies stays FAIL (E4). | Product direction; **resolved at requirements freeze** in E1, before sealed v1. Owner: E1 author (Auditor). | E1 contract; E3 |
| **OQ-3** | **Any difference from the policy is FAIL**, tighter or looser, because the tool reports conformance, not risk. The frozen FAIL codes must tell `limits_looser` apart from `limits_tighter`, so a reader can judge severity. | Product direction; **resolved at requirements freeze** in E1, before sealed v1. Owner: E1 author (Auditor). | E1; E4 |
| **OQ-4** | The subsumption rules are frozen in E1, from these principles (the strict readings):<ul><li>filesystem roots are compared on lexically cleaned, absolute paths by **whole path components**, so `/a/b` does not cover `/a/bc`. A trailing slash is normalised away. Any `..` left after cleaning, or a relative root, is malformed (FAIL);</li><li>coarse `network` is broader than every `network:fetch=host[:port]`. A host without a port is broader than host:port;</li><li>`inherit:filesystem` counts as the broadest filesystem grant (coarse; M2D:778-779);</li><li>`docs/cli.md:45` says receipts carry **sorted unique** grants, so a receipt with duplicate or unsorted `grant[]` is malformed (FAIL). Duplicates in a policy are a policy error with its own exit code.</li></ul>The exact grammar and the tie cases are left to the E1 author. | Product principles; **resolved at requirements freeze** in E1 (exact grammar), before sealed v1. Owner: E1 author (Auditor). | E1; E4 |
| **OQ-5** | A pinned fixture allowlist, `milestone3/eco/tests/fixtures/E5_ALLOWLIST`, with its sha256 in `FREEZE.md`. Every other path under `milestone3/eco/` is scanned. A `tampered` name outside the allowlist still FAILs, and the reviewer proves it with a control file. | Product decision; the allowlist contents are **resolved at requirements freeze**, before sealed v1. Owner: Auditor. | E5 |
| **OQ-6** | **Superseded:** W13 parses the loader with a pinned parser (Python stdlib `html.parser`, version and script sha256 pinned). It checks for exactly one `script` element with `src="app.js"` and no inline content, no other executable elements, no `on*` attributes, no `javascript:`/`data:`/`vbscript:` URLs, and the pinned hash. The earlier `rg` count is withdrawn. | **Resolved here** (superseded: parser check). The parser script is frozen at requirements freeze. Owner: Auditor. | W13 |
| **OQ-7** | A pinned headless-Chrome driver is allowed as WE-2 test tooling (tool, version and sha256 recorded; vendored or from a hash-verified local copy; no network). It is not an app runtime dependency. **W9 runs both the API and the browser suites.** | **Resolved here.** The driver is allowed, and W9 runs the browser suites. | WE-2; W9 |
| **OQ-8** | The server is **always HFBC under `howlframe run`**. If WE-3 falls back, W8 runs the same frozen W8 workload truncated to a pre-registered request count N (receipt at most half the 10k cap), recorded in `FREEZE.md` before the freeze. Any W8 or held-out W8 receipt with `effects_truncated: true` is FAIL. Assurance may pick the M3-W-H14 workload, but the same no-truncation rule applies to it. | Product decision; the value of N is **resolved at requirements freeze** in `FREEZE.md`, before sealed v1. Owner: W1 author (Dev Lead or Auditor). | WE-3; W8 |
| **OQ-W1** | `set_text` is required for any data-derived value. `set_html` is allowed only with compile-time literal markup. W1 freezes a URL-scheme allow-list for `href`/`src`: relative, `http:` and `https:` only. | Product direction; **resolved at requirements freeze** in W1, before sealed v1. Owner: W1 author (Dev Lead or Auditor). | W4 |
| **OQ-W2** | W1 must freeze one session mechanism. Whichever it is, a CSRF/same-origin check is **active**, there is no permissive CORS, a cookie session uses `HttpOnly` and `SameSite=Strict`, and no token goes in `localStorage` or `sessionStorage`. The choice of mechanism is left to the W1 author. | Product direction; the mechanism is **resolved at requirements freeze** in W1, before sealed v1. Owner: W1 author (Dev Lead or Auditor). | W5 |
| **OQ-W3** | **A CSP is required**, at least `default-src 'self'; script-src 'self'; object-src 'none'; base-uri 'none'`, so Assurance's conditional CSP check is **active**. This is feasible because the loader has only `<script src>` (W13). | Product direction; the exact CSP is **resolved at requirements freeze** in W1, before sealed v1. Owner: W1 author. | W4 |
| **OQ-W4** | **Yes**, a slow-client robustness clause equivalent to M2 R17 (the B3 lesson, M2D:760-761; M2R:357), so the conditional robustness check is **active**. | Product direction; the bound values are **resolved at requirements freeze** in W1, before sealed v1. Owner: W1 author. | W2 |
| **OQ-W5** | **Yes**, W1 freezes a code-to-message table. | Product direction; the table is **resolved at requirements freeze** in W1, before sealed v1. Owner: W1 author. | W6 |
| **OQ-9** | The bound is an **absolute monotonic value**, frozen at requirements freeze by the suite author and recorded in `FREEZE.md`. It is not derived from the implementation. The measurement is the worst monotonic elapsed time in the **reviewer's scoring run** of that gate at the scoring pin. If it is above bound ÷ 50, the gate is FAIL. A bound is never raised after its first run; any change needs a dated revision before that run. | **Resolved here.** Bound frozen at requirements freeze; measured at the reviewer's scoring run. | §5 rule 3 (read with this answer) |
| **OQ-10** | **Not Product's to decide.** Where the sealed set is stored is Assurance's operating practice, and the files sit outside this repo. Product's recommendation, the stricter reading: remove the shared-box copy once the hashes are committed, keep only the agent-store copy, and record the removal in the Assurance journal. Assurance decides. | **Open, Assurance's.** It is Lain's blocker #3; Product does not edit it. | none (recommendation to Assurance) |

**Unanswered, with reason:** OQ-10 only. Where the sealed copy is stored is Assurance's operating practice, and it is Lain's blocker #3 in the `baeb0d03` re-review; Product leaves §7 untouched. Every other OQ is either resolved here or marked **resolved at requirements freeze**, with its owner, before sealed v1.

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
