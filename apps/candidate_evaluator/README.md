# Candidate Evaluator

Capability-bounded candidate evaluation application for the Howl ecosystem.

Demonstrates HowlFrame independently auditing speculative candidates from HowlDream:
- Intent is not authority: candidates claiming execution authority or self-approval are categorically rejected.
- Candidates with critical contradictions or verification failures are rejected.
- Grounded candidates with verified constraints are accepted for deliberate development.
- Inconclusive or ungrounded candidates remain unresolved.

## Compiling to Standalone Bytecode

```bash
howlframe -compile-bc candidate_evaluator.howl -o candidate_evaluator.hfbc
```

## Running

```bash
howlframe -run-bc -allow-caps filesystem candidate_evaluator.hfbc /path/to/candidate.json
```

Outputs structured evaluation conforming to `howl.assessment/v1`.
