# 2026-10-12: Held-out correctness fixes

Base: main `c21a029c` (#136 merged). This slice fixes the four correctness
bugs the held-out benchmark (heldout-v0) surfaced. No new built-ins. No token
claims. No production `-compile-bc`/HFIR flip.

## 1. Nested shorthand under the governed profile

**Bug:** shorthand (`str_replace`, `read_text`, `str`, ...) nested inside the
argument of a value-taking built-in (`write_file`, `to_lower`, ...) was not
expanded. The governed build then failed with "no lowering for construct".

**Fix:** `internal/ast/desugar.go` `isSugarExprContext` now lists every
value-taking built-in as a parent. These include write_file, read_file,
to_lower/to_upper, str_trim, the escapes, regex_match, is_nil, map_keys, env,
exec, store_*, the req_*/res_* readers, llm_generate and others. A new drift
test (`desugar_context_drift_test.go`) walks the construct registry. Every
supported, non-opaque construct must expand nested sugar unless it is listed as
structural, with a written reason.

**Fail-closed is unchanged:** the order is still pre-desugar profile scan,
then desugar, then post-desugar `hfir.VerifyProfile`, then build. A forbidden
construct inside a nested shorthand argument is rejected before any artifact
exists, and nothing runs. `TestHeldoutNestedShorthandGovernedFailsClosed`
covers this: an `exec` hidden in `str_replace` inside `write_file` yields no
artifact and no side file.

## 2. Short-circuit `and` / `or`

**Bug:** both operands were always evaluated.

**Fix:**
- **AST interpreter:** `evalBinop` decides before it evaluates the right side.
- **AST bytecode:** `bytecode.ShortCircuit` emits a JUMP_IF_FALSE guard around
  the right operand. The HFIR bytecode lowering uses the same helper.
- **Go/JS backends:** they already used native `&&`/`||`.

The right operand of `and` runs only when the left is true, and the right of
`or` only when the left is false. Effects (`write_file`) and errors
(`to_int "qq"`) on the skipped side do not happen.

## 3. `map_get` on a missing key returns nil (behavior change)

**Before:** a miss returned `""`, so `is_nil` and `nz` defaults never fired on a
miss.

**Now:** a miss returns nil on every path:
- **VM paths:** interpreter, bytecode and HFIR bytecode.
- **JS:** returns `null`.
- **Go:** returns nil for dynamic maps.

Text rendering is kept stable. `vm.TextOf` renders nil as `""` in `print`,
`stderr`, `to_string` and `str_join`. The Go/JS backends render nil/null as `""`
in `print` and `to_string`. So `(print (map_get m "zz"))` still prints an empty
field. `is_nil`, `nz` and `=` now tell a miss (`nil`) apart from a present `""`:
`(= (map_get m "zz") "")` is now **false**.

Other consequences:
- A chained `map_get` through a missing intermediate is still `TYPE_ERROR`,
  now "expected dict, got nil".
- A missing `store_get` record now prints as an empty field instead of `<nil>`.
  `is_nil` is the way to observe absence.

**Audit and migration:**
- `apps/task_api`: the title/id validation compared with `""`. It now uses
  `(or (is_nil x) (= x ""))`, so it still rejects both a missing and an empty
  value. Short-circuit makes this safe.
- `apps/release_authority`: it already had `is_nil` fallbacks to `"unknown"`,
  which were dead code before. They now fire. The test expects `"unknown"` when
  a proposal omits `target`.
- The vm/gogen/js absence and path tests were renamed `...MissIsNil` and their
  expectations updated.
- The lowered-HFIR ABI conformance fixture `map_get_absence` now expects
  `is_nil` of a miss to be `true`.
- A grep of all `.howl` for `(= (map_get …) "")` found nothing else.

**Go typed-map caveat:** a Go `map[string]string` cannot hold nil, so a value
read of a miss is still `""`. `is_nil (map_get m k)` on a named map is compiled
as a key-presence check (`howlFrameMapGetIsNil`), so it agrees with the VM.
`nz (map_get m k) d` on a typed string map in the Go target may still see `""`.

## 4. Lists are values (decision)

**Decision:** explicit value semantics, applied the same way on every target.
- `append` rebinds only the named variable to a new list.
- A list stored in a map, or bound to another name, is unchanged.
- To update the map, `map_set` the list back.
- Dicts keep their in-place `map_set` mutation, which is unchanged.

The VM paths already copied on append. The bug was that the backends did not:
- **JS:** used `push`, which aliased. `howlFrameAppend` now returns a copy and
  the emitted code is `xs = howlFrameAppend(xs, item)`.
- **Go:** `append` could write into a shared backing array. It now uses a full
  slice expression, `append(xs[:len(xs):len(xs)], item)`, in both the direct
  form and the helper.

## Regression tests

- `heldout_correctness_cli_test.go`: each bug runs on the AST interpreter
  (`-run`), governed AST bytecode (`build --profile governed` then `-run-bc`)
  and HFIR bytecode (`-compile-hfir-bc` then `-run-bc`). It includes the
  effect-skipping short-circuit test with `write_file` on the skipped side.
- `internal/backend/gogen/heldout_semantics_test.go` and
  `internal/backend/javascript/heldout_semantics_test.go` cover list aliasing,
  nil rendering, `is_nil` of a miss and short-circuit. Go dict literals are
  string-valued, so the list-in-map case is covered on the VM and JS paths.
- `desugar_context_drift_test.go` keeps the shorthand parent list complete.

## Benchmarks (verification only; no claims)

- **token-efficiency-v0 `run.sh`:** howl 60/60, python 60/60, js 60/60, go 60/60.
- **token-efficiency-heldout-v0 `run.sh`:** exit 0. All 126 references verified
  (howl 30, plus 32 each for python, js and go), with the same 2 recorded gaps
  (h31, h32). The committed results files are unchanged.
- **h05/h07/h12 natural forms:** rewritten in a scratch run with nested
  `str_replace (read_text …)`, `or` with `to_int` on the right, and
  `nz (map_get …) 0`. All three pass the verifier. They were **not** committed,
  so the frozen solution hashes and the #136 report stay as they are.

## Known gaps

- The JS backend's `nz` lowering emits `let` in expression position, which is
  a syntax error. This bug predates this slice and is untouched here.
- Go dict literals cannot hold lists or ints (string-valued only). This also
  predates this slice.
