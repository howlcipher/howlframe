# 2026-10-05: Model-call capability gates (bugs.md #57)

## Context

The AI-native architecture review (`docs/ai-native-language-review/`) checked
every path by which a program can cause an external effect. Three bytecode
opcodes and two interpreter paths could reach the local model host
(`127.0.0.1:11434`) with no capability grant.

## Reproduction on `d496d972`

A recording HTTP listener on `127.0.0.1:11434`, then:

| Program | Mode | Grant | Observed |
| --- | --- | --- | --- |
| `(print (confidence "the sky is blue"))` | `-run-bc` | none | `POST /api/generate`, printed `0.9` |
| same | `-run` | none | `POST /api/generate`, printed `0.9` |
| `(print (neural_circuit ("x") "classify"))` | `-run-bc` | none | `POST /api/generate` |
| `(print (ephemeral_circuit ("x") "classify"))` | `-run-bc` | none | `POST /api/create`, `DELETE /api/delete`, `POST /api/generate` |
| `tests/test_lazy_synthesize.howl` | `-run` | none | `POST /api/generate`, then the reply ran as the function body |
| same | `-run-bc` | none | `CAPABILITY_DENIED` (already gated) |

## Change

- `internal/bytecode/opcode.go`: `OpConfidence`, `OpNeuralCircuit`, and
  `OpEphemeralCircuit` declare `capability.Network`.
- `internal/capability/capability.go`: `ForConstruct` returns `network` for
  `confidence`, `neural_circuit`, and `ephemeral_circuit`.
- `internal/vm/vm.go`: the interpreter checks `network` before building the
  `lazy_synthesize` request, as the bytecode VM already does.
- Generated references: the capability column in
  `docs/reference/bytecode_reference.md` and the regenerated
  `docs/reference/construct_coverage.md`.

Only those rows were touched in `bytecode_reference.md`. Running
`go run ./cmd/codegen` also rewrites that file's `SPAWN_AGENT` operand row
(`string, int64` since #100), drops a hand-written "Artifact Serialization &
Compatibility" section the generator does not produce, and adds `int_operand`
to `SpawnAgentInstruction` in `tools/orchestrator/orchestrator_schema.py`. That
drift was already there before this change and is left for a separate change.

## Evidence

- New `internal/vm/model_call_capability_test.go`. The model host port is
  hard-coded, so the test binds it when free and counts requests. It checks
  denial under an empty grant and under every grant except `network`. It
  checks that a denied run prints nothing and sends no request. A granted
  `confidence` call sends exactly one request, which shows the probe works.
- With the production change stashed, the new tests fail: opcode capability
  `""`, no `CAPABILITY_DENIED`, and the interpreter runs `confidence` and
  `lazy_synthesize`.
- `gofmt -l .` is clean. `go vet ./...`, `go build ./...`, and
  `go test ./...` pass (38 packages ok).

## Boundaries

- No opcode, operand, or artifact-format change. Existing artifacts that use
  these opcodes now need `-allow-caps network`. That is the intended
  fail-closed result.
- Generated Go and JavaScript still mediate only `env`, `exec`, `read_file`,
  `write_file`, `mkdir`, and `fetch`. Generated Go still calls the model host
  directly for these constructs. That gap is already documented and is
  tracked in the review roadmap.
- Production `-compile-bc` is still AST bytecode. #90 stays Partial. The
  Assurance tip-lock `4d74dbcf` was not retaken.
