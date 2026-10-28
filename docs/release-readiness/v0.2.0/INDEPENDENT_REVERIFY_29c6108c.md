# Re-Verification Report: HowlFrame v0.2.0 Candidate (Correction Round 1)

**Reviewer:** Independent Reviewer (DeepMind Advanced Agentic Coding)  
**Date:** 2026-10-28  
**Prior RC Commit:** `bbd4c8f5`  
**Corrected Candidate Commit:** `29c6108c`  
**Base Commit:** `def34540` (`main`)  
**Target Release:** v0.2.0 (HFBC format: 2)  
**Working Directory:** `/tmp/claude-1000/-run-media-system-tallgeese-dev/3ac1ff33-04ad-47b6-b4cf-327b85acc602/scratchpad/review2`  
**Report Output:** `work/REVERIFY.md`  

---

## 1. Executive Summary

This evaluation performs the bounded re-verification of the HowlFrame v0.2.0 release candidate following one correction round by the authors. The review covers the four checks requested in `BRIEF.md`:
1. Re-verification of FINDING-01 (`spawn_agent` format 1 artifact compatibility claims across documentation).
2. Re-verification of FINDING-02 (presence of `tree/docs/release-readiness/v0.2.0/` and resolution of citations).
3. Concrete line-by-line validation of claims introduced in `diff_since_review.patch`.
4. Confirmation that `diff_since_review.patch` modifies documentation exclusively.

---

## 2. Check 1: FINDING-01 (`spawn_agent` Compatibility Claims)

### Evaluation Scope
Verify whether the updated statements regarding legacy `spawn_agent` format 1 bytecode in:
- `tree/docs/reference/artifact_compatibility.md` (line 41)
- `tree/docs/releases/v0.2.0.md` (line 44)
- `tree/docs/release-readiness/v0.2.0/HOWLFRAME_RELEASE_COMPATIBILITY.md` (lines 52–53, 225–226)

are now accurate and verifiable by executing both bytecode forms:
- **Form A**: `tree/examples/swarm_dogfood/swarm_dogfood.howl` (bearing `(task ...)` sub-forms) compiled with `bin/howlframe-v0.1.0`.
- **Form B**: `(cli_app (spawn_agent "worker" (print "child")) (print "parent"))` (without `task` sub-form) compiled with `bin/howlframe-v0.1.0`.

Each compiled format 1 artifact was executed on both `bin/howlframe-v0.1.0` and `bin/howlframe` (staged v0.2.0) using `-run-bc -allow-caps process`.

### Commands Run

```bash
# 1. Compile Form A (swarm_dogfood) with v0.1.0 compiler
./bin/howlframe-v0.1.0 -compile-bc tree/examples/swarm_dogfood/swarm_dogfood.howl -o work/swarm_v1.hfbc

# 2. Prepare and compile Form B with v0.1.0 compiler
cat << 'EOF' > work/form_b.howl
(cli_app (spawn_agent "worker" (print "child")) (print "parent"))
EOF
./bin/howlframe-v0.1.0 -compile-bc work/form_b.howl -o work/form_b_v1.hfbc

# 3. Verify format 1 header on compiled artifacts
od -t x1 work/swarm_v1.hfbc | head -n 1
# Output: 48 46 42 43 01 00 00 00 ... (HFBC format: 1)

# 4. Form A execution on v0.1.0
./bin/howlframe-v0.1.0 -run-bc -allow-caps process work/swarm_v1.hfbc

# 5. Form A execution on v0.2.0
./bin/howlframe -run-bc -allow-caps process work/swarm_v1.hfbc

# 6. Form B execution on v0.1.0
./bin/howlframe-v0.1.0 -run-bc -allow-caps process work/form_b_v1.hfbc

# 7. Form B execution on v0.2.0
./bin/howlframe -run-bc -allow-caps process work/form_b_v1.hfbc
```

### Observed Output

1. **Form A on v0.1.0**:
   ```text
   swarm start
   VM internal panic: unknown opcode: TASK
   ```
   *Exit code:* `1`
2. **Form A on v0.2.0**:
   ```text
   swarm start
   [Swarm VM] Spawning agent "Researcher" for task: "collect sources"
   [Swarm VM] Agent "Researcher" completed task: "collect sources"
   [Swarm VM] Agent "Writer" for task: "draft summary"
   [Swarm VM] Agent "Writer" completed task: "draft summary"
   [Swarm VM] Spawning agent "Reviewer" for task: "check summary"
   [Swarm VM] Agent "Reviewer" completed task: "check summary"
   swarm done
   ```
   *Exit code:* `0`
