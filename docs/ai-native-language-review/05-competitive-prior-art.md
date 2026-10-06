# 05: Competitive and prior-art review

The comparison question is not "is HowlFrame a better language than X". It
is **"for a model-proposed multi-step program that must run under
runner-owned authority and leave a checkable record, what does each option
give you, and what would you have to build?"**

Ratings: ● strong, ◐ partial, ○ absent or not the goal. Ratings for
HowlFrame describe `d496d972` plus the #57 fix in this PR.

| System | Determinism | Termination / cost bound | Capability model | Static checks before run | Audit / provenance | Model-generation fit | Maturity |
| --- | --- | --- | --- | --- | --- | --- | --- |
| **HowlFrame (today)** | ◐ (clock, sleep, model calls, and stdin are ambient) | ◐ instruction count only. No memory, wall-clock, or output bound. | ◐ 5 coarse classes, runner-owned, deny by default | ◐ checker + construct registry + 2 blocking HFIR codes | ◐ deterministic artifact + hash. Manifest in library only. C5 adds optional runner-written bytecode receipts; signing/attestation still open. | ◐ small grammar. JSON graph transport prototype with bounded repair. | ○ ~10 weeks, one author |
| **WASM + WASI 0.2 / component model (wasmtime)** | ◐ (deterministic core; host imports decide) | ● fuel and epoch interruption, `ResourceLimiter` for memory and tables | ● capability-based imports. Preopened dirs. WIT-typed interfaces. | ● module validation (type-checked stack machine) | ◐ module hashes. Signing via external tooling. | ○ models don't write Wasm. They write a source language that compiles to it. | ● |
| **Starlark (Bazel, starlark-go)** | ● hermetic by design. No I/O unless the host injects it. Frozen values. | ◐ `SetMaxExecutionSteps` in starlark-go. Bazel forbids recursion. | ◐ host-injected builtins are the only effects | ◐ parse/resolve. Dynamic types. | ○ | ● Python-like, so models write it well | ● |
| **CEL** | ● | ● non-Turing-complete. Static cost estimation plus runtime cost limit. | ● host provides all data. No effects. | ● type-checked expressions | ○ | ● for predicates. ○ for workflows. | ● (Kubernetes admission, IAM conditions) |
| **OPA / Rego** | ● (except `http.send`) | ◐ | ◐ decision only. The executor is someone else's code. | ◐ | ● decision logs, signed bundles | ◐ Datalog-like, harder for models | ● |
| **Lua / Luau** | ◐ | ◐ instruction hooks, interrupt callbacks, allocator limits | ◐ sandbox by removing globals. Luau has a sandbox mode. | ○ (Luau has an optional type checker) | ○ | ● | ● |
| **Rhai** | ◐ | ● max operations, call depth, string/array/map sizes, expression depth | ◐ host-registered functions only | ○ | ○ | ◐ | ◐ |
| **Deno permissions** | ○ | ○ | ● **resource-scoped** `--allow-read=PATH`, `--allow-net=HOST`, `--allow-run=CMD`, `--allow-env=VAR` | ◐ TypeScript | ○ | ● TS | ● |
| **eBPF verifier** | ● | ● statically proven termination (bounded loops) | ● helper-function allow-lists per program type | ● abstract interpretation of every path, bounds, pointer safety | ○ | ○ | ● |
| **Dhall / CUE / Nickel** | ● | ● Dhall is total | ● Dhall imports are pinned by semantic hash | ● | ◐ integrity hashes | ◐ | ◐ |
| **JSON function calling / MCP tools** | n/a | ● one call | ◐ per-tool, host-side | ● schema validation | ◐ host logs | ● | ● |
| **Cloudflare Code Mode (V8 isolates)** | ◐ | ◐ isolate CPU and memory limits | ● no network by default. Effects only through host bindings. | ◐ TS types | ○ the docs say it does **not** give durable per-call approval | ● TS | ◐ experimental |
| **CaMeL (DeepMind, 2025)** | ◐ | ○ | ● per-value capabilities **plus data-flow (provenance) tracking**, policies checked before each tool call | ◐ restricted Python subset | ◐ data-flow graph | ● restricted Python | ○ research artifact |
| **smolagents CodeAgent / restricted Python** | ○ | ◐ op counts | ◐ import allow-lists | ○ | ○ | ● | ◐ |

