# Milestone 3: EvidenceDesk and a real HowlBoard projector replacement

Status: independently selected and frozen before implementation, 2026-10-20.
Authority: HOWLFRAME_PRODUCT_DIRECTIVE.md sections 6 and 8, D30 A, B, J.

EvidenceDesk is a local evidence-ledger inspection application: import a bounded
HowlPlane JSONL slice, inspect task history without confusing claims with passed
checks, and keep persistent reviewer notes. Both backend and interactive browser
logic are HowlFrame. A companion standalone HowlFrame projector replaces the
useful export responsibility of HowlBoard's existing ledger_import tool, with
strict validation and an exact entry limit. Actual sibling HowlBoard must consume
its native stores; fixture-only assertions do not establish adoption.

## Sequence and architecture decision

Milestone 1 supplies proven CLI/file workflows; Milestone 2 supplies HTTP,
validation, persistence and request diagnostics. First implement and verify the
bounded projector/parser contract, then the API, then web_app interaction, then
prove consumption by the real sibling Board. These are two D30 proofs delivered
by one cohesive package, not extra numeric product gates.

Alternatives considered:

* Extend sibling HowlBoard wholesale: existing useful full-stack app, but would
  mix unrelated mission authority and lifecycle behavior into this bounded task.
* Build a fresh generic task manager: simpler domain, but repeats Ticketdesk and
  provides no genuine ecosystem component.
* EvidenceDesk plus a Board-compatible projector (selected): independently useful
  review workflow and a real existing consumer; cost is maintaining the narrow
  projection compatibility surface. Keep all app/framework code under
  milestone3/evidencedesk; core changes require demonstrated application blockers.

Sources inspected: sibling howlboard/README.md, tools/ledger_import/ledger_import.howl,
backend/server.howl, data/fixtures/ledger_sample.jsonl, and sibling
howlplane/schemas/evidence-entry.schema.json. Existing importer silently ignores
malformed JSON and its `seen > limit` boundary processes one too many entries.
It is already HowlFrame: claim a useful component replacement, not first adoption.

Requirements and independent acceptance are in evidencedesk/requirements/.
The source and tests are frozen before implementation in a dedicated commit.
The user authorized all three milestones; stop after independently accepted D30
proofs and evidence closure, with no follow-on optimization or unrelated work.