3. **Form B on v0.1.0**:
   ```text
   child
   VM internal panic: unknown opcode: SPAWN_AGENT
   ```
   *Exit code:* `1`
4. **Form B on v0.2.0**:
   ```text
   child
   {"phase":"runtime","code":"STACK_UNDERFLOW","function":"main","instruction":2,"opcode":"SPAWN_AGENT","message":"stack underflow at SPAWN_AGENT"}
   ```
   *Exit code:* `1`

*(Note: Execution via `howlframe run --allow-caps process` yielded identical results on both artifacts).*

### Analysis
The measured runtime behaviors exactly match the updated documentation:
- `artifact_compatibility.md:41`: states `spawn_agent (did not run on v0.1.0: unknown opcode: SPAWN_AGENT or TASK): **Depends on the form**: with (task ...) bodies it now runs (examples/swarm_dogfood completes under process); a body without task fails closed with STACK_UNDERFLOW at SPAWN_AGENT`.
- `v0.2.0.md:44`: states reads format 1: `spawn_agent runs or fails closed depending on its form`.
- `HOWLFRAME_RELEASE_COMPATIBILITY.md:52-53`: records `spawn_agent without task` fails on both with `unknown opcode: SPAWN_AGENT` and `STACK_UNDERFLOW at SPAWN_AGENT`; and `spawn_agent with (task ...) bodies` fails on v0.1.0 with `unknown opcode: TASK` but runs to `swarm done` under `process` on v0.2.0.

### Status
**RESOLVED**

---

## 3. Check 2: FINDING-02 (`docs/release-readiness/v0.2.0/` Evidence Directory)

### Evaluation Scope
Verify whether directory `tree/docs/release-readiness/v0.2.0/` exists in tree commit `29c6108c`, and whether all references to it in:
- `tree/SECURITY.md` (line 18)
- `tree/docs/reference/artifact_compatibility.md` (line 35)
- `tree/docs/releases/v0.2.0.md` (lines 102–103)
resolve to real files.

### Commands Run

```bash
# 1. Check directory existence and contents
ls -la tree/docs/release-readiness/v0.2.0/

# 2. Check citation targets
# SECURITY.md:18 -> docs/release-readiness/v0.2.0/HOWLFRAME_RELEASE_SECURITY.md
ls -l tree/docs/release-readiness/v0.2.0/HOWLFRAME_RELEASE_SECURITY.md

# artifact_compatibility.md:35 -> HOWLFRAME_RELEASE_COMPATIBILITY.md under docs/release-readiness/v0.2.0/
ls -l tree/docs/release-readiness/v0.2.0/HOWLFRAME_RELEASE_COMPATIBILITY.md

# docs/releases/v0.2.0.md:102-103 -> docs/release-readiness/v0.2.0/
ls -ld tree/docs/release-readiness/v0.2.0/
```

### Observed Output

1. `tree/docs/release-readiness/v0.2.0/` exists and contains four markdown documents:
   - `HOWLFRAME_NUMERIC_CORRECTNESS.md` (10,105 bytes)
   - `HOWLFRAME_RELEASE_COMPATIBILITY.md` (8,098 bytes)
   - `HOWLFRAME_RELEASE_SECURITY.md` (6,158 bytes)
   - `HOWLFRAME_RELEASE_TEST_RESULTS.md` (4,470 bytes)
2. `tree/SECURITY.md:18` points to `docs/release-readiness/v0.2.0/HOWLFRAME_RELEASE_SECURITY.md`: **Resolves**.
3. `tree/docs/reference/artifact_compatibility.md:35` points to `HOWLFRAME_RELEASE_COMPATIBILITY.md under docs/release-readiness/v0.2.0/`: **Resolves**.
4. `tree/docs/releases/v0.2.0.md:102–103` points to `docs/release-readiness/v0.2.0/`: **Resolves** to the newly committed directory.

### Status
**RESOLVED**

---

## 4. Check 3: Statement-by-Statement Audit of `diff_since_review.patch`

### Evaluation Scope
Verify concrete technical and factual claims across all lines added by `diff_since_review.patch`. Per `BRIEF.md`, template placeholders in `HOWLFRAME_RELEASE_TEST_RESULTS.md` (`FINAL_GATES`, `FINAL_RC_SHA`, `STAGED_ACCEPTANCE`, `STAGED_BINARY_HASHES`, `STAGED_EVIDENCEDESK`) are acknowledged pending final regression recording. All other claims were verified against the code and binaries.

