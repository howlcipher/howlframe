# R5 integration compatibility clarification, 2026-10-20

> **Superseded 2026-10-22 (Owner decision D).** Kept as a historical record only; it is not a current compatibility
> requirement. It was approved by the implementing orchestrator, not the Owner, and was written against a stale HowlBoard
> checkout (`cd2f2fd`). Current HowlBoard declares its own root grants, so applying this adapter to it adds a second
> `(grants ...)` form and fails to compile. See [../../CORRECTION_2026-10-22.md](../../CORRECTION_2026-10-22.md).

Approved by the parent agent within the user's all-milestone implementation
scope before the adapted sibling integration is accepted. Independent selector:
agent milestone3_requirements. Original REQUIREMENTS.md remains frozen intact.

The original R5 predicted that compiling the actual sibling HowlBoard consumer
would require no sibling modifications. Live execution disproved that assumption:
its current source lacks root route-capability declarations, and the current
HowlFrame runtime correctly attenuates undeclared route capabilities, so its
mission routes fail with database capability denial even when the runner has
database/filesystem grants. Preserve that failed original attempt in evidence.

R5's real-consumer test may compile a temporary copy of the actual sibling
backend/server.howl with one explicit root `(grants "database"
"inherit:filesystem")` declaration added. This is solely a capability-contract
compatibility adapter. No mission/projection/domain behavior may change, no
runtime default may be weakened, and the sibling working tree remains untouched.
The runner must still grant the database and scoped filesystem authority used
by the actual consumer. No other authority is implied.

Record sibling commit, original source SHA-256, exact deterministic patch,
adapted source SHA-256, compiler commit/version, grants, failed original execution,
and successful adapted mission/timeline requests. Source fixtures and production
stores remain read-only; use temporary output stores. Explicitly identify the
consumer as adapted for current route declarations; do not claim unmodified
sibling compatibility, first HowlFrame adoption, or cross-provider independence.

This clarification changes only the integration build procedure; all original
projector semantics, actual mission/timeline assertions, browser, capability,
reproduction and independent review gates remain binding.
