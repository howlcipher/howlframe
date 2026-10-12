# 2026-10-12 — Issue #143: bytecode carries the fetch body

**Criteria change approved by the Owner (William Elias).** The prod-flip
criteria lock and the ABI conformance tests used to pin "the bytecode
fetch body is skipped". They now pin "the body is sent".

## What changed
- The AST bytecode compiler (`internal/bytecode`) and the HFIR bytecode
  lowerer (`internal/hfir`) both compile `(fetch url method body)` with the
  new shared helper `bytecode.FetchWithBody`:
  `url, method, body, CONVERT to_string, HTTP_REQUEST 1, LOAD_CONST "body", MAP_GET`.
  - This adds no new opcode, no new capability and no artifact-format
    change. HTTP_REQUEST already carries a body and already needs `network`.
  - The result is the response body bytes, the same value FETCH pushes.
  - Semantics match the interpreter: the body is stringified, so `42` is
    sent as `"42"`, and the method and URL are unchanged.
  - A fetch without a body still compiles to the bare FETCH, so existing
    artifacts and programs are unchanged.
- Tests updated to the new behavior:
  - `TestProdFlipCriteriaLock/fetch_body` now requires both compilers to emit
    HTTP_REQUEST rather than a body-less FETCH, and to keep the body string.
  - The hfir unit test `TestLowerToBytecodeFetchSkipsBodyOperand` is replaced
    by `...SendsBodyOperand`, which locks the exact instruction sequence.
  - `TestHFIRBytecodeFetchOperandOrder`: the PUT body is now received (2
    bodies, one per compiler), and the operand shapes include the body.
  - `nested_multi_effect_*` (fixture `21_nested_multi_effect.howl`, difftest
    and VM tests): the only body fetch sits in a dead `while false` branch.
    - Its marker is renamed from `phase2d-body-not-sent` to
      `phase2d-dead-branch-body`.
    - Both compilers must now load that body (they no longer drop it).
    - The server still sees no body, because the branch never runs.
- New regression test `TestIssue143FetchBodyEveryPath` uses a local
  httptest server only.
  - It covers the interpreter (`-run`), AST bytecode (`build` + `-run-bc`)
    and HFIR bytecode (`-compile-hfir-bc` + `-run-bc`).
  - The cases are a POST with a string body, a PUT with a numeric body, and
    a GET with no body. All three paths must produce identical stdout and
    identical requests.
  - With the network grant denied, the bytecode paths must open no
    connection and send no body.

## Caveats
- A transport failure on a fetch with a body reports `http_request failed` /
  `http_request body exceeds maximum...` instead of `fetch failed`. The
  code (IO_ERROR or LIMIT_EXCEEDED) is the same; only the opcode and message
  text differ. The receipt records HTTP_REQUEST for these calls.
- The body goes through CONVERT to_string. Strings and numbers match the
  interpreter's `fmt.Sprint`. Other values, such as lists, dicts and bytes,
  are stringified by the VM's own to_string, which may not format them the
  way Go's `fmt.Sprint` does.
- Production `-compile-bc`/HFIR is not flipped. #90 stays Partial. No tip
  lock, no live LLM, no DOM, no external network. Hange's
  `experiments/token-efficiency-heldout-v2` is untouched.
