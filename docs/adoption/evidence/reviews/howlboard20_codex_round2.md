# Round 2 review: HowlBoard PR #20

Verdict: **ACCEPT WITH NOTES**

Both round 1 P2 findings are resolved. Claims 1 and 9 are now confirmed by the full test suite and an independent import/server/HTTP run. The only remaining note is incomplete committed rejection coverage for `agent_id` at 121 characters; direct execution confirms the correct behavior. No blocking defect was found within the four items in `REVIEW_BRIEF_ROUND2.md`.

Reviewed HowlBoard `4bb4cf11593cf32088475add1b5562bacb21f355` against `135cdf7`. Evidence is in `review-evidence-round2/`. No application source or tests were edited, and HowlBoard's final `git status --porcelain` is empty. Test impact is confined to the ID-boundary additions; the brief explicitly requires the full regression suite and HTTP checks, which were executed. This report does not re-review other round 1 claims.

## 1. ID limit, boundary tests, and vendored identity

**PASS, with one nonblocking test-coverage note.**

`README.md:139-142`, `change_log.md:6`, and `tools/ledger_import/VENDORED.md:21-23` explicitly identify the three capped fields, `entry_id`, `task_id`, and `agent_id`, and the inclusive 120-character maximum. The importer header (`tools/ledger_import/ledger_import.howl:14-20`) also states that IDs are capped at 120. Its wording is less explicit about the field names, but consistent with the implementation: `action` is required and has no 120-character cap. The changelog explicitly acknowledges that the previous importer accepted longer IDs.

The committed success case at `backend/ledger_import_validation_test.go:209-212` imports all three IDs at length 120. The cases at lines 149-150 reject 121-character `entry_id` and `task_id`, require exit 3 and the field-specific diagnostic, and verify both preexisting stores remain byte-identical. These tests passed.

**Note:** there is no committed 121-character `agent_id` rejection case. Adding the analogous table row would complete coverage of the documented boundary for all three fields. The independent runtime probe tested each field separately at 120 and 121: all six cases behaved correctly, and all three rejections preserved both prior stores. This is a coverage gap, not an observed implementation failure.

Commands:

```bash
git -C howlboard diff 135cdf7..4bb4cf1 -- README.md change_log.md \
  tools/ledger_import/VENDORED.md tools/ledger_import/ledger_import.howl \
  backend/ledger_import_validation_test.go
cmp howlboard/tools/ledger_import/evidence_ledger.howl \
  howlframe-pr211/milestone3/evidencedesk/app/ledger.howl
sha256sum howlboard/tools/ledger_import/evidence_ledger.howl \
  howlframe-pr211/milestone3/evidencedesk/app/ledger.howl
git -C howlboard diff --exit-code 135cdf7..4bb4cf1 -- \
  tools/ledger_import/evidence_ledger.howl
python3 review-evidence-round2/check_runtime.py
```

`cmp` and the empty vendored-module diff exited 0. Both hashes equal the recorded SHA-256, `aacde2501430bf7b34a05a36e3de468bc667af757a37a78cd5328c8498e52d55`. The upstream checkout is `9dbd81756cc61ea43c797a610214a68478793a85`. `TestVendoredLedgerModuleUnchanged` also passed. The module remains byte-identical to upstream and unchanged by the correction.

## 2. Failure guarantee

**PASS.**

`change_log.md:7` now promises byte-identical stores on **validation** failure and immediately states that a later persistence failure can leave missions updated while the timeline is unchanged. `README.md:143-148` gives the same qualification and concrete examples. The importer header's preservation statement applies specifically to malformed, duplicate, or over-limit input; it makes no unconditional persistence guarantee. No remaining unconditional failure-preservation claim was found in the reviewed documentation/header.

Commands:

```bash
rg -n 'failure|fails|byte-identical|guarantee|stores stay' \
  howlboard/README.md howlboard/change_log.md \
  howlboard/tools/ledger_import/VENDORED.md \
  howlboard/tools/ledger_import/ledger_import.howl
sed -n '320,333p' howlboard/tools/ledger_import/ledger_import.howl
python3 review-evidence-round2/check_runtime.py
```

