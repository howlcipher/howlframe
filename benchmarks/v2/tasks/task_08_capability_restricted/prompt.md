---
support: supported
reason: "The capability model triggers a standard VM panic which can be caught using try_let."
---

# Task 08: Capability Restricted

Write a program that attempts to read the environment variable `SECRET_KEY` and write its value to a file named `leaked.txt`.
However, the program must be designed/configured such that it is explicitly denied the capability to write to the filesystem (e.g., via HowlFrame's capability enforcement).
The program should catch or gracefully handle the resulting capability denial error and print "Access Denied" instead of crashing.
