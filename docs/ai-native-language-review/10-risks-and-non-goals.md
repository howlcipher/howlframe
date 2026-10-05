# 10: Risks and non-goals

## Top risks (ranked)

| # | Risk | Kind | Likelihood | Impact | Mitigation |
| --- | --- | --- | --- | --- | --- |
| R1 | **Authority holes undercut the core claim.** One more #44 or #57-class gap, published, ends the "intent is not authority" story. | Security / product | Medium (two found so far) | High | P0.2 conformance test. Governed profile. Receipts. |
| R2 | **"Bounded" is only instruction-bounded.** A model-proposed program can use GBs of memory or block forever with zero grants. | Security (DoS) | High (demonstrated) | Medium/High | P1.1 resource limits. Run inside an OS sandbox meanwhile. |
| R3 | **No user needs programmability over JSON plans.** The language and VM then become a large trusted computing base for no gain. | Product | Medium/High (unknown) | High | Run 09 before P2+. Accept the pivot to a broker/receipt layer if it fails. |
| R4 | **Scope creep into a general app platform** (HTTP, DOM, stores, swarm, backends) eats the solo builder's time. | Execution | High (it's where recent commits went) | High | Freeze platform growth outside HowlBoard's concrete needs (P4). |
| R5 | **Semantic divergence between interpreter, VM, and Go** changes effects (S10) and erodes "verified". | Technical | Medium | Medium | Governed profile = VM only. Differential tests for profile constructs. |
| R6 | **Overclaiming** ("verified", "AI-native", "sandbox") invites scrutiny the system can't pass yet | Reputation | Medium | Medium | P0.4 honest wording. Release Authority already has a good disclaimer. |
| R7 | **Bus factor of one and a fast-moving trusted computing base** (434 commits in 10 weeks) | Execution | High | Medium | Keep the governed core small. Rely on the existing heavy test culture (more test lines than production lines). |

## Non-goals (recommended to state in README)

- Competing with Go, JS, Python, Ruby, or Rust as a general-purpose language.
- Models emitting opcodes or bytecode ("AI writes bytecode"). The
  orchestrator's JSON-bytecode path is legacy.
- Runtime self-modification (`lazy_synthesize`, "auto-mutating runtime")
  inside a governed run.
- Being a sandbox for hostile native code. Use Wasm, gVisor, or Firecracker
  under the VM for that.
- Formal verification claims. "Verified" means "passed the named checks of
  verifier version X for profile Y".
- New DOM APIs, more backends, or a production HFIR flip as part of this
  product direction.
