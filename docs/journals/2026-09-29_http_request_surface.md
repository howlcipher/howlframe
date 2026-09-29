# HTTP request surface

## Why this slice

Body-and-literal-path was the ceiling for an `http_server` handler.
`req_method` and `res_header` already existed. Query strings, path
segments, and request headers did not, so a Board-style filter, an auth
header, or `/tasks/{id}` could not be read. Improvement #102 is that
slice. It does not rewrite the router.

## Contract

Three pure reads of a request the handler already holds:

- `(req_query req name)` is the first query-string value. Names are
  case-sensitive. A repeated key contributes only the first value.
- `(req_header req name)` is the first header value. Lookup is
  case-insensitive, matching `net/http`.
- `(req_path req name)` is one whole-segment capture from the route that
  matched this request. Names are case-sensitive.

A missing name is `""`. A present query value that is empty is also `""`.
That is the same string a caller already uses for a missing `map_get`, and
this slice does not change `map_get` or `store_get`.

A receiver that is not an `*http.Request`, a name that is not a non-empty
string, and a route path whose braces are not a whole `{name}` segment fail
closed with `TYPE_ERROR`. A second pattern with the same literal skeleton
fails closed with `RUNTIME_ERROR` at registration. An exact literal route
wins over a pattern that would capture the same path.

`{name}` is an identifier and a whole path segment, for example
`/tasks/{id}/notes/{note}`. The path is cleaned the way `net/http` cleans
it, including `.` and `..`, and a trailing slash is significant.
`/tasks/{id}` does not match `/tasks/9/`.

The same forms lower in the tree-walking interpreter, the bytecode VM, the
Go backend, and the JavaScript backend. JavaScript reads a Fetch `Request`
(`url`, `headers.get`) or a plain object (`url` or `query`, `headers`,
`pathParams`). It does not gain an HTTP server.

Direct HFIR lowering emits the same three opcodes. The production compiler
remains AST to bytecode.

## Authority

These reads declare no capability. `ForConstruct` returns none for
`req_query`, `req_header`, `req_path`, and the opcode names. A handler can
run them with an empty grant when the request is already bound.

That is deliberate, and it is not how `req_method` is classified.
`req_method` still requires `network`. Copying that grant onto a field read
would invent a check for data the handler already holds. Starting the
server, registering a route, serving, and writing a response still require
`network`. These opcodes do not grant `database`, `filesystem`, or any new
capability.

Literal routes stay registered on the existing `ServeMux`. Only `{name}`
patterns use the small dispatcher in `internal/httpreq`. Go 1.21 has no
wildcard mux, and this slice does not replace the mux.

## What this does not do

No general router. No method dispatch, no regular expressions, no optional
segments, no catch-all, no middleware redesign. No header-write changes.
No modules, no `html_escape`, no chained `map_get`, no absence-idiom lock
(#103–#105). No Factory, Board, or HowlPlane changes. Repeated query and
header values are not returned as a list. HFIR is not promoted to the
production compiler.
