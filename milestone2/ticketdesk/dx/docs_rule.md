# Documentation rule (gate H, PLAN.md section 3 item 6)

Frozen 2026-10-13. A checker script (written by the implementer; the rule below is the contract) enforces it.

1. The file `docs/guides/http_api.md` MUST exist in the repository.
2. It MUST contain these level-2 headings (exact text, any order): `Routing`, `JSON bodies`,
   `Authentication`, `Validation`, `Errors`, `Limits`, `Logging`, `Running standalone`.
3. Fenced code blocks use one of three info strings, and every fenced block in the file MUST use one:
   * ```` ```lisp check ```` : a **complete program**. The checker extracts the body to a temporary
     `.howl` file and runs `howlframe check` and `howlframe build`; both MUST succeed (exit 0).
   * ```` ```lisp fragment ```` : an illustrative fragment. The checker wraps it as
     `(cli_app\n<fragment>\n)` for a fragment whose first non-blank token is not `(cli_app`,
     `(http_server`, `(web_app` or `(wasm_app`, and for an `(http_server ...` fragment it is used
     unwrapped; the wrapped text MUST pass `howlframe check` (build not required).
   * ```` ```text ```` or ```` ```bash ```` or ```` ```json ```` : not compiled (output samples, shell lines, JSON).
   A plain ```` ``` ```` fence, or a ```` ```lisp ```` fence without `check` or `fragment`, is a rule violation.
4. The guide MUST contain at least 3 ```` ```lisp check ```` blocks, and at least one of them MUST be a complete
   `http_server` program that has a `route` with a path parameter and one that reads `req.body` via `parse_json`.
5. Every `howlframe` flag named in the guide (e.g. `--max-request-bytes`, `--access-log`, `--allow-caps`) MUST
   appear in `howlframe run --help` or in `docs/cli.md`; the grant list the guide gives for running the
   example MUST be exactly the minimal grants of requirement R2 in `milestone2/ticketdesk/requirements/REQUIREMENTS.md`.
6. The guide MUST link to `milestone2/ticketdesk/` (the worked example) and MUST NOT contain the strings of
   any real token secret used in the acceptance suite (it uses placeholders like `<token>`).
7. A test (`go test` or script, run by `scripts/test.sh`) executes rules 1-6 and MUST fail when a block is
   edited to be invalid. Its output is part of the Milestone 2 evidence.

Before evidence: `docs/guides/http_api.md` does not exist at the freeze commit.
