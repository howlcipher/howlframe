# HowlBoard Dogfooding Journal

## Experiment

HowlBoard is a standalone full-stack task management application written entirely in HowlFrame to test the viability of HowlFrame for building real-world web applications. It tests the bytecode server backend and the JavaScript frontend backend.

## Frictions Discovered

### A. Core Semantic Limitations

1. **Missing HTTP Route Return Semantics**
   * **Problem:** In a `route` block, calling a helper function (like `with_cors`) that executes a `return` statement only returns from that helper function, not the surrounding route handler. Because HTTP response macros like `res_json` directly write to the response, you cannot easily abort a route early if a helper function already responded.
   * **Workaround:** Used `if (!= (req_method req) "OPTIONS")` in the route handler after the helper function to skip the rest of the handler body.
   * **Classification:** B (Ergonomic Issue)

2. **No Web App Input Reading**
   * **Problem:** There was no way to read the value of a DOM input element. `get_attr` did not exist in the JavaScript backend.
   * **Fix:** Implemented `(dom_value selector)` construct which emits `.value` in JavaScript.
   * **Classification:** A (Blocking Issue)

3. **No CORS Support in Bytecode VM**
   * **Problem:** The HTTP server could not serve cross-origin requests because there was no way to read the request method to intercept `OPTIONS`, and no way to set response headers.
   * **Fix:** Added `req_method` to read the method of an HTTP request, and `res_header` to set response headers.
   * **Classification:** A (Blocking Issue)

### B. Compiler and Backend Implementation Bugs

1. **`parse_json` inside `try_let` fails JS Type Checker**
   * **Problem:** The `try_let` special form and `parse_json` construct were not registered in `checkJSStatement` in the `checker.go` file. When the checker attempted to process `try_let`, it fell through to default AST walking, which then attempted to process `parse_json` as a generic statement, leading to an `Unknown statement for JS: parse_json` panic.
   * **Fix:** Added special cases for `try_let` and `parse_json` in the JS backend checker.
   * **Classification:** C (Implementation Bug)

2. **`fetch` Optional Body Arity**
   * **Problem:** `fetch` required exactly two arguments (URL and Method).
   * **Fix:** Updated the checker and both Go/JS backends to support an optional third argument for the request body.
   * **Classification:** B (Ergonomic Issue)

## Progress

The dogfooding loop was successfully executed. The HowlBoard application was created as a sibling directory, the aforementioned friction points were encountered during development, fixes were directly applied to the HowlFrame repository, and HowlBoard now successfully builds and runs.
