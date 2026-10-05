# 01: What HowlFrame is already becoming

## The history, read as a trajectory

| Period | Dominant work | What it optimized for |
| --- | --- | --- |
| Jul 23 to Jul 31 | Go transpiler, `web_app` JS, WAT; many "AI-native" primitives (`semantic_match`, `fuzzy_cast`, `lazy_synthesize`, `achieve`, `neural_circuit`, `ephemeral_circuit`, `spawn_agent`); write-cost benchmark | An AI-friendly *source language* that generates Go |
| Aug 1 to Aug 14 | HFIR v1, verifier, manifests and CAS, model adapter, semantic repair, failure localization, runtime capability enforcement, instruction budgets, standalone `.hfbc` artifacts, v0.1 | A *verification and authority layer* around execution |
| Aug 15 to Sep 30 | VM hardening (structured errors, type errors, request surface, html escaping, absence idiom), HowlBoard dogfooding, lowered-HFIR ABI phases, prod-flip criteria, Assurance tip-lock `4d74dbcf` | A *standalone runtime* good enough to host real apps |
| Oct 1 to Oct 5 (#89 to #103) | Every named root writes `.hfbc` and stops. No gogen fallback on the default path. TASK and SPAWN_AGENT bodies run in the VM. Child-failure isolation. Fail closed on malformed body lengths. | *One* execution substrate (the HowlFrame VM) with fail-closed edges |

## Plain reading

HowlFrame is no longer mainly an S-expression language that compiles to Go.
In the last two months of commits it has become:

1. **A capability-gated bytecode VM with runner-owned policy.** The program
   cannot raise its own budget or grant itself capabilities. That is the
   center of gravity in the code.
2. **A fail-closed compiler front end for that VM.** The checker, construct
   registry, and HFIR gate all mean "if we can't express it safely, refuse
   before writing an artifact".
3. **A prototype model-facing contract** (HFIR candidate transport plus
   bounded repair deltas with hash preconditions). It is real and
   well-tested, but it is only a library and covers a narrow subset.
4. **A small application platform** (HTTP server, native store,
   cooperative agents, `web_app` JS). This is where most of the friction and
   dogfood work goes.

## Tension the code already shows

- Items 1 to 3 point at the *governed execution of AI-generated programs*
  thesis. Item 4 points at a *general app platform*, which the brief rules
  out as the goal.
- The early runtime AI primitives point the opposite way from items 1 to 3.
  They put model calls, and in one case model-written code, *inside* the
  trusted execution. `lazy_synthesize` compiles model output at runtime and
  skips the checker, HFIR, and the construct registry.
  `docs/extreme_ai_paradigms.md` describes an "auto-mutating runtime" and
  "stochastic control flow" as goals. Both contradict "intent is not
  authority".
- `docs/architecture_roadmap.md` still says the capability vocabulary is
  "advisory", but runtime enforcement exists. It also still says HFIR
  should become the canonical artifact, while HOWL-CANON-010 froze HFIR as a
  gate. The docs trail the code in some places and run ahead of it in
  others.

## Conclusion

The code is *already becoming* a governed runtime for small programs. The
product question is not whether to pivot to that. It is whether to **stop
spending on item 4 and on the runtime-AI primitives** and spend on what makes
item 1 trustworthy and item 3 usable. Three things make it trustworthy:
complete effect gating, resource limits, and receipts. One thing makes it
usable: a CLI-reachable propose, check, inspect, grant, run, audit loop.
