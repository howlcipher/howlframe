# External Consumer v0.1.1 Journal

## Scope

Consumer-driven HowlFrame conformance investigation for HowlBoard and
HowlChangeOps. This journal records executable evidence only.

## System snapshot

- Date: 2026-08-13 UTC.
- Remote: `https://github.com/howlcipher/howlframe.git`.
- Default branch: `main`.
- Verified `origin/main` SHA: `628066d2e86ae7991cd491b0832547ff55123725`.
- GitHub’s latest `main` CI at that SHA was successful (run 31713847419).
- Open pull requests: none.
- Latest public release: `v0.1.0` (published 2026-08-13).
- Candidate source worktree branch: `feat/consumer-driven-v011`, based on
  the verified `origin/main` SHA.
- HFBC format: `1` (`howlframe.go:29`).

## Candidate

- Candidate path: `/tmp/howlframe-candidate`.
- Built from: `628066d2e86ae7991cd491b0832547ff55123725`.
- `howlframe version`: `HowlFrame 0.1.0`; `HFBC format: 1`.
- SHA-256:
  `407978c986eaf4f6a9fc6eaf5f3f84d58dbaa16719a44a818b2bd4473af743e6`.

## Baseline validation

All Phase 1 commands completed successfully from the verified source:

- `gofmt -l .`
- `go build ./...`
- `go vet ./...`
- `go test ./...`
- `go test -race ./...`
- `python3 -m unittest benchmarks/v2/harness/test_harness.py`
- `go run tools/difftest/main.go`
- `go run ./cmd/codegen`
- `git diff --check`

The benchmark harness intentionally exercises failing candidate fixtures as
part of its retry/negative-case tests; its final result was `OK` (8 tests).

## Request-body investigation

The initially suspected missing primitive is not a confirmed capability gap.
There is no `req_body` construct in the construct registry, capability map,
opcode registry, bytecode reference, or generated construct coverage. But
the current bytecode compiler has a supported `parse_json` opcode whose
string operand is the source variable name. The VM special-cases precisely
`req.body`: it obtains the route request from the environment, calls
`io.ReadAll(req.Body)`, restores `req.Body`, and unmarshals JSON.

Independent public-CLI reproduction used `/tmp/howlframe-req-body-repro.howl`:

```howl
(http_server 18091
  (route "/echo" (lambda (req)
    (try_let (body (parse_json Body req.body))
      (catch err (res_json 400 (dict ("error" "invalid_json"))))
      (res_json 200 (dict ("title" (map_get body "title"))))
    ))))
```

The candidate accepted `check`, built an HFBC artifact, and served it with
`run --allow-caps network`. A `POST` of `{"title":"Ship v0.1.1"}` returned
`200 {"title":"Ship v0.1.1"}`; malformed JSON returned
`400 {"error":"invalid_json"}`.

This path is usable by HowlBoard today, so adding `req_body` merely for
availability is not justified. It has a real safety/conformance deficiency:
the VM uses unbounded `io.ReadAll`, ignores its read error, and has no
centralized documented request-body limit. The Go backend instead passes the
raw body to `json.NewDecoder`, which also has no equivalent enforced limit.
No runtime modification has been made pending coordinator review.

## Conformance and frontend CLI findings

The minimal conformance slice adds a focused standalone VM test for a JSON
route body, `try_let`'s malformed-input catch branch, and object/list values
returned through `res_json`. Existing Task API integration tests cover the
same composition through a long-lived real HTTP server and state mutations.
The JavaScript generator has focused `parse_json` and optional-body `fetch`
tests because the HowlBoard frontend uses `fetch`; it does not have inbound
route request parity.

`howlframe build frontend.howl` correctly rejects a `web_app` root with
`HFIR_TARGET_INFEASIBLE`, because that command intentionally emits HFBC only.
The existing compatibility source invocation, `howlframe -o build
frontend.howl`, generates `app.js`; the current HowlBoard frontend generated
successfully and passed `node --check`. This is a documented public
compatibility route, so it is not a release-blocking platform gap. The
subcommand CLI does not yet offer a corresponding JavaScript-build command.
