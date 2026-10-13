# Network authority (#165)

HowlFrame is a capability-governed runtime, not a sandbox. Grants are
checked inside the host process before an effect runs; they are not an
OS-level isolation boundary. Run untrusted programs inside a container or VM
with its own egress policy as well.

## Two kinds of network authority

| Grant | Authorizes |
|---|---|
| `network` | Serving: `http_server`, `route`, `http_server_serve`, plus the model forms (`llm_generate`, `confidence`, `achieve`, `neural_circuit`, `ephemeral_circuit`, `lazy_synthesize`). **Never** an outbound `fetch` / `http_request`. |
| `network:fetch=<host>[:<port>]` | Outbound `fetch` and `http_request` to exactly that host and port. Repeat the grant once per host. |

Response and request forms inside a route (`res`, `res_json`, `res_header`,
`http_res`, `req_method`, `req_query`, `req_header`, `req_path`) need no
capability at all: they only touch the request the server already accepted.

## Host matching

```
-allow-caps 'network:fetch=api.example.com'          # :80 for http, :443 for https
-allow-caps 'network:fetch=api.example.com:8443'
-allow-caps 'network:fetch=127.0.0.1:9000,network:fetch=[::1]:9000'
```

* Matching is exact. Host names compare case-insensitively; there are no
  wildcards and no suffix matching (`example.com` does not cover
  `a.example.com` or `evilexample.com`).
* A grant without a port covers only the scheme default (80 for `http`, 443
  for `https`).
* IP literals compare in canonical form and match only a grant that names
  the same IP. A grant for `127.0.0.1` does not cover `localhost`, and a grant
  for `localhost` does not cover `127.0.0.1`. Ambiguous numeric hosts
  (`127.1`, `0x7f.0.0.1`, `2130706433`, `0177.0.0.1`) are refused outright,
  in grants and in URLs. IPv6 literals must be bracketed and carry no zone.
* Only `http` and `https` URLs are fetched. Userinfo is ignored for matching,
  so `http://allowed.example@169.254.169.254/` is checked as
  `169.254.169.254`.
* Every redirect hop is re-checked against the same allow-list (at most 10
  hops). A hop to an ungranted host stops the fetch with `CAPABILITY_DENIED`
  before that host is contacted.
* A granted DNS name may not connect to a link-local, multicast or
  unspecified address (`169.254.0.0/16`, `fe80::/10`, `0.0.0.0`, `::`), which
  is where cloud metadata services live. Granting the IP literal itself (for
  example `network:fetch=169.254.169.254`) is the only way to reach one.
* `HTTP_PROXY` / `HTTPS_PROXY` are ignored by governed fetches.

## Denials and receipts

A denied fetch raises `CAPABILITY_DENIED` before any connection opens:

```
capability denied: network: fetch host is not in the allow-list (requires network:fetch=169.254.169.254:80)
```

On `-run-bc` the receipt records a denied `FETCH` / `HTTP_REQUEST` effect
whose `grant` field is the grant that would have authorized it
(`network:fetch=host:port`; never the path, query or userinfo). An allowed
fetch records the matched grant, and each followed redirect hop is recorded
as its own effect with target summary `redirect`.

Receipt `grant` strings are copied verbatim from the normalized grant. For
`process:exec=` entries that includes every pattern literal exactly as
written (for example `process:exec=/usr/bin/git log --oneline *`), so do not
put secrets in grant patterns.

## Routes

A route handler holds only its own `(grants ...)` list, or the server-level
`(grants ...)` default. Nothing is implicit, not even `network`: a route with
no grants can answer requests but cannot fetch anything, including its own
server (`/admin`) or a metadata address. To let a route call out, list the
host:

```
(http_server 8080
  (route "/relay" (grants "network:fetch=api.example.com")
    (lambda (req) (http_res (http_request "https://api.example.com/v1" "GET")))))
```

The runner must hold every grant a route lists, or registration fails with
`CAPABILITY_DENIED` at `HTTP_ROUTE` before the server listens.

When a handler fails, the HTTP client receives only
`500 {"error":"internal server error"}`. The runtime error (code, opcode,
message) goes to the server's stderr log (and the receipt when one is
recorded).

## Targets

| Path | Behavior |
|---|---|
| Interpreter (`-run`) | Same host matching, redirect re-check and dial guard. No receipts. |
| `-run-bc` (AST and HFIR artifacts) | Full enforcement plus receipts. |
| Generated Go | Same host matching, redirect re-check and dial guard from `HOWLFRAME_ALLOW_CAPS`. Generated Go runs every route with the whole process's grants, so `fetch` / `http_request` inside a route or middleware fails closed at build time with `UNSUPPORTED_ON_TARGET`. |
| Generated JavaScript (Node) | Same host matching (WHATWG-canonical hosts) and per-hop redirect re-check (`redirect: "manual"`); DNS names are resolved first and refused if they resolve to a link-local or unspecified address (a resolve-then-connect check, so a rebinding race remains). |
| Generated JavaScript (browser) | Same host matching. Opaque redirects are refused. A browser cannot resolve names, so the link-local guard does not apply there. |

## Required-capabilities profile

`-required-caps` reports `network:fetch=host:port` for a fetch whose URL is a
constant, and `network:fetch=<dynamic>` (not a valid grant) for a computed
URL; replace it with one grant per host the program may reach.
