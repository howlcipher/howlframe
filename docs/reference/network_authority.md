# Network authority (#165, #171, #172)

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
* Hostname grants block DNS results in loopback (`127.0.0.0/8`, `::1`),
  RFC1918 (`10/8`, `172.16/12`, `192.168/16`), IPv6 ULA (`fc00::/7`),
  NAT64 (`64:ff9b::/96`), link-local (`169.254/16`, `fe80::/10`) and
  unspecified ranges. IPv4-mapped IPv6 is checked as IPv4. Go checks the
  resolved address before dialing and again at socket creation. Node pins
  its connection to the address checked by its lookup.
* Explicit IP-literal grants and the exact name `localhost` are deliberate
  opt-outs for those ranges. They still require the exact host and port;
  `localhost` does not authorize `127.0.0.1`, or vice versa. Multicast
  (`224.0.0.0/4`, `ff00::/8`) is always refused, even with a literal grant.
* HTTPS-to-HTTP redirects are refused even when both hosts are granted.
  On any host, effective port or scheme change, Authorization and Cookie
  headers are removed for the rest of the chain, including subsequent hops
  on the new origin.
* `HTTP_PROXY` / `HTTPS_PROXY` (and lowercase variants) are ignored by
  governed fetches. Node 24 with `NODE_USE_ENV_PROXY=1` honors proxy env in
  its native fetch; the governed Node path uses native HTTP requests with
  `agent: false` and a pinned lookup instead. The proxy trap test also runs
  with that setting enabled at Node startup.
* JavaScript previously accepted `network:fetch=127.1` because WHATWG URL
  parsing canonicalized it to `127.0.0.1`. Ambiguous numeric grants and URL
  hosts are now refused like Go. Trailing-dot names are invalid hostnames.


## Denials and receipts

A denied fetch raises `CAPABILITY_DENIED` before any connection opens:

```
capability denied: network: fetch host is not in the allow-list (requires network:fetch=169.254.169.254:80)
```

On `-run-bc` the receipt records a denied `FETCH` / `HTTP_REQUEST` effect
whose `grant` field is the grant that would have authorized it
(`network:fetch=host:port`; never the path, query or userinfo). An allowed
fetch records the matched grant, and each followed redirect hop is recorded
as its own effect with target summary `redirect`. An allowed effect under
an explicit IP-literal or localhost grant also records
`private_destination_opt_out: "explicit IP-literal or localhost grant"`.
This reports permission to reach private destinations; it does not claim
that the request successfully connected or reached a private IP.

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

Granting `network:fetch=localhost:<server-port>` lets a route reach its own
`/admin` (and every other path on that origin). Grants do not restrict URL
paths. An explicit loopback literal grants equivalent access when used
as the URL host.

The `apps/webhook_relay` route is pinned to `127.0.0.1:8099` but reads
`UPSTREAM_URL` at request time. That environment variable selects the URL,
not the grant: any other host or port is denied. To change the upstream,
update the route grant and the runner's grant as well as `UPSTREAM_URL`.

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
| Generated JavaScript (Node) | Same strict grant matching, per-hop redirect checks and private/multicast guards; checked DNS addresses are pinned into native HTTP requests, which ignore proxy env. No receipt emission on this target. |
| Generated JavaScript (browser) | Same host matching. Opaque redirects are refused. Literal multicast and HTTPS downgrades are refused. A browser cannot inspect DNS results or pin a connection, so private-name DNS enforcement depends on browser/network policy; it cannot provide the Node dial guard. |

## Required-capabilities profile

`-required-caps` reports `network:fetch=host:port` for a fetch whose URL is a
constant, and `network:fetch=<dynamic>` (not a valid grant) for a computed
URL; replace it with one grant per host the program may reach.