### Commands Run & Verifications

#### A. Numeric Correctness Claims (`HOWLFRAME_NUMERIC_CORRECTNESS.md`)
1. **Defects N1–N4 & Reproductions:**
   ```bash
   # Test float multiplication overflow across -run and run
   echo '(cli_app (let (x (to_float "1e308")) (print (* x 10.0))))' > work/overflow.howl
   ./bin/howlframe -run work/overflow.howl
   # Output: exit 1, {"reason":"floating-point overflow: 1e+308 * 10 is not finite","line":1,"column":45}
   ./bin/howlframe build work/overflow.howl -o work/overflow.hfbc && ./bin/howlframe run work/overflow.hfbc
   # Output: exit 1, {"phase":"runtime","code":"RUNTIME_ERROR","function":"main","instruction":0,"message":"floating-point overflow: 1e+308 * 10 is not finite"}

   # Test integer literal out of range check
   echo '(cli_app (print 99999999999999999999))' > work/int_lit.howl
   ./bin/howlframe check work/int_lit.howl
   # Output: exit 1, {"reason":"integer literal 99999999999999999999 is outside the int64 range [-9223372036854775808, 9223372036854775807]","line":1,"column":17}

   # Test float literal out of range check
   python3 -c 'print("(cli_app (print 1" + "0"*309 + ".0))")' > work/float_lit.howl
   ./bin/howlframe check work/float_lit.howl
   # Output: exit 1, {"reason":"float literal ... is outside the finite float64 range","line":1,"column":17}

   # Test JSON parsing of 1e400
   echo '(cli_app (let (raw "{\"v\": 1e400}") (let (d (parse_json Any raw)) (print d))))' > work/json.howl
   ./bin/howlframe -run work/json.howl
   # Output: exit 1, {"reason":"parse_json failed: strconv.ParseFloat: parsing \"1e400\": value out of range","line":1,"column":46}

   # Test scientific exponent token check
   echo '(cli_app (print 1e10))' > work/exp.howl
   ./bin/howlframe check work/exp.howl
   # Output: exit 1, {"reason":"undefined reference to \"e10\"","line":1,"column":18}

   # Test to_float with Go-accepted syntax
   echo '(cli_app (print (to_float "1_000.5")))' > work/tf.howl
   ./bin/howlframe -run work/tf.howl
   # Output: exit 0, 1000.5
   ```
   *Result:* Fully verified. All numeric boundary and error behaviors conform to the claims.

2. **Targeted Numeric Regression Tests:**
   ```bash
   cd tree && GOTOOLCHAIN=go1.25.14 go test ./internal/vm -run 'TestFloatArithmetic|TestNaNOrdering|TestConfidenceRefuses' -count=1
   cd tree && GOTOOLCHAIN=go1.25.14 go test ./internal/bytecode -run TestArtifactRefusesNonFiniteConstants -count=1
   cd tree && GOTOOLCHAIN=go1.25.14 go test ./internal/backend/javascript -count=1
   cd tree && GOTOOLCHAIN=go1.25.14 go test . -run 'TestNumeric|TestIssue151|TestIssue158|TestIssue161|TestIssue156|TestIssue149|TestIssue225' -count=1
   ```
   *Result:* All test suites passed cleanly with exit code 0 (`vm`: 0.017s, `bytecode`: 0.012s, `javascript`: 22.896s, root package: 35.768s).

#### B. Compatibility Claims (`HOWLFRAME_RELEASE_COMPATIBILITY.md`)
1. **Format 1 vs Format 2 Version Reporting:**
   - `./bin/howlframe-v0.1.0 version` -> `HowlFrame 0.1.0` / `HFBC format: 1`.
   - `./bin/howlframe version` -> `HowlFrame 0.2.0` / `HFBC format: 2`.
   - `./bin/howlframe help` -> banner displays `HowlFrame 0.2.0 CLI`.
   - `./bin/howlframe interop --json` -> `"version": "0.2.0"`, `"hfbc_format": "2"`.
   - Artifact header bytes: Format 1 begins `48 46 42 43 01 00 00 00`, Format 2 begins `48 46 42 43 02 00 00 00`.
   *Result:* Fully verified.

2. **Format 1 Artifact Compatibility Matrix:**
   - `kv_cli.hfbc`: On v0.1.0 sets/gets theme (`dark`, exit 0); on v0.2.0 fails closed with `STACK_UNDERFLOW at STORE_OPEN` (exit 1).
   - `todo_cli.hfbc`: On v0.1.0 adds and lists tasks (`ADDED 1 Task 1`, exit 0); on v0.2.0 fails closed with `STACK_UNDERFLOW at STORE_OPEN` (exit 1).
   - `release_authority.hfbc`: Executed with proposal missing `target`: on v0.1.0 outputs `"target": ""`; on v0.2.0 outputs `"target": "unknown"` due to `map_get` missing key returning `nil`.
   *Result:* Fully verified.

