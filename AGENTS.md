# HowlFrame Agent Instructions

- Treat red CI or a non-zero validation command as incomplete work.
- Before a commit or push, run the repository CI-equivalent validation appropriate to the change.
- Do not weaken tests to obtain a passing result; prefer behavioral assertions unless wording is a public contract.
- Keep generated artifacts out of the repository root and use the authoritative registries and code generator for construct coverage.
- Verify compatibility claims against executable evidence and record consumer-driven findings in `docs/journals/`.
