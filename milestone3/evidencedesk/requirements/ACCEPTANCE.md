# Independent acceptance protocol

Frozen 2026-10-20 by milestone3_requirements before implementation. Requirements
are authoritative; tests are falsification aids, not a replacement for R1-R6.
No implementation-dependent baseline is imported by these tests.

1. Build compiler and application from committed sources in a fresh temp checkout,
   empty HOME/TMPDIR/Go caches using host Go, Python, Bash and Git. Record network
   module-download prerequisites and browser dependencies honestly.
2. Start fresh server with explicit grants; run tests/acceptance.py with
   EVIDENCEDESK_URL and PROJECTOR_ARGV (JSON argv prefix for scripts/project.sh).
   All tests must execute, with no skips. Repeat server restart after importing
   review.jsonl and saving a note; confirm exact events and note survive restart.
3. Real browser test using installed Playwright + Chromium (available local cache):
   open /; paste review.jsonl into #ledger-input; click #import-button; expect
   counts and two tasks; type REVIEW-1 in #task-filter; select it; inspect ordered
   events and literal claimed; save note and reload; confirm note; clear filter;
   load REVIEW-2; import malformed line 2 and verify useful line error with prior
   list retained. Import a result containing <img src=x onerror="window.pwned=1">;
   assert literal text visible and window.pwned absent. Exercise keyboard controls
   and labels, and inspect no uncaught JS errors. A DOM mock is insufficient.
4. Actual ecosystem consumer (mandatory, no fixture-only replacement): compile
   sibling howlboard/backend/server.howl; run projector on review.jsonl into empty
   temp dir; launch sibling Board bytecode in that cwd; GET /api/missions and
   POST /api/missions/get {"id":"REVIEW-1"}; assert provenance LEDGER,
   origin test_fixture, state VERIFYING, verification status claimed, and two
   ordered timeline events. REVIEW-2 state COMPLETED. Repeat with sibling's
   committed sanitized data/fixtures/ledger_sample.jsonl (72 records), asserting
   source task IDs are queryable and total timeline count matches input. Record
   sibling commit/hash and commands; never read live/private telemetry. Stop
   Board before touching stores. No sibling modification is needed.
5. Deny serving/read/write/environment grants individually where used and record
   predictable failure without persisted corruption; bound runaway work using
   an instruction budget and show a too-small budget is enforced. Verify process
   and remote-fetch grants are unnecessary. Verify loopback listen address live.
6. Review sources for real .howl domain and interaction logic, native bytecode,
   independently compiled frontend, minimal capabilities and honest limitations.
   Re-run repo CI appropriate to any core fixes; keep regression reproductions.
7. Independent final reviewer signs requirement-by-requirement decision with
   evidence index, known limits, evaluator role and freeze commit. Retain failures.
   Acceptance is pending until all above pass; no cross-provider proof is claimed.

Tests do not automate lifecycle/restart/real sibling/browser checks themselves;
those mandatory checks require executable scripts and retained output authored
by implementation and independently rerun during final review. This distinction
must remain visible in the final evidence record.
