# Task 02: CLI File Processing

Write a command-line application that takes a file path as an argument.
The file contains a list of names, one per line.
The application should read the file and print `Hello, <name>!` for each non-blank line.
If the file does not exist, the application must handle the error gracefully by printing `Error: File not found` to stderr and exiting with code 1, without crashing or printing a stack trace.