3. **Malformed Artifact Rejection:**
   Tested empty, truncated, invalid magic (`NOPE...`), and version 3 (`HFBC 03...`) artifacts against `run`, `-run-bc`, and `inspect --caps`.
   *Result:* All fail closed with exit code 1 and clean JSON diagnostics (no panics).

#### C. Security Claims (`HOWLFRAME_RELEASE_SECURITY.md`)
1. **Capability Denials & Path Constraints:**
   - `exec` with bare `process` grant fails closed (`capability denied: process`).
   - `process:env=LD_PRELOAD` is refused at startup.
2. **Resource Limits & Budget Exit Status Inconsistency:**
   - `./bin/howlframe run --max-instructions 0 work/hello.hfbc` exits with code `1`.
   - `./bin/howlframe run --max-memory-bytes 0 work/hello.hfbc` exits with code `2`.
   - `./bin/howlframe -run-bc -max-memory-bytes -5 work/hello.hfbc` exits with code `1`.
   *Result:* Accurately matches the documentation in `HOWLFRAME_RELEASE_SECURITY.md:313` and release notes.
3. **SEO Validation:**
   `python3 tree/scripts/test_seo.py` ran with output `All SEO validations PASSED successfully!`.

#### D. Clean Install Procedure Claims (`HOWLFRAME_RELEASE_TEST_RESULTS.md`)
Replicated all CLI commands in Section 4 (`check`, `build`, `run`, `inspect --caps`, `check --profile governed`, `--max-instructions 5000`, `--deadline 300ms`).
*Result:* Every command exhibited the exact return codes and stdout/stderr output documented.

---

### Finding Uncovered During Audit: FINDING-03

While auditing `HOWLFRAME_NUMERIC_CORRECTNESS.md`, line 167 of `diff_since_review.patch` (corresponding to line 135 of `tree/docs/release-readiness/v0.2.0/HOWLFRAME_NUMERIC_CORRECTNESS.md`) introduces the following assertion:
> *"Its overall verdict and the two documentation findings it raised elsewhere are recorded in `HOWLFRAME_RELEASE_CANDIDATE.md`."*

Additionally, `tree/docs/releases/v0.2.0.md` lines 102–103 states:
> *"The release-candidate evidence (numeric correctness, compatibility, test results, security classification and the independent review) is in `docs/release-readiness/v0.2.0/`."*

However, no file named `HOWLFRAME_RELEASE_CANDIDATE.md` exists in `tree/docs/release-readiness/v0.2.0/`, nor anywhere else in the repository. The prior review report remained as `PRIOR_REVIEW.md` outside the tree and was never committed into `docs/release-readiness/v0.2.0/`.

#### Finding Details
- **ID:** FINDING-03
- **Severity:** Low
- **Location:**
  - `tree/docs/release-readiness/v0.2.0/HOWLFRAME_NUMERIC_CORRECTNESS.md:135`
  - `tree/docs/releases/v0.2.0.md:102–103`
- **Description:**
  `HOWLFRAME_NUMERIC_CORRECTNESS.md` states that the independent review's overall verdict and findings are recorded in `HOWLFRAME_RELEASE_CANDIDATE.md`, and the release notes state that the independent review is located in `docs/release-readiness/v0.2.0/`. However, `tree/docs/release-readiness/v0.2.0/HOWLFRAME_RELEASE_CANDIDATE.md` does not exist in the repository tree.
- **Reproducible Command:**
  ```bash
  ls -l tree/docs/release-readiness/v0.2.0/HOWLFRAME_RELEASE_CANDIDATE.md
  ```
- **Observed Output:**
  ```text
  ls: cannot access 'tree/docs/release-readiness/v0.2.0/HOWLFRAME_RELEASE_CANDIDATE.md': No such file or directory
  ```
- **Suggested Fix:**
  Commit the independent review report into `tree/docs/release-readiness/v0.2.0/HOWLFRAME_RELEASE_CANDIDATE.md` or update the reference.

### Status
**NEW FINDING** (FINDING-03, Low severity)

---

## 5. Check 4: Documentation-Only Patch Confirmation

