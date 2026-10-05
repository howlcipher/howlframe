# Independent review: HowlFrame AI-native product direction

You are one of several INDEPENDENT reviewers. You will not see the other reviewers' answers and they will not see yours. Skepticism is rewarded; flattery is penalized. If the evidence says the idea is weak, say so plainly.

## Repository
Your current working directory is a clean checkout of https://github.com/howlcipher/howlframe at commit d496d972 (current main). Treat it as READ-ONLY: do not edit, create, or delete tracked files, do not commit, do not push, do not open PRs. You MAY run read-only commands (ls, rg/grep, git log, go build/go test, building the CLI to /tmp and running it on small /tmp programs). If you build or test, set GOCACHE=/tmp/hf-ainative-gocache-$ROLE and write any scratch files only under /tmp/hf-ainative-scratch-$ROLE/.

You MUST inspect the real implementation before forming opinions; do not rely on README claims alone. At minimum look at: README.md, AGENTS.md, improvements.md (skim), bugs.md (skim), docs/architecture_roadmap.md, docs/ai_native_paradigms.md, docs/hfir_execution_status.md, docs/hfir_model_adapter_status.md, docs/reference/lowered_hfir_prod_flip_criteria.md, howlframe.go, internal/capability/, internal/bytecode/opcode.go, internal/bytecode/artifact.go, internal/construct/construct.go, internal/hfir/ (verifier.go, model_adapter.go, manifest.go, storage.go, incremental.go), internal/vm/vm.go, apps/release_authority/, apps/action_executor/, contracts/howl/, and `git log --oneline -40`. Cite evidence as path:line wherever you make a factual claim. Mark each claim as VERIFIED (you saw it in code/tests or ran it) or INFERRED.

## Product thesis under evaluation
HowlFrame should NOT try to be a general-purpose language competing with Go/JS/Python/Ruby/Rust, and the thesis is NOT "AI writes bytecode". The thesis is: HowlFrame is a target for AI-GENERATED PROGRAMS whose execution is constrained, verified, and auditable — the model proposes a small program; deterministic machinery (checker, HFIR verifier, construct registry, capability grants, instruction budgets, manifests) decides what may run and records what happened. Slogan in the repo: "intent is not authority".

Standing constraints (do not recommend violating them casually): do not flip the production -compile-bc default to the HFIR lowering path; improvement #90 stays Partial; do not invent DOM APIs; do not redesign the language wholesale or casually break artifact compatibility.

## Questions (answer every one, numbered, concisely, with evidence)
1. What is HowlFrame, factually, today (what actually runs end-to-end)? What is merely documented/aspirational?
2. Based on the last ~40 commits and docs, what is it already becoming?
3. Is the "constrained/verified/auditable execution for AI-generated programs" thesis sound? Strongest argument for and against.
4. Who would actually use this, for what job, instead of what alternative?
5. What is the trusted computing base, and is it trustworthy today? Name concrete gaps.
6. Does the .howl surface syntax matter for this thesis, or should models target HFIR/JSON directly? What should the model-facing contract be?
7. How does it compare with WASM/WASI (+component model), Starlark, CEL, OPA/Rego, Lua, Rhai, Deno permissions, gVisor/Firecracker-style sandboxes? What is genuinely differentiated vs. reinvented?
8. What existing features conflict with the thesis and should be quarantined, cut, or redesigned?
9. Top 5 risks (technical, security, product), ranked.
10. What should the capability/authority model become (granularity, resource scoping, budgets, model calls as effects)?
11. What does "verified" concretely mean here — what can be statically checked, what must be runtime-enforced, what cannot be claimed?
12. A prioritized roadmap P0 (now) .. P4 (later/never), each item with rationale and size (S/M/L).
13. The single first falsifiable experiment: hypothesis, setup, metric, pass/kill criteria, cost.
14. If you were advising the owner (a solo builder) for the next 2 weeks, what exactly should he do and NOT do?

## Output format
Markdown, 1500–3000 words. Start with a 5-line executive summary. Then the numbered answers. Then your role-specific section (below). End with "Confidence and blind spots" listing what you did not verify.
