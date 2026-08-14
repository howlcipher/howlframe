# External Consumer HFIR Gap Matrix

This matrix compares the exact current usage of HowlFrame constructs across our two external consumers (ChangeOps and HowlBoard) against the direct HFIR execution support after Phase 2.

| Category | Constructs | ChangeOps | HowlBoard Backend | HowlBoard Frontend | Direct HFIR Support |
| :--- | :--- | :---: | :---: | :---: | :---: |
| **Core Values/Logic** | `let`, `set`, `if`, `=`, `+`, `<`, `and`, `is_nil`, `to_int`, `to_string` | Yes | Yes | Yes | **Supported** |
| **JSON** | `parse_json` | Yes | Yes | Yes | **Supported** |
| **Recovery** | `try_let`, `catch` | Yes | Yes | Yes | **Supported** |
| **Iteration** | `for`, `while` | `for` | `while` | `for` | `for` only |
| **Functions** | `defun` | No | Yes | Yes | Missing |
| **Call/Return** | `call`, `return` | No | Yes | Yes | Missing |
| **HTTP** | `http_server` | No | Yes | No | Missing |
| **Route/Lambda** | `route`, `lambda` | No | Yes | `lambda` | Missing |
| **Request API** | `req_method`, `req.body` | No | Yes | No | Missing |
| **Response API** | `res_json`, `res_header` | No | Yes | No | Missing |
| **Stores** | `store_open`, `store_get`, `store_put`, `store_delete` | No | Yes | No | Missing |
| **Fetch** | `fetch` | No | No | Yes | Missing |
| **DOM** | `dom_query`, `dom_value`, `set_html`, `set_text`, `toggle_class` | No | No | Yes | Missing |
| **Events** | `on_event` | No | No | Yes | Missing |
| **JS Backend** | `web_app`, `export` | No | No | Yes | Missing |

## Conclusion

ChangeOps now runs fully on direct HFIR execution.
HowlBoard uses a significantly larger subset of the language, particularly revolving around persistent state (`stores`), HTTP routing (`route`, `res_json`), function definitions (`defun`, `call`, `return`), and web application frontend logic (`web_app`, `dom_query`, `set_html`, `fetch`, `on_event`).

**Phase 2B Recommendation**: The next consumer-driven HFIR phase must target HowlBoard. This will require extending semantic HFIR, the verifier, and the lowerer to support `defun`/`call`/`return` as well as the HTTP routing, store primitives, and DOM interaction constructs.