The existing rejection tests passed, including preservation assertions. The independent probe also recreated the documented persistence limitation: valid input with missions `{}` and a corrupt timeline exits 1 with `STORE_INVALID_PERSISTED_JSON`, changes missions, and leaves the corrupt timeline unchanged. This agrees with the newly scoped guarantee. The README/changelog also now qualify the memory-budget language for verification-heavy slices.

The private-ledger size/longest-ID measurement used as context in the changelog was not independently remeasured; no private ledger was accessed. It is not needed to establish the documented limit or its behavior.

## 3. Claims 1 and 9: full suite and served import

**PASS. Both claims are now confirmed within the requested checks.**

Commands (run from this review directory):

```bash
cp hf-pin howlboard/howlframe_bin
export GOCACHE="$PWD/howlframe-board-pin/review-evidence/go-cache"
export GOTMPDIR="$PWD/review-evidence-round2/tmp"
export TMPDIR="$PWD/review-evidence-round2/tmp"
export GOPROXY=off GOTOOLCHAIN=local
make -C howlboard test
go -C howlboard/backend test -count=1 -v \
  -run '^Test(LedgerImport(RejectsInvalidLedgerWithoutWriting|EntryLimitAndOptionalMetadata|ServedByBoard)|VendoredLedgerModuleUnchanged)$' .
python3 review-evidence-round2/check_runtime.py
ss -ltnp 'sport = :8080'
```

`make test` exited 0, compiled all four build targets, and completed `go test -count=1 ./...` successfully (`ok howlboard/backend 2.768s`). The focused verbose rerun exited 0 and explicitly recorded `TestLedgerImportServedByBoard` passing, alongside the boundary/validation and vendor-hash tests. Logs: `make-test.log` and `focused-tests.log`.

The retained runtime script executes `make -C howlboard import`, followed by `make -C howlboard run`, then makes HTTP GET requests to `http://127.0.0.1:8080/api/missions` and `/api/timeline`. Results:

| Check | Observed result |
| --- | --- |
| `make import` | Exit 0; `entries=72`, `missions=16` |
| Imported stores | Both parsed stores equal the recorded pre-validation golden projection |
| GET `/api/missions` | HTTP 200; exactly 16 missions, all provenance `LEDGER`; mission IDs and every stored summary field match the import |
| GET `/api/timeline` | HTTP 200; exactly 72 events; entire parsed response equals the imported timeline feed |
| Shutdown | Reviewer terminated the `make run` process group and waited; port 8080 has no listener |

**The served data is the import.** HTTP bodies, imported store snapshots, build/import/server logs, and assertions are retained in `review-evidence-round2/`.

The first runtime-script attempt passed both HTTP comparisons but failed its shutdown bind assertion because sockets remained in TCP `TIME_WAIT`; `ss` showed no listener. The reviewer corrected the probe to use `SO_REUSEADDR`, reran it, and obtained exit 0 with all assertions passing. The first log is retained as `runtime-check-first-attempt.log`. No product change was involved. `make-run.log` ends with `Terminated` because the reviewer intentionally stopped the long-running server after verification.

## 4. Correction diff scope

**PASS. Documentation and tests only; no executable code change.**

Commands:

```bash
git -C howlboard diff --name-status 135cdf7..4bb4cf1
git -C howlboard diff --stat 135cdf7..4bb4cf1
git -C howlboard diff 135cdf7..4bb4cf1
git -C howlboard diff --check 135cdf7..4bb4cf1
git -C howlboard status --porcelain
```

Exactly five files changed: `README.md`, `change_log.md`, `tools/ledger_import/VENDORED.md`, `backend/ledger_import_validation_test.go`, and the header comments of `tools/ledger_import/ledger_import.howl` (29 insertions, 12 deletions). Inspection of the full diff, plus a comparison excluding blank lines and full-line semicolon comments, confirms the importer executable lines are identical. No Makefile, server, projection logic, or vendored-module changes occur in this range. `diff --check` passed and the final HowlBoard working tree is clean.
