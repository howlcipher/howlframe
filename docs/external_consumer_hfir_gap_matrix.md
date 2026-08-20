# External Consumer HFIR Gap Matrix

This matrix reflects the clean consumer checkouts used by the 2026-08-14 runtime integrity gate. “Direct HFIR proven” means the current consumer policy was compiled with `compile-hfir-bc` and exercised through `run-bc`; it does not imply that every consumer feature is in HFIR.

| Category | Current constructs | HowlChangeOps | HowlBot | HowlBoard backend | HowlBoard frontend |
| --- | --- | --- | --- | --- | --- |
| Core logic | `let`, `set`, `if`, `do`, equality, `and`, `or`, conversions | Direct HFIR proven | Direct HFIR proven | Legacy bytecode | JS-only |
| CLI args | `cli_app`, `cli_args` | Direct HFIR proven | Direct HFIR proven | Missing | Missing |
| JSON | `parse_json`, `dict`, `map_get` | Direct HFIR proven | Missing | Legacy bytecode | JS-only |
| Recovery | `try_let`, `catch` | Direct HFIR proven; interpreter path unsupported | Missing | Legacy bytecode | JS-only |
| Iteration | `for` | Direct HFIR proven | Direct HFIR proven | Legacy bytecode | JS-only |
| Functions | `defun` | Missing | Missing | Legacy bytecode | JS-only |
| Call/return | `call`, `return` | Missing | Missing | Legacy bytecode | JS-only |
| HTTP | `http_server` | Missing | Missing | Legacy bytecode | Missing |
| Route/lambda | `route`, `lambda` | Missing | Missing | Legacy bytecode | JS-only |
| Request/response | `req_method`, `res_json`, `res_header` | Missing | Missing | Legacy bytecode | Missing |
| Stores | `store_open`, `store_get`, `store_put`, `store_delete` | Missing | Missing | Legacy bytecode | Missing |
| Filesystem | `read_file`, `bytes_to_string` | Direct HFIR proven | Missing | Legacy bytecode | Missing |
| Fetch | `fetch` | Missing | Missing | Missing | JS-only |
| DOM | `dom_query`, `dom_value`, `set_html`, `set_text`, `toggle_class` | Missing | Missing | Missing | JS-only |
| Events | `on_event` | Missing | Missing | Missing | JS-only |
| JS backend | `web_app`, `export` | Missing | Missing | Missing | JS-only |

## Evidence and interpretation

HowlChangeOps, HowlPlane, and HowlBot represent primary external policy consumers. HowlChangeOps and HowlBot are the two direct-HFIR policy consumers; their current policy artifacts produced matching decisions on their tested normalized inputs. HowlPlane operates as a primary real-world dogfooding consumer for AI engineering control plane workflows. HowlBoard remains a legacy-bytecode application and was used here as a runtime stress consumer; it was not migrated.

HowlBot’s role/set and JSON-array ergonomics were not execution blockers. They remain ergonomic backlog items rather than new language surface for this milestone.

HowlBoard’s `set_html` path still maps to raw `innerHTML`; the existing XSS concern remains tracked in `docs/howlboard_xss_set_html_issue.md`.

The next HFIR expansion, if the final gate remains green, should target the HowlBoard backend in consumer-driven phases: function call/return semantics, stores, and then HTTP request/response behavior.