### Evaluation Scope
Confirm that `diff_since_review.patch` modifies documentation files only, with zero alterations to Go source, Howl source, tests, shell scripts, or CI workflows.

### Commands Run

```bash
python3 - << 'EOF'
import re

with open("diff_since_review.patch") as f:
    patch = f.read()

diff_files = re.findall(r'^diff --git a/(.*?) b/(.*?)$', patch, re.MULTILINE)
for a, b in diff_files:
    print(f"{a} -> {b}")
EOF
```

### Observed Output
```text
README.md -> README.md
docs/reference/artifact_compatibility.md -> docs/reference/artifact_compatibility.md
docs/release-readiness/v0.2.0/HOWLFRAME_NUMERIC_CORRECTNESS.md -> docs/release-readiness/v0.2.0/HOWLFRAME_NUMERIC_CORRECTNESS.md
docs/release-readiness/v0.2.0/HOWLFRAME_RELEASE_COMPATIBILITY.md -> docs/release-readiness/v0.2.0/HOWLFRAME_RELEASE_COMPATIBILITY.md
docs/release-readiness/v0.2.0/HOWLFRAME_RELEASE_SECURITY.md -> docs/release-readiness/v0.2.0/HOWLFRAME_RELEASE_SECURITY.md
docs/release-readiness/v0.2.0/HOWLFRAME_RELEASE_TEST_RESULTS.md -> docs/release-readiness/v0.2.0/HOWLFRAME_RELEASE_TEST_RESULTS.md
docs/releases/v0.2.0.md -> docs/releases/v0.2.0.md
```

### Analysis
All 7 modified/added files are Markdown (`.md`) documentation files:
- 1 file in repository root (`README.md`).
- 6 files in `docs/` (`docs/reference/`, `docs/releases/`, and `docs/release-readiness/v0.2.0/`).
- Zero changes to Go source code (`*.go`).
- Zero changes to Howl application code (`*.howl`).
- Zero changes to test suites (`*_test.go`, `*.py`).
- Zero changes to build or CI workflow configurations (`.github/workflows/*`).

### Status
**CONFIRMED** (Documentation only)

---

## 6. Findings Summary

| Finding | Severity | Status | Summary |
|---|---|---|---|
| **FINDING-01** | Medium | **RESOLVED** | Legacy `spawn_agent` format 1 artifact behavior is accurately documented across all release documentation: `(task ...)` bodies execute cleanly to completion under `process`, while bodies without `task` fail closed with `STACK_UNDERFLOW at SPAWN_AGENT`. |
| **FINDING-02** | Low | **RESOLVED** | The evidence directory `tree/docs/release-readiness/v0.2.0/` is committed and contains `HOWLFRAME_NUMERIC_CORRECTNESS.md`, `HOWLFRAME_RELEASE_COMPATIBILITY.md`, `HOWLFRAME_RELEASE_SECURITY.md`, and `HOWLFRAME_RELEASE_TEST_RESULTS.md`. Citations in `SECURITY.md`, `artifact_compatibility.md`, and `v0.2.0.md` resolve to this directory. |
| **FINDING-03** | Low | **NEW FINDING** | `HOWLFRAME_NUMERIC_CORRECTNESS.md:135` cites `HOWLFRAME_RELEASE_CANDIDATE.md` for the independent review report and `docs/releases/v0.2.0.md:102–103` lists the independent review as present in `docs/release-readiness/v0.2.0/`, but `HOWLFRAME_RELEASE_CANDIDATE.md` is missing from the repository tree. |

---

## 7. Final Verdict

### **ACCEPT WITH DOCUMENTED LIMITATIONS**

The HowlFrame v0.2.0 candidate corrections successfully resolve both prior review findings:
1. `spawn_agent` legacy format 1 compatibility claims are now completely accurate, discriminating between `(task ...)` and bare bodies, both verified experimentally on v0.1.0 and v0.2.0 runtimes.
2. The release-readiness evidence directory `docs/release-readiness/v0.2.0/` has been created and populated with comprehensive reports covering numeric correctness, compatibility, security, and test results.
3. The correction patch is strictly scoped to documentation, leaving all compiler, runtime, and CI configurations untouched.
4. All concrete numeric, security, budget, and packaging claims introduced in `diff_since_review.patch` are truthful and verified by direct execution.

This acceptance is subject to the single documented limitation identified in FINDING-03 (the independent review markdown file `HOWLFRAME_RELEASE_CANDIDATE.md` cited in `HOWLFRAME_NUMERIC_CORRECTNESS.md` and release notes should be committed to `docs/release-readiness/v0.2.0/`).
