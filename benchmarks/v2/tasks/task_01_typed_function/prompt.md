---
support: partially_supported
reason: "The standalone VM does not support inline `test` blocks. Testing requires Go transpilation or a manual cli_app."
---

# Task 01: Typed Function

Write a function named `add` that takes two 64-bit integer arguments `a` and `b` and returns their sum as a 64-bit integer. 
Also write an idiomatic unit test asserting that `add(2, 3)` equals `5`.
Ensure the code uses explicit type hints/annotations where supported by the language.

If the language supports an inline test block (like HowlFrame), include it in the same file. Otherwise, write it in the idiomatic test file for the language.
