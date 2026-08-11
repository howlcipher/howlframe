---
support: partially_supported
reason: "HowlFrame lacks structs or match statements (unsupported in standalone VM). Structured errors require returning dictionaries or lists."
---

# Task 06: Structured Error Handling

Write a function named `divide` that takes two integers `a` and `b`.
If `b` is 0, it must return a structured error or result type indicating division by zero, rather than crashing or throwing an unhandled exception.
If `b` is not 0, it returns the integer quotient.
Write tests covering both the success case and the division-by-zero error case.
