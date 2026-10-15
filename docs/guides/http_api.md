# Building an HTTP API in HowlFrame

This guide builds a small JSON API on the standalone VM and shows the patterns that a real backend
needs: routing, JSON bodies, authentication, validation, one error shape, limits, logging and running
the server with minimal grants. The complete worked example is
[`milestone2/ticketdesk/`](../../milestone2/ticketdesk/): an issue tracker with roles, persistence and a
101-test acceptance suite. Every `lisp check` block below is a complete program that the repository's
tests compile; every `lisp fragment` block is checked inside a wrapper.

## Routing

A server is `(http_server PORT ...)` with one `route` per path. A path segment written `{name}` captures
one whole segment, read with `req_path`. A path with no braces is exact, and an exact route wins over a
pattern. There is no method routing: a handler looks at `req_method` and answers `405` with an `Allow`
header itself. A route written `"*"` is the fallback that receives every request no other route matched, so
the API can answer unknown paths with its own JSON instead of a plain-text 404.

```lisp check
(http_server 18081
  (route "/v1/health" (lambda (req) (res_json 200 (dict ("status" "ok")))))
  (route "/v1/items/{id}" (lambda (req)
    (if (= (req_method req) "GET")
      (res_json 200 (dict ("id" (req_path req "id"))))
      (do
        (res_header "Allow" "GET")
        (res_json 405 (dict ("error" "method_not_allowed")))))))
  (route "*" (lambda (req)
    (res_json 404 (dict ("error" "not_found") ("path" (req_url_path req)))))))
```

The port may also be an expression, for example `(http_server (to_int (env "PORT")) ...)`. Every response
carries an `X-Request-Id` header chosen by the server, and `(req_id req)` returns the same value.

## JSON bodies

`(req_body req)` returns the request body as a string, bounded by `--max-request-bytes`; a larger body raises a catchable `LIMIT_EXCEEDED` (answer `413`, as the second example does). Check
`(utf8_valid body)` first, then parse with `parse_json`, which needs a variable name and a type hint.
Inside a `try_let` a malformed body becomes an ordinary `catch` branch. Types are not coerced: classify a
value yourself, for example by the first character of `(encode_json value)`.

```lisp check
(http_server 18082
  (route "/v1/echo" (lambda (req)
    (try_let (doc (parse_json Any req.body))
      (catch err (res_json 400 (dict ("error" "invalid_json"))))
      (res_json 200 (dict ("keys" (map_keys doc)))))))
  (route "/v1/echo-checked" (lambda (req)
    (try_let (raw (req_body req))
      (catch err
        (if (regex_match "LIMIT_EXCEEDED" err)
          (res_json 413 (dict ("error" "payload_too_large")))
          (res_json 500 (dict ("error" "internal_error")))))
      (if (not (utf8_valid raw))
        (res_json 400 (dict ("error" "invalid_json")))
        (try_let (doc (parse_json Any raw))
          (catch err (res_json 400 (dict ("error" "invalid_json"))))
          (res_json 200 (dict ("bytes" (str_len raw))))))))))
```

## Authentication

Never store or compare a secret in clear. Store `(sha256_hex secret)` and compare digests with
`(secure_equal a b)`, which takes the same time whatever the inputs. `(rand_hex 32)` returns 32 secure random
bytes as hex, enough for an API token. Read the `Authorization` header with `req_header` and answer `401`
for anything that is not `Bearer <secret>`, always with the same message so the response does not reveal
whether a token exists.

```lisp check
(http_server 18083
  (route "/v1/me" (lambda (req)
    (let (h (req_header req "Authorization"))
      (if (and (>= (str_len h) 8) (= (to_lower (str_slice h 0 7)) "bearer "))
        (if (secure_equal (sha256_hex (str_slice h 7)) (sha256_hex "example-only-not-a-real-secret"))
          (res_json 200 (dict ("role" "admin")))
          (res_json 401 (dict ("error" "unauthorized"))))
        (res_json 401 (dict ("error" "unauthorized"))))))))
```

## Validation

Collect every violated field and answer once, so a client fixes all problems in one round trip. Reject
unknown fields, check each type strictly, and count lengths in code points with `str_len`.

```lisp fragment
(defun check_title ((doc any)) list
  (let (problems (list) title (map_get doc "title"))
    (do
      (if (is_nil title) (append problems "title: required"))
      (return problems))))
```

## Errors

Use one error shape for every failure, with a stable machine code, a human message and the request id, so a
report from a user can be matched to a log line. Never put runtime error text, paths or secrets in a
response.

```lisp check
(http_server 18084
  (defun send_error ((req any) (status int) (code string) (message string)) int
    (do
      (res_json status (dict ("error" (dict ("code" code) ("message" message) ("request_id" (req_id req))))))
      (return 0)))
  (route "/v1/boom" (lambda (req)
    (try_let (n (to_int "not-a-number"))
      (catch err (call send_error req 500 "internal_error" "An internal error occurred."))
      (res_json 200 (dict ("n" n))))))
  (route "*" (lambda (req) (call send_error req 404 "not_found" "No such route."))))
```

A handler that raises an uncaught runtime error gets a safe `500` with the request id from the runtime, and
the error text goes to the server log as a `fault` line, never to the client.

## Limits

The runner enforces the limits; an application only reacts. `--max-request-bytes N` bounds a request body
(default 1 MiB): reading a larger body with `req_body` or `parse_json` raises a catchable `LIMIT_EXCEEDED`
that you can turn into a `413`. `--max-instructions`, `--max-memory-bytes`, `--max-call-depth` and
`--deadline` bound a run, and `(serialize_requests)` directly under `http_server` makes handlers run one at
a time, which makes every read-modify-write atomic (the body is read before the lock is taken, and the server drops stalled connections). With a `"*"` fallback route, paths are matched exactly as sent, so `/v1//x` and `/v1/./x` are unknown routes.

```bash
howlframe run --allow-caps network --max-request-bytes 65536 server.hfbc
```

## Logging

`--access-log` writes one line per request on stderr, in a fixed format that tools can parse. A handler that
fails writes a `fault` line with the same request id, the method, the path and the runtime error.

```text
access method=GET path=/v1/tickets status=200 duration_ms=3 request_id=3f2a9c0d11e84b7a9a0c5b6d7e8f9a0b
fault request_id=3f2a9c0d11e84b7a9a0c5b6d7e8f9a0b method=GET path=/v1/boom error={"phase":"runtime","code":"TYPE_ERROR",...}
```

Request paths are logged as sent (percent-encoded), so one request is always one line. Tokens and
`Authorization` values never appear in either line.

## Running standalone

Build a `.howl` file to a bytecode artifact and run it with the grants it needs, nothing more. The worked
example needs these five grants, where `<DIR>` is the directory that holds its data file:

```bash
howlframe build milestone2/ticketdesk/src/ticketdesk.howl -o ticketdesk.hfbc
TICKETDESK_ADMIN_TOKEN=<token> TICKETDESK_DATA=<DIR>/state.json \
  howlframe run --allow-caps network,database,environment,filesystem:read=<DIR>,filesystem:write=<DIR> \
  --max-request-bytes 1048576 --access-log ticketdesk.hfbc
```

`(grants "database" "inherit:filesystem")` directly under `http_server` gives route handlers the database
capability and exactly the filesystem grants the runner holds, nothing broader. Use `(granted "network")`
at startup to fail early with a clear message when a grant is missing. From a clean checkout:
`milestone2/scripts/clean_repro.sh` builds the toolchain and the server and runs the acceptance suite.
