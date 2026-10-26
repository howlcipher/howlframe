# HowlFrame documentation audit (protocol G5, gate 3)

The HowlFrame arm's agent gets exactly this documentation: the HowlFrame `README.md`, `docs/cli.md`, `docs/reference/`
and the HowlPlane skill `SKILL.md`, pinned in `../harness/toolchains.json`. The audit checks that it matches the pinned
compiler (`e77e008f`).

* **Auditor:** Antigravity (`gemini-3.8-flash-high`), 2026-10-25. Antigravity's agents are not measured in this
  experiment. Its report is `AUDIT_antigravity_2026-10-25.md`, unedited apart from paths made relative. It found 20
  discrepancies: 9 *breaks*, 8 *misleads* and 3 *cosmetic*.
* **Operator:** Claude Opus 5.5 checked each claimed discrepancy against the compiler before acting on it.
* **Rule for fixes:** correct only statements that are wrong. Do not add guidance the documentation never gave
  (rounding, a `nil` literal, default VM limits), because adding it would mean tuning the HowlFrame arm's materials
  towards the tasks. Those gaps are listed below for the Owner instead.

## Dispositions

| Finding | Disposition |
|---|---|
| SKILL.md `compile-js` (breaks) | Fixed: `build --target js` (HowlPlane PR, below) |
| SKILL.md `-caps` at run time (breaks) | Fixed: `run --allow-caps`, plus `inspect --caps` to list an artifact's needs |
| SKILL.md `web_app` example fails `check` (breaks) | Fixed: `(type_hint return "void")`. The default types of untyped `defun` parameters and returns are now stated, because the example cannot be correct without it |
| SKILL.md module call (breaks) | Fixed: documented `(call alias/func ...)`. The compiler accepting the dotted form is a bug: **howlframe#224** (not fixed; runtime and compiler changes are outside this mission) |
| SKILL.md `try_let` with two operands (breaks) | Fixed: three operands |
| SKILL.md `append` (misleads) | Fixed: an in-place statement with no value |
| README: the default backend writes `server.go` (breaks, twice) | Fixed: with no subcommand the program runs in the VM; `build --target go` writes `server.go` |
| README web-app commands (breaks) | Fixed: `build --target js` writes `app.js` and `app.test.js`; `node --test` passes |
| README `optimize_signature` example (breaks) | Fixed: the symbol is bound in scope; `-optimization-plan` prints the plan |
| README benchmark commands (breaks) | Clarified: they need a repository checkout. They are absent from the agent's documentation bundle by design |
| `NUMERIC_CONTRACT.md` §2.3: `to_float "nan"` (breaks) | **Not changed.** The contract is normative and the runtime deviates from it: **howlframe#225**. Whether to fix the runtime or amend the contract is the Owner's decision |
| README `.bc.bin` default (misleads) | Fixed: `build` writes `.hfbc`; the legacy `-compile-bc` writes `<input>.bc.bin` |
| README build output location (misleads) | Fixed: written to the current directory |
| README `parse_json` takes a variable (misleads) | Fixed: stated in general, not only for `req.body` |
| README Ollama (misleads) | Clarified: needs a repository checkout and Ollama |
| cli.md: missing `inspect`, `check --profile` and build options (misleads, cosmetic) | Fixed |
| cli.md: `run` accepts `.howl` (cosmetic) | Fixed. Also documented, found while checking: a literal `--` after the artifact is passed through to the program, although the built-in usage line shows `[-- arguments...]` |
| `artifact_compatibility.md` version table (misleads) | Fixed: current builds print `0.1.0` with `HFBC format: 2`, so the table now keys on the format line. A stale local `file://` link was replaced with a relative one |
| `process_authority.md`: `/usr/bin/find` is a symlink (cosmetic) | **Rejected:** `/usr/bin/find` is a regular file on this host and in the pinned base root |

**Pull requests:** HowlFrame `docs/audit-fixes-2026-10-25` (README, `docs/cli.md`, `artifact_compatibility.md`) and
HowlPlane `docs/howlframe-skill-audit-fixes` (`SKILL.md`). Both are documentation-only. Once merged, the pinned
HowlFrame commit, the HowlPlane skill commit, the documentation bundle and `../harness/lock.json` must be refreshed. The
auditor should then re-run the examples against the merged documentation.

## Gaps for the Owner (not wrong, but missing)

From writing the HowlFrame calibration reference (`../calibration/calib-apilog-analyzer/references/howlframe/REFERENCE_NOTES.md`):

* **VM limits.** The default resource limits (`--max-instructions` 100,000, `--max-memory-bytes` 64 MiB) are documented
  in `docs/cli.md`. A program processing a 46 MB input needed both raised (the reference uses 2×10⁹ instructions and
  64 GiB). An agent must notice and choose these; the baselines have no equivalent step.
* **Missing built-ins.** There is no rounding function, and no `nil` literal (the reference builds a null from a
  missing map key).
* **Arity and types.** `stderr` takes exactly one argument. `read_file` returns bytes.

Documenting these would help agents in the HowlFrame arm. Whether that is a fair pre-registration change, or tuning
towards the tasks, is the Owner's call before the freeze.
