# HowlFrame Numeric Contract

## 1. Scope

This contract governs the meaning of numeric values and numeric operations in every supported HowlFrame execution backend. A single HowlFrame program must produce the same value, decision, and error across the interpreter, production bytecode VM, generated Go, and generated JavaScript when the program stays within the supported surface documented here.

## 2. Value Representations

### 2.1 Integer

* Signed two's-complement 64-bit (`int64`).
* Guaranteed exact range: `[-2^63, 2^63-1]`.
* Integer literals without a decimal point or exponent that parse as a base-10 signed 64-bit value are `int64`.
* Values produced by integer-returning builtins such as `list_len` and `time_now` are `int64`.

### 2.2 Floating-point

* IEEE 754 binary64 (`float64`).
* Floating literals that contain a decimal point or exponent are `float64`.
* Results of operations that must be real (division) are `float64`.

### 2.3 No implicit NaN or Infinity in policy values

* `NaN` and `+Inf`/`-Inf` are not valid policy values.
* Any operation that would produce them (e.g., division by zero) raises a runtime `DIVISION_BY_ZERO`/`RUNTIME_ERROR` rather than returning an inexact value.
* `to_float` of `"NaN"`, `"Inf"`, `"Infinity"`, etc. fails with `CONVERSION_ERROR`.

## 3. Literal Semantics

* Integer literal: `42`, `-7`, `0` → `int64`.
* Floating literal: `3.14`, `2.0`, `1e10` → `float64`.
* A literal `2.0` is a float; a literal `2` is an integer. The type is determined by syntax, not by magnitude.

## 4. Mixed Arithmetic

For `+`, `-`, `*`:

| Left | Right | Result type | Semantics |
|---|---|---|---|
| int64 | int64 | int64 | Exact signed 64-bit integer arithmetic. Overflow is a runtime error. |
| int64 | float64 | float64 | Promote the integer to float64, then IEEE 754 arithmetic. |
| float64 | int64 | float64 | Promote the integer to float64, then IEEE 754 arithmetic. |
| float64 | float64 | float64 | IEEE 754 arithmetic. |

String concatenation using `+` is unchanged: both operands must be strings.

## 5. Division

Model A — `/` is **always real division**.

* `9 / 2` → `4.5` (float64).
* `8 / 2` → `4.0` (float64).
* Mixed operands are evaluated as float64.
* Division by zero raises a runtime error (`DIVISION_BY_ZERO`).
* To obtain integer truncation, compose `to_int`: `(to_int (/ a b))` truncates toward zero.

This model was chosen because it is backend-agnostic and avoids the ambiguity of host-language integer division rules.

## 6. Comparison and Ordering

### 6.1 Equality (`==`, `=`, `!=`)

* Equality is mathematical and exact across representations.
* `int64(2) == float64(2.0)` is `true`.
* `int64(9007199254740993) == float64(9007199254740992.0)` is `false`; the integer is never rounded through float64 merely to compare.
* Collections (lists/dicts) are never equal to numeric values.
* `!=` is the logical negation of `==`.

### 6.2 Ordering (`<`, `>`, `<=`, `>=`)

Ordering follows exact mathematical order:

* Both int64 → signed 64-bit comparison.
* Both float64 → IEEE 754 comparison.
* Mixed int64/float64:
  * If the float is a whole number and fits in int64, compare both as int64.
  * Otherwise, compare as float64 after converting the int64 to float64.
* This rule keeps `9007199254740993 > 9007199254740992.0` true instead of collapsing it through float64 rounding.

## 7. Overflow and Underflow

* Signed 64-bit integer overflow, underflow, and division by zero are runtime errors.
* Floating-point overflow to `+Inf`/`-Inf` or NaN is not allowed as a policy value; if an operation would produce one, the runtime reports a numeric error.
* Backend implementations must check before relying on host-language wraparound or infinities.

## 8. Numeric Conversions

### 8.1 `to_int`

Accepts:

* `int64` → unchanged.
* `float64` → truncates toward zero if the value is finite and fits in int64; otherwise error.
* Numeric string (`"123"`, `" -7 "`) → parses signed base-10 int64; whitespace is trimmed. Empty, invalid, out-of-range, or non-numeric strings fail with `CONVERSION_ERROR`.
* Non-numeric types → `CONVERSION_ERROR`.

### 8.2 `to_float`

Accepts:

* `int64` → exact float64 conversion.
* `float64` → unchanged.
* Numeric string (`"3.14"`, `"1e2"`) → parses float64; whitespace trimmed. Invalid or overflow strings fail with `CONVERSION_ERROR`.
* Non-numeric types → `CONVERSION_ERROR`.

### 8.3 `to_string`

Produces a decimal representation consistent with the value's type without exposing internal tags.

## 9. JSON Numbers

* JSON integer tokens that fit in signed 64-bit are decoded as `int64`.
* JSON fractional or exponent values are decoded as `float64`.
* Values outside the int64 range that are still JSON numbers are decoded as `float64` and may lose precision; operations on such values follow the float64 branch of this contract.
* A JSON value decoded with `parse_json` and an equivalent literal must behave the same way under this contract.

## 10. Numeric-Producing Builtins

* `list_len` returns `int64`.
* `time_now` returns `int64` (Unix timestamp).
* `to_int`/`to_float` behave as defined above.
* HTTP status values, argument indexes, and string-to-number conversions are governed by the same rules.

## 11. Backend Implementation Rule

Each supported backend must implement the contract above, not merely mirror the host language. Shared semantic helpers are preferred over scattered one-off fixes.

## 12. Fail-Closed Defaults

If a backend cannot represent a value or perform an operation in a way that preserves this contract, it must raise a structured runtime error rather than silently approximate. Static checks should reject constructs that are known to be unsupported on the target.

## 13. Target Limits

* **Generated JavaScript** represents every number as binary64, so integers outside +/-(2^53-1) are not exact. In obedience to section 12, the checker rejects integer literals outside that range in `web_app` programs, and the generated `parse_json` rejects integer JSON tokens outside that range where the engine exposes token source. Statically int-typed `+`, `-`, and `*` in generated JavaScript raise a runtime error when the result leaves that range instead of rounding. The VM, interpreter, and Go keep exact int64 results there, so such programs fail closed on JavaScript rather than agreeing with the other backends. Where operand types are not statically known (for example JSON fields), generated JavaScript applies the same range check at runtime when both operands are integer-valued, which also rejects whole-valued float results beyond 2^53; that is a deliberate fail-closed trade-off.
* **Wasm** has no int-to-float promotion for `/`; integer division fails closed at serialization.

## 14. Version

This contract applies to the numeric parity mission run `20261001-001` and supersedes any undocumented host-language behavior that previously differed across backends.