## What this says about HowlFrame

**Reinvented and currently weaker than prior art:**

- *Sandboxing and resource limits.* wasmtime, Rhai, Luau, and V8 isolates
  all bound memory and time. HowlFrame bounds only instruction count.
- *Capability granularity.* Deno and WASI scope grants to paths, hosts,
  commands, and variables. HowlFrame grants a whole class (`filesystem`
  means every path).
- *Static verification.* eBPF and Wasm validate every artifact before
  execution. HowlFrame's artifact validation checks opcodes and jumps, not
  stack or type discipline.
- *Policy decisions and logs.* OPA has mature decision logs and signed
  bundles.

**Genuinely differentiated (if finished):**

1. **Proposal versus authority in one artifact contract.** Most systems
   give you either a sandbox (Wasm, isolates) or a policy decision (OPA,
   CEL). HowlFrame's design puts the *proposal* (a program),
   *authority* (runner grant and budget), and *record* (manifest, receipt)
   around one deterministic artifact. Code Mode explicitly leaves durable
   per-call approval to the host. That gap is the opening.
2. **Bounded semantic repair.** `replace_node` deltas with graph and node
   hash preconditions that cannot add effects or widen authority
   (`internal/hfir/model_adapter.go`). No mainstream embedded language has
   a model-facing repair protocol.
3. **Fail-closed construct registry.** Unsupported constructs are rejected
   before an artifact exists. That is a useful property for generated
   code, though Starlark and CEL get similar safety by having no unsafe
   constructs at all.

**Closest prior art: CaMeL.** It uses an LLM-written program in a custom
interpreter with capabilities checked before each tool call. Its key extra
idea is **data-flow / provenance tracking**: values derived from untrusted
tool output cannot steer control flow or reach sensitive sinks. HowlFrame
has nothing comparable yet. If HowlFrame's audience includes agents reading
untrusted data, provenance-aware policy is the strongest idea to adopt.

## Ideas to steal (ranked by leverage and cost)

1. **Resource-scoped grants** (Deno, WASI preopens): `filesystem:read=/data`,
   `network=api.example.com`, `process=git`. Keep the coarse names as
   aliases.
2. **Memory, wall-clock, and output limits** (wasmtime `ResourceLimiter` and
   epochs, Rhai size limits).
3. **Static cost or required-capability report before running** (CEL cost
   estimation). HowlFrame can compute required capabilities from opcodes and
   store URIs today (see 11, C1).
4. **Decision and execution logs** (OPA decision logs) as an execution
   receipt bound to the artifact hash and grant.
5. **Provenance-tagged values** (CaMeL) for untrusted inputs such as
   `req.body`, `fetch` results, model outputs, and `read_file`.
6. **Pinned imports by hash** (Dhall) for `include` / `use` in
   model-authored source.
7. **Full artifact validation** (Wasm and eBPF style stack and type checks)
   before `-run-bc` trusts an artifact that came from elsewhere.

## Where HowlFrame should not compete

- With Wasm as a portable sandbox or native target. If isolation from
  hostile native code is needed, run the HowlFrame VM *inside*
  Wasm/gVisor/Firecracker. Don't rebuild them.
- With CEL for predicates or OPA for policy decisions. Embed or interoperate.
  For example, an approval check could be a CEL expression the runner
  evaluates, not HowlFrame code.
- With TypeScript or Python on model fluency. Models will always write
  those better. HowlFrame's argument has to be *checkability and authority*,
  not fluency.

Sources: wasmtime docs (fuel, epoch interruption, `ResourceLimiter`); WASI
0.2 / component model docs; starlark-go spec and `Thread.SetMaxExecutionSteps`;
cel.dev (cost estimation); OPA docs (`http.send`, decision logs, bundle
signing); Lua 5.4 manual (`debug.sethook`); Luau sandboxing docs; Rhai book
"Safety" chapter; Deno permissions docs; Linux kernel eBPF verifier docs;
Dhall import integrity docs; Cloudflare blog "Code Mode" and Agents docs
(per-call approval note); Debenedetti et al., *Defeating Prompt Injections
by Design* (arXiv:2503.18813). Two independent reviewers (Codex A and B)
cited primary docs for WASI, Starlark, CEL, OPA, Lua, Rhai, Deno, gVisor,
and Firecracker, and their positioning matches this table.
