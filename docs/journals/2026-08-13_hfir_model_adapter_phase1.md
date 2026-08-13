# Goal

Prove or disprove one provider-neutral, model-authored HFIR program can pass
deterministic validation, direct HFIR-to-bytecode lowering, isolated VM
execution, and a bounded semantic repair without producing `.howl` or an AST.

# Starting SHA

`f39f76049b9354008645b7bd5116a8112a4923b5` (`origin/main`, verified live).

# Hypothesis

A model can author only the executable Phase-1 HFIR transport, receive stable
structured diagnostics, and repair a declared local semantic region without
controlling authority or backend details.

# Existing executable HFIR subset

`program`, `sequence`, `let`, `set`, `if`, `const`, `symbol`, `binary`,
`list`, `dict`, `dict_entry`, conversions, list/map/string operations,
`print`, `stderr`, `exit`, and `env`, as recorded in
`docs/hfir_execution_status.md`.

# Current adapter/orchestrator state

No HFIR adapter exists. The optional orchestrator emits bytecode through an
OpenAI-compatible local experiment, so it is explicitly outside this path.

# Trust boundary

The model proposes semantic structure and constants only. The trusted caller
selects capability grants and instruction policy. Schema validation, graph
validation, verification, opcode selection, artifact validation, execution,
and behavioral oracles are deterministic-owned.

# Model-owned data

Versioned transport graph node IDs, supported node kinds, input references,
semantic values, literal kinds, and bounded provenance labels.

# Deterministic-owned data

Schema rules, graph version acceptance, decoder limits, canonical graph hash,
capability effects and grants, execution policy, verifier/lowerer rules,
bytecode opcodes, artifact acceptance, and all test oracles.

# Non-goals

No provider implementation, public build-path switch, AST fallback, broad
HFIR coverage, content-addressed storage, full patch system, HTTP, stores,
functions, loops, recovery, or experience memory.
