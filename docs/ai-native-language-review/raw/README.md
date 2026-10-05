# Raw agent material

These files are exactly as the reviewers returned them. Grading and
corrections are in `../04-agent-responses-summary.md` and
`../06-reconciliation-matrix.md`.

- `common-prompt.md` is the prompt every reviewer received.
- `role-X.md` is the role-specific section appended for reviewer X.
- `out-X.md` is the answer from reviewer X.

| Role | Engine |
| --- | --- |
| A, B, D, F | Codex CLI |
| C | AGY `gemini-3.1-pro-high` |
| E | AGY `gemini-3.8-flash-high` |

`out-E.md` came back malformed: the header is duplicated and the text is
truncated. It is preserved unchanged.

Path and line citations in `out-C.md` and `out-E.md` often point to
positions inside the review context pack, not to the real repository
files. Treat them as unverified.
