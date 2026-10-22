Verdict: **REJECT**

The adoption is real at the source/build/import level, and the tested projections match. However, it adds an undocumented ID-length restriction, and the changelog's unconditional failure-preservation promise is false. Fix or explicitly document the restriction and narrow the failure guarantee before acceptance. No new projection corruption was found. Full HTTP verification is blocked by this sandbox's socket prohibition.

Reviewed checkouts: HowlBoard `135cdf79` against `f4c7bb5`; HowlFrame PR #211 `9dbd8175` against main `bfc4ff8f`; independently built Board compiler pin `88e49597`. No application source was modified. Reviewer scripts, inputs, artifacts and logs are in `review-evidence/`. The supplied implementer outputs were treated as claims, not verification.

**Commands and environment**

All paths below are relative to this directory. Go commands ran offline (`GOPROXY=off`, `GOTOOLCHAIN=local`), with `GOCACHE` pointing to `howlframe-board-pin/review-evidence/go-cache` and temporary directories under `review-evidence/tmp`. These environment assignments are omitted from later command listings for readability.

```bash
mkdir -p review-evidence/tmp
(cd howlframe-board-pin && GOCACHE="$PWD/review-evidence/go-cache" \
  GOPROXY=off GOTOOLCHAIN=local go build -o ../hf-pin howlframe.go)
cp hf-pin howlboard/howlframe_bin
git -C howlboard show f4c7bb5:tools/ledger_import/ledger_import.howl > review-evidence/old_import.howl
./hf-pin -compile-bc review-evidence/old_import.howl -o review-evidence/old_import.hfbc
./hf-pin -compile-bc howlboard/tools/ledger_import/ledger_import.howl -o review-evidence/new_import.hfbc
python3 review-evidence/probe.py > review-evidence/probe.jsonl
python3 review-evidence/desk_probe.py > review-evidence/desk-probe.jsonl
```

Builds exited 0. `probe.py` independently seeds prior stores, runs old/new importers in separate directories, compares actual file bytes, and records hashes, exits and diagnostics. Both use `-allow-caps filesystem,database -max-instructions 200000000 -max-memory-bytes 268435456`. This controls the resource-budget difference when comparing behavior; it does not imply the old Makefile used 256 MiB.

**Numbered claims**

1. **UNVERIFIED end to end; source/build/import portion CONFIRMED.** Commands:

   ```bash
   cat howlboard/Makefile
   sed -n '70,180p' howlboard/tools/ledger_import/ledger_import.howl
   rg -n 'ledger|store_open|api/missions|api/timeline' howlboard/backend/server.howl
   make -C howlboard import > review-evidence/make-import.log 2>&1
   timeout 10 make -C howlboard run > review-evidence/make-run.log 2>&1
   (cd howlboard/backend && go test -count=1 -v -run '^TestLedgerImportServedByBoard$' .)
   ```

   `make import` compiles the actual importer, which imports `evidence_ledger.howl` and calls `ledger/parse_within` before projection and store writes. Its run exited 0 with `entries=72`, `missions=16`. The server routes read those same mission/timeline files. `make run` compiled and attempted the real server, then failed with `HTTP_SERVER_SERVE: http listen failed: listen tcp :8080: socket: operation not permitted`. The dedicated serving test likewise failed at the socket precheck after importing. HTTP responses could not be observed. See `make-run.log` and `served-test.log`; the test's generic “port ... already in use” wording does not describe the underlying permission error.

2. **CONFIRMED.** Commands:

   ```bash
   cmp howlboard/tools/ledger_import/evidence_ledger.howl howlframe-pr211/milestone3/evidencedesk/app/ledger.howl
   sha256sum howlboard/tools/ledger_import/evidence_ledger.howl howlframe-pr211/milestone3/evidencedesk/app/ledger.howl
   ```

   `cmp` exited 0. Both SHA-256 values are `aacde2501430bf7b34a05a36e3de468bc667af757a37a78cd5328c8498e52d55`, matching `VENDORED.md`.

3. **CONFIRMED for jointly accepted inputs tested, with the compatibility qualification in finding 1.** Commands: the independent old-source extraction, bytecode builds and `python3 review-evidence/probe.py` above; also:

   ```bash
   git -C howlboard diff -w f4c7bb5..HEAD -- tools/ledger_import/ledger_import.howl
   ```

   Both stores are byte-identical for the 72-entry sanitized fixture (`entries=72`, `missions=16`), its CRLF variant, Unicode IDs, 120-character Unicode task IDs, a 2 MiB single record, and 2,000 distinct entries sharing one task (`entries=2000`, `missions=1`). Comparison includes prior stored data. The projection body is unchanged except for the documented handling of absent/non-object metadata. This does not establish equivalence for every input the old importer accepted: IDs over 120 characters are newly rejected, and metadata inputs the old importer crashed on now succeed.

