# Task 07: Native Store

Write a simple application that uses the language's native stateful key-value store capabilities (if using HowlFrame, use native stores).
The application should have a function `increment_counter(key)` that reads an integer value for `key` from the store, increments it by 1, and saves it back. If the key does not exist, initialize it to 1.
Write a unit test that calls `increment_counter("visits")` twice and asserts the final value is 2.
