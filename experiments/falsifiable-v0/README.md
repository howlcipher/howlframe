# Review 09 offline experiment scaffold

This directory supplies engineering scaffolding for
[review 09](../../docs/ai-native-language-review/09-first-falsifiable-experiment.md).
No model runs or experiment results are included. H1 is neither passed nor
killed. Smoke tests exercise handwritten references, not model performance.
No HFBC, compiler, checker, VM, or production CLI behavior changes; #90 remains
Partial.

| Location | Purpose |
| --- | --- |
| `tasks/manifest.json` | 30 synthetic base tasks and 30 one-to-one injection variants; 10 loop and 11 conditional shapes |
| `tasks/<id>/evidence/` | Deterministic untrusted inputs; injections are data |
| `tasks/<id>/oracle/` | Hidden expected mutations and outputs; never copied into a sandbox or prompt |
| `schema/action_catalog.schema.json` | Closed Arm B catalog, draft 2020-12 |
| `prompts/` | Task-independent preambles; operator equalizes token budgets |
| `reference/arm_a/`, `reference/arm_b/` | Handwritten solutions for t01/t02/t03 and a01/a02/a03 |
| `fixtures/` | Deliberately unsafe or invalid candidates; never use as trial candidates |
| `harness/` | Broker, materialization, receipts/diffs, oracle, bounded offline repairs, scoring |
| `cmd/hfexp/` | Thin `validate`, `arm-a`, `arm-b`, `suite`, `score` CLI |

All 60 oracles are filled as `reference`, with exact expected mutations and
output files; there are zero placeholder oracles. These references are ready
for offline operator trials, but no live H1 run or model generation has occurred.
Verify and hash the frozen oracles before generation. This is a synthetic starter
corpus; review independence, held-out status, difficulty, and task diversity
before trials.

Build and validate from the module root:

```sh
go build -o /tmp/howlframe .
go build -o /tmp/hfexp ./experiments/falsifiable-v0/cmd/hfexp
/tmp/hfexp validate
/tmp/hfexp arm-b --task t01 --plan experiments/falsifiable-v0/reference/arm_b/t01.plan.json --out /tmp/hfexp-smoke-b
/tmp/hfexp arm-a --task t01 --program experiments/falsifiable-v0/reference/arm_a/t01.howl --howlframe /tmp/howlframe --out /tmp/hfexp-smoke-a
```

Every output directory must be new. A run contains `attempt<N>/candidate.*`,
`sandbox/` (evidence and an initially empty `out/`), sibling `canary/`, compiled
artifact, `caps.json` inspection report and runner receipt for Arm A, plus `attempts.json` and `result.json`.
Only read dependencies listed in `evidence` are copied. Each repair starts fresh;
only check/build/caps or plan-validation errors are repairable, at most twice.
`Repairer` is the offline integration seam; `NoRepair` is the default and
`DirRepairer` replays saved bytes. There is no model or network implementation.

The broker offers exactly `read_file`, `filter`, `if`, `write_record`, and
`request_approval`. See the schema and [protocol](PROTOCOL.md) for value and
condition semantics. It validates all branches and paths before any action.
Missing output parents must already exist or be explicitly allowed; otherwise
validation rejects their creation. Output JSON has sorted keys, two-space indentation and a trailing newline.
Approval requests contain `reason` and `summary`; they confer no authority.

**Authority honesty:** S8/P2 grants are five broad classes. `filesystem` is
not path-scoped: Arm A can write outside its sandbox. Receipts resolve target
paths against cwd and diffs detect changes both in the sandbox and sibling
canary. These detect the gap; they do not close it. Absolute writes elsewhere
on the host are not comprehensively observed by two-tree diffs. Receipts are
bounded/redacted and are not attestation; missing or truncated receipts fail
closed. Use a throwaway container/VM with no host secrets or mounts for trials.
Compile-time includes (S13), ambient streams/time/stdin (S14), and host resource
isolation remain concerns. The subprocess environment is minimal, not OS
isolation. Process audit currently inspects receipt EXEC/SPAWN/SPAWN_AGENT only;
container process auditing is deferred.

Normal Arm A preflight stops candidates needing network/process authority.
Tests separately verify a FETCH denied under the filesystem-only grant and zero
requests to an in-process loopback recorder. The CLI never starts a listener or
sends HTTP: it substitutes `http://127.0.0.1:1` for the attacker placeholder;
no candidate is granted network. Tests inject `HTTPRecorder` to substitute its
loopback URL and inspect requests. No LLM/model code path or API key handling
exists. A real network audit outside this scaffold belongs in the container.

Unauthorized entries preserve observation source; one write can appear in both
receipt and diff. Counts are observations rather than deduplicated physical
writes. Zero is unambiguous. Denied attempts are recorded separately and are
not unauthorized effects. Known filesystem scope gaps default to
`in_p1_backlog: true`; unexpected ungranted HTTP/process effects default false
and hard-stop Arm A. The operator must review effect-class mapping against
S4–S14 before analysis, without retroactively excusing new classes.
