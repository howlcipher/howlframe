---
support: unsupported
reason: "HowlFrame lacks list_len and sort constructs, making list iteration and sorting virtually impossible."
---

# Task 05: List/Dictionary Transformation

Write a function named `transform` that takes a list of dictionary-like objects (or equivalent records/structs).
Each object contains `id` (integer), `name` (string), and `active` (boolean).
The function should return a list of `name` strings for all objects where `active` is true, sorted alphabetically by name.
Write a unit test that verifies this behavior.
