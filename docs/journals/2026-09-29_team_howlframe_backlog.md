# Team HowlFrame backlog

## Source

HowlFutureWorks engineering-manager synthesis of a five-seat poll on
2026-09-29 ET.

Seats: Dev Lead Motoko, Product Quatre, R&D Hange, Assurance Lain, Auditor
Heinrich.

This journal is the durable copy of that synthesis. It files backlog rows. It
does not implement them.

## Context

`map_keys` landed on main as pull request #43 (`d1616a0`). It returns a
lexicographically sorted key list, an empty dict is an empty list, a non-dict
fails closed with `TYPE_ERROR`, and the opcode grants no capability
(`OpMapKeys` has an empty capability field). The poll treats `map_keys` as
Done. Product asks only that HowlBoard verify the consumer. HowlFrame does
not reopen the opcode.

HowlBoard pull request #9, "Project Factory status and preview exact Pending
rows" (https://github.com/howlcipher/howlboard/pull/9), merged the same day
and is separate Board work. A paste preview of Pending rows is not admission.

## Near-term consensus

Implementation sequence from the synthesis. The Ranked Backlog in
`improvements.md` still sorts by Value × Decay ÷ Effort, so a cheap contract
lock can outscore a larger feature. The sequence below is the agreed build
order.

1. HTTP request surface: query string, path params, and `req_header`. A
   fail-closed small set. No general router rewrite. App-facing. `req_method`
   and response `res_header` already exist; query, path segments, and request
   headers do not (`apps/task_api/DEVELOPMENT_NOTES.md`). Filed as #102.
2. One absence idiom: document and lock `map_get` missing → `""` versus
   `store_get` missing → nil across the VM, Go, and JS. The bytecode VM and
   the interpreter already push `""` for a missing `map_get`. `store_get`
   pushes nil and is the missing-record sentinel. Go `map[string]any` indexing
   still yields nil. Filed as #103.
3. Chained `map_get`: path access without a pile of `let`s. Pure. No new
   capability. Today `map_get` requires a symbol for the dict, so a nested
   call is rejected. Filed as #104.
4. `html_escape` (and a cheap attribute helper if it falls out of the same
   work) plus structured encode, so an untrusted id is not concatenated into
   handler JavaScript. Pair it with a contract-test harness for escape,
   data-attribute, and constant-handler invariants. `set_html` is still
   `innerHTML` (`docs/howlboard_xss_set_html_issue.md`). Filed as #105.
   Assurance's release-blocker contract tests for those invariants live on
   this row, not as a second item.

## Platform goals

High rank. Not the next implementation pull request.

5. Bytecode modules. Design spike and ROI proof only. Do not rush a half
   linker. Do not smuggle HFIR module linking into a quick patch. Improvement
   #95 stays Done: it closed compile-time AST linking for `use` / `export` /
   `module` and explicitly rejected runtime module opcodes. The bytecode-tier
   ceiling is a new Pending, #106, scoped to the spike.
6. Capability surface honesty. Keep pure operations versus
   `store_keys` = `database` explicit in docs and tests. `map_keys` is pure.
   `store_keys` stays `database`, and a `file://` store additionally requires
   `filesystem`. Filed as #107. Improvements #79 and #94 stay Done.
7. Fail-closed `TYPE_ERROR` completeness on the other dict and list operations
   that still soft-fail. `map_keys` already fails closed on a non-dict. JS
   `map_get` / `map_set` / `append` do not use that guard, and Go `map_get` is
   still raw indexing. Filed as #108.

## Parallel tracks

Recorded here. New rows only where the backlog did not already have one.

* Assurance. After `map_keys`, dict enumeration still needs a non-BMP sort
  check. The seat ask was Wasm↔Go parity. `map_keys` today is the VM, the Go
  backend, and JavaScript; the Wasm SSA backend does not emit it yet (#73).
  Go orders keys with byte-wise `sort.Strings` (and the VM uses Go string
  `<`). JavaScript uses `Array.prototype.sort`, which compares UTF-16 code
  units. Ordinary keys match; supplementary-plane keys may not. Filed as
  #109 for the backends that already emit `map_keys`, and #73 must use that
  order if it grows a Wasm `map_keys`. The escape/handler contract harness
  is part of #105, and Assurance treats those tests as release blockers.
* Auditor and Board. Verifiable envelopes are mostly Board and ChangeOps.
  This poll does not invent HowlFrame opcodes for Board authority.
* R&D. HFIR stays an ahead-of-time verifier. The production path remains
  AST → bytecode. Gate Wasm expansion on the lowered-HFIR ABI (#90) before
  the thin backend grows. Prefer the existing rows: #88 (adapter protocol),
  #90 (lowered ABI), #92 (Wasm binary pipeline). No duplicate rows.
* Product, honorable. Date and time primitives. Unix `time_now` already
  ships on the VM, Go, and JS backends. A richer calendar or formatting
  surface was not in the near-term sequence, so it is not filed.

## Hard nos

Unanimous. These are constraints on later work, not backlog rows.

* No new capabilities for pure data operations.
* Store enumeration stays `database`, plus `filesystem` for `file://`.
* No Frame or Board path that starts Factory, takes the supervisor lock,
  writes a queue, or trusts a remote `owner_direction`.
* No second Factory.
* No authority or envelope bypass offered as a developer-experience shortcut.
* No raising HowlFutureWorks context budgets to paper over Frame gaps.
* A Board paste preview is not admission.

## Seat citations

* Motoko: HTTP, then chained `map_get`, then `html_escape`, then absence,
  then modules as a goal rather than the next pull request.
* Quatre: bytecode modules first, then verify `map_keys` from HowlBoard,
  then absence, chained `map_get`, and `escape_html`; after that, query/path
  and datetime.
* Heinrich: envelopes, then capability honesty, then bytecode modules, then
  absence, then `escape_html`.
* Lain: structured encode, then Wasm/Go parity, then `TYPE_ERROR`
  completeness, then modules, then the contract harness.
* Hange: modules as a design spike, HFIR stays experimental, Wasm stays thin,
  HTTP as a small proof, and the #90 ABI gates backend growth.

## What was filed

| # | Row | Score | Role |
| --- | --- | --- | --- |
| 102 | HTTP request surface | 2.00 (8×1÷4) | Near-term |
| 103 | One absence idiom | 2.33 (7×1÷3) | Near-term |
| 104 | Chained `map_get` | 2.00 (6×1÷3) | Near-term |
| 105 | `html_escape` and handler contract tests | 1.75 (7×1÷4) | Near-term |
| 106 | Bytecode modules design spike | 1.50 (6×1÷4) | Platform, not next |
| 107 | Capability surface honesty | 1.67 (5×1÷3) | Platform, not next |
| 108 | `TYPE_ERROR` on remaining dict/list ops | 1.50 (6×1÷4) | Platform, not next |
| 109 | Dict key sort parity, including non-BMP | 1.33 (4×1÷3) | Assurance parallel |

#103 scores above #102 because the formula rewards the smaller effort. The
build order in "Near-term consensus" is unchanged.

## What this does not do

No compiler, runtime, capability-grant, Factory, HowlPlane, or HowlBoard
code changes. #95 is not reopened. `map_keys` is not reopened. Envelopes are
not given Frame opcodes. Datetime beyond `time_now` is not filed.