4. **REFUTED.** Commands:

   ```bash
   git -C howlboard diff f4c7bb5..HEAD -- README.md change_log.md
   cat howlboard/tools/ledger_import/VENDORED.md
   git -C howlboard log -1 --format=full
   rg -n '120|identifier|UTF-8|utf8' howlframe-pr211/milestone3/evidencedesk/requirements/*.md howlboard/README.md howlboard/change_log.md howlboard/tools/ledger_import/VENDORED.md
   python3 review-evidence/probe.py
   ```

   The three IDs have a new 120-character ceiling, absent from the Board README, changelog, vendoring summary and commit description. Old imports with 121-character IDs succeed; new imports fail at line 1. EvidenceDesk explicitly requires this limit in its own `requirements/REQUIREMENTS.md`; that does not document a Board compatibility change. The supplied files do not include the actual GitHub PR bodies, so their exact wording/documentation coverage remains unverified.

5. **REFUTED as an unconditional failure guarantee; CONFIRMED for tested input-validation rejections.** Commands:

   ```bash
   python3 review-evidence/probe.py > review-evidence/probe.jsonl
   (cd howlboard/backend && go test -count=1 -v ./... -run 'Test(LedgerImport(Truth|MatchesRecordedProjection|RejectsInvalidLedgerWithoutWriting|EntryLimitAndOptionalMetadata|DeniedCapabilities)|VendoredLedgerModuleUnchanged)$')
   ```

   All six selected top-level tests passed (`importer-tests.log`). Malformed/truncated JSON, nonobjects, wrong schema, missing/empty IDs, duplicate IDs, invalid UTF-8, empty/blank files, excess entries/bytes, invalid MAX and unreadable paths preserved prior stores byte-for-byte in these tests/probes. However, valid input with a corrupt preexisting timeline exits 1 after changing the missions file; both otherwise valid stores also diverge when the timeline write is denied by a scoped grant. These are persistence failures, not validation rejections, and the corrupt-timeline failure also occurs in the old importer. See finding 2.

6. **CONFIRMED; I agree that the added flag is a resource budget, not a grant.** Commands:

   ```bash
   git -C howlboard diff f4c7bb5..HEAD -- Makefile
   ./hf-pin inspect --caps review-evidence/old_import.hfbc
   ./hf-pin inspect --caps review-evidence/new_import.hfbc
   sed -n '1050,1105p' howlframe-board-pin/howlframe.go
   ```

   Both artifact inspections report `capabilities:["database","filesystem"]`. Import grants remain `filesystem,database`; run grants remain `network,database,filesystem,environment` plus the two existing GitHub fetch scopes. `resourcePolicy` assigns the memory value to `Limits.MaxMemoryBytes`, independently of capability grants. The VM's cumulative allocation budget rises from the default 64 MiB to 256 MiB; this is a real resource-policy expansion, with no permission expansion.

7. **CONFIRMED.** Command:

   ```bash
   git -C howlboard diff f4c7bb5..HEAD -- backend/ledger_import_test.go backend/contract_test.go
   ```

   `ledger_import_test.go` only adds the `strconv` import, the `EntryID` fixture field, and `"truth-" + strconv.Itoa(index)` assignment. No assertion changes. The vendored-module test actually lives in the new `ledger_import_validation_test.go`. `contract_test.go` extracts server launching into a helper while retaining existing store cleanup and assertions. `TestLedgerImportTruth` passed independently.

8. **CONFIRMED for the changed parser and executable projector paths.** Commands:

   ```bash
   git -C howlframe-pr211 diff bfc4ff8f..HEAD
   (cd howlframe-pr211 && go test -count=1 -v -run '^TestEvidenceDeskLedgerParseWithin$' .)
   python3 review-evidence/desk_probe.py > review-evidence/desk-probe.jsonl
   ```

   The new test passed. The independent probe compiles the actual main and PR modules/projectors with the same pinned compiler: all 27 cases have identical parse results, projector exits/stdout/stderr and store bytes. Exact 262143- and 262144-byte valid inputs succeed. At 262145 bytes both return `payload_too_large`, line 0, message `Ledger exceeds 262144 bytes`; both projectors exit 3. Only the ledger module and documentation/tests change; backend and projector source behavior is otherwise untouched. The full HTTP/browser workflow suite was not rerun because sockets are prohibited here.

9. **UNVERIFIED overall; clean compilation and `make import` CONFIRMED.** Commands:

   ```bash
   cp hf-pin howlboard/howlframe_bin
   make -C howlboard test > review-evidence/make-test.log 2>&1
   make -C howlboard import > review-evidence/make-import.log 2>&1
   ```

   All build steps succeeded: frontend, backend bytecode, importer bytecode and showcase. `make test` exited 2: the first HTTP test cannot bind localhost, and `httptest.NewServer` then panics on another prohibited socket. This is an environment limitation, not evidence of an application regression. `make import` exited 0 (`entries=72`, `missions=16`). The six importer-only tests and the PR #211 module test passed separately.

**Defects, ranked by severity**

