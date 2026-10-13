# Milestone 1 baseline (2026-10-13)

Directive: `HOWLFRAME_PRODUCT_DIRECTIVE.md`. Active gate: Milestone 1 (three independently selected, frozen, standalone CLI apps, D24 A-J evidence each).

## Verified now (main @ eb9ccf87, Fedora 44 host)

* `go build ./...` and `go vet ./...` pass.
* `go test ./...` fails in 5 packages, all exec tests: `internal/backend/gogen`, `internal/backend/javascript`, `internal/vm`, `tools/difftest` (conformance `exec_granted`, `nested_exec_granted`, `nested_multi_effect_*`).
* Cause: on this host `/usr/bin/{printf,echo,touch}` are symlinks into `/usr/lib/cargo/bin/coreutils/` (uutils). The #163 exec hardening correctly refuses symlinked programs ("grant and call its canonical path"), while the tests and frozen ABI fixtures hard-code `/usr/bin/<tool>`. This is a test-portability issue, not a product defect. Not fixed here: roughly 20 literal sites plus frozen conformance fixtures, unrelated to Milestone 1. Milestone 1 apps must grant canonical program paths on this host.
* #147 (task_api/status_api parallel flake) did not reproduce in this run; left open.

## Library conflicts

* #175 was deliberately reopened: `AdapterDeadline` exists but `-adapter-deadline` is not wired into the CLI. Left open.
* Open: #181, #182, #174, #164, #157. Draft PR #178 (live-v1 harness) untouched. Fixed only if a Milestone 1 app hits them.

## Next

Independent selector agent writes `milestone1/<NN>-<slug>/requirements` and acceptance suites plus `milestone1/SELECTION.md`. The user approves them, then they are frozen in a commit before any implementation.
