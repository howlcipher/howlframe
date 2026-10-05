# SPAWN_AGENT body length bounds

## Diagnosis and fix

Malformed HFBC could supply a negative or oversized SPAWN_AGENT body length,
causing a host slice-bounds panic or moving the instruction pointer backwards.
The VM now validates the int64 operand against [0, len(insts)-ip-1] before
converting it to int. Invalid lengths fail closed with RUNTIME_ERROR at the
spawn instruction, including the supplied length and allowed range.

The guard follows the task type and spawn depth checks and precedes spawning
output and slicing. Valid programs retain their existing behavior.

## Tests

RunBytecodeWithEvidence regression tests allow capability.Process and cover -1,
5 with one trailing instruction, and math.MaxInt64. Each requires RUNTIME_ERROR,
SPAWN_AGENT, instruction index 1, empty stdout, and no escaping host panic;
the structured error code also excludes a recovered host panic (VM_INTERNAL).
An exact remaining-length body executes successfully and prints its output.

Validation:

```
gofmt -l internal/vm            # clean
go vet ./internal/vm/           # clean
go test ./internal/vm/... ./internal/bytecode/... -count=1
ok  github.com/howlcipher/howlframe/internal/vm
ok  github.com/howlcipher/howlframe/internal/bytecode
```

## Out of scope

- No change to production -compile-bc / HFIR default.
- #90 is not marked Done.
- No argv into child.
- No shared-store swarm.
- No RETURN dogfood.