1. **P2 — Undocumented rejection of previously supported IDs over 120 characters.** Location: `howlboard/tools/ledger_import/evidence_ledger.howl:76`; public contract: `howlboard/README.md:136` and the new changelog entry.

   Minimal input: one object with schema `ai.evidence_entry/v1`, `entry_id` consisting of 121 `e` characters, `task_id:"T"`, `agent_id:"agent"`, `action:"task_created"`, and `metadata:{}`. The independent harness writes this as `review-evidence/cases/entry_id_121.jsonl`.

   Minimal rerun using that retained input:

   ```bash
   make -C howlboard import LEDGER="$PWD/review-evidence/cases/entry_id_121.jsonl" MAX=2000
   ```

   The bytecode process exits 3 (GNU make exits 2): `invalid_ledger at line 1: entry_id exceeds 120 characters; nothing was imported`. See `review-evidence/id-limit-repro.log`. The independently rebuilt old importer exits 0 with `entries=1`, `missions=1` on the same file. The same change affects `task_id`, `agent_id`, and 121-character Unicode IDs; a 121-character action remains accepted. These inputs satisfy the published Board validation description. The upstream HowlPlane JSON Schema was not supplied, so this finding does not claim full schema validity. Document the new restriction and its compatibility impact, or deliberately preserve Board's previous accepted range. Add regression coverage for the chosen contract.

2. **P2 — “On every failure the existing stores are byte-identical” overstates the implementation.** Location: `howlboard/change_log.md:6`; writes: `howlboard/tools/ledger_import/ledger_import.howl:324-329`. The implementation commits missions individually before opening/writing the timeline.

   Minimal reproduction with the retained valid one-entry input:

   ```bash
   mkdir -p review-evidence/minimal-write-failure
   python3 - <<'PY'
   from pathlib import Path
   import subprocess
   root = Path.cwd()
   work = root / 'review-evidence/minimal-write-failure'
   missions = work / 'howlboard_missions.json'
   timeline = work / 'howlboard_timeline.json'
   missions.write_text('{}')
   timeline.write_text('not-json')
   before = missions.read_bytes()
   p = subprocess.run([str(root / 'hf-pin'), '-run-bc', '-allow-caps',
                       'filesystem,database',
                       str(root / 'review-evidence/new_import.hfbc'),
                       str(root / 'review-evidence/cases/max_007.jsonl'), '10'],
                      cwd=work, capture_output=True, text=True)
   print(p.returncode, p.stderr.strip())
   print('missions_changed=', missions.read_bytes() != before)
   PY
   ```

   Result: exit 1, `STORE_INVALID_PERSISTED_JSON` on the timeline, `missions_changed=True` (`review-evidence/minimal-write-failure.log`). The larger harness independently confirms this for both old and new importers and also confirms partial mutation with initially valid files plus a denied timeline write. This is a preexisting persistence limitation, newly obscured by the absolute documentation promise. Narrow the promise to input-validation rejection; transactional persistence would be a separate change.

**Additional edge-case results and limits**

| Input | Observed new behavior |
| --- | --- |
| CRLF, ordinary Unicode IDs, 120 Unicode characters | Success; both stores byte-identical to old |
| UTF-8 BOM | Exit 3, malformed JSON at line 1; prior stores unchanged |
| Invalid UTF-8 inside an otherwise parseable string | Exit 3, `Ledger must be UTF-8`; unchanged |
| `MAX=007`, `+7`, `" 7 "` | Accepted; same output as old |
| `MAX=1e3`, integer overflow | Exit 2 usage; unchanged; old exits 1 conversion error |
| Ledger path is a directory | Exit 3, `Failed to read ledger file`; unchanged |
| Duplicate entry ID separated by blank lines | Exit 3, duplicate at physical line 4; unchanged |
| 2,000 entries sharing one task, with artifacts | Success; one projected mission, matching stores |
| 2,000 verification-heavy entries sharing one task | Both old/new hit 256 MiB cumulative allocation limit, exit 1; unchanged |
| 2 MiB single JSON line | Success; matching stores |
| Single record over 3 MiB | Exit 3 with 3145728-byte ceiling diagnostic; unchanged |

The repeated-verification limit is a preexisting resource sensitivity, not evidence of a new projection regression. It does qualify broad “a 3 MiB slice fits” language: byte size and entry count alone do not guarantee success.

A preexisting documentation discrepancy is also a sync candidate: `howlframe-board-pin/internal/ast/desugar_corelib.go` describes `str_len` as byte length, but the live Unicode probe accepts 120 three-byte characters and rejects 121. This report follows the executable character-count behavior.

**Overstated descriptions and evidence gaps**

- The supplied blanket compatibility claim needs the accepted-input qualification and an explicit ID-length change. The Board README/changelog are incomplete on this point.
- The changelog's “On every failure” statement should say validation failures. No store-wide transaction exists.
- Actual GitHub PR descriptions were not included; these observations apply to `REVIEW_BRIEF.md`, the local commit descriptions and repository documentation, not an unseen PR body.
- The supplied passing HTTP/workflow logs were not accepted as independent proof. Local socket denial leaves claims 1 and 9 partially unverified. No network publication, GitHub comment, or out-of-directory mutation was performed.
