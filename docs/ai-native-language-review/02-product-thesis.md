# 02: Product thesis evaluation

## Thesis as stated

> HowlFrame is a target for **AI-generated programs** whose execution is
> **constrained, verified, and auditable**. A model proposes a small
> program. Deterministic machinery decides what may run and records what
> happened. It is not "AI writes bytecode", and it does not compete with
> Go, JS, Python, Ruby, or Rust for general use.

## Steelman (for)

1. **The niche is real.** Agents increasingly need to do *more than one tool
   call*: branch on data, loop over results, transform records, then ask for
   a bounded mutation. JSON tool-call schemas handle one step. General
   sandboxes (containers, microVMs) handle hostile code but say nothing
   about *intent versus authority*. A small, typed program between those
   two is a legitimate design point.
2. **The repo already has the right shape.** Authority comes from the
   runner (`-allow-caps`, `--max-instructions`), not from the program.
   Unsupported constructs fail closed before an artifact exists. The model
   adapter cannot express grants, budgets, or oracles
   (`docs/hfir_model_adapter_status.md`, "Trust boundary").
3. **A small vocabulary helps models.** Weak evidence (3 tasks; 11
   candidates) agrees with the general finding that constrained,
   schema-checked outputs reduce invalid generations.
4. **Bounded repair is an actual differentiator.** Hash-preconditioned,
   node-local `replace_node` deltas that cannot add effects or widen
   authority (`internal/hfir/model_adapter.go:376`) are a better
   generate, verify, repair primitive than "regenerate the whole file". No
   mainstream embedded language offers this.

## Strongest case against

1. **The security value lives in the host API, not the language.** Starlark
   with host functions, CEL for predicates, OPA for decisions, or
   WASM/WASI with wasmtime fuel can sit behind the same runner policy. A
   new language adds a compiler and a VM to the trusted computing base.
2. **The boundary is not yet trustworthy.** This review found and fixed
   model calls that ran under an empty grant (bugs.md #57). Memory and
   wall-clock time are unbounded: about 100 instructions under the default
   budget allocated more than 2 GB RSS (see 07). Grants are five coarse
   classes, and `process` alone can write files through `/bin/sh -c`. The
   trusted computing base is the parser, checker, HFIR, compiler, decoder,
   and VM, all written in about 10 weeks.
3. **"Verified" means less than it sounds.** The production gate blocks on
   two diagnostic codes. There is no stack or type validation of artifacts,
   no execution receipt, and no signed provenance.
4. **No demonstrated user.** The best candidate job, the Release Authority
   / Action Executor pattern, can be built today with a JSON action schema
   and a 300-line Go executor. Nobody has shown that programmability beats
   that.

## Verdict

**Sound but unproven, and narrower than the repo's self-description.** The
thesis survives if HowlFrame is framed as:

> *a deny-by-default runtime and contract for small, model-proposed programs,
> where the model can only propose, the runner owns authority, and every run
> leaves a checkable record.*

It does **not** survive as an "AI-native language" with runtime model calls
inside the trusted path. Those features must be quarantined behind an
explicit profile (see 08 and 11).

The thesis is **falsifiable**: if model-written HowlFrame programs do not
beat JSON action plans on completion or review cost for multi-step governed
tasks, with zero unauthorized effects in both, then the language layer is not
earning its cost (see 09).

## What HowlFrame must not become

- A general-purpose language that competes on libraries, performance, or
  ergonomics.
- "AI writes bytecode". The orchestrator's JSON-bytecode generation
  (`tools/orchestrator`) is the wrong layer. Models should target the
  semantic contract (HFIR candidate transport or checked `.howl`), never
  opcodes.
- A runtime that asks a model what to do next while it holds the user's
  authority.
