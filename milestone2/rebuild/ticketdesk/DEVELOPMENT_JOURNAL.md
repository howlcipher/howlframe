# Fresh Ticketdesk development journal

2026-10-20. Root implementer: Codex. Independent evaluator: the separate
`milestone2_evaluator` agent in this session. No cross-provider claim is made.

The user explicitly rejected treating Cursor's previous work as a trustworthy
baseline and requested new implementations from the frozen requirements. The
initial reassessment and preliminary CLI fixes were insufficient for that request;
all four CLI/backend candidates were then independently rewritten in `rebuild/`.
The old records and original frozen requirements/tests remain intact.

Design alternatives were a full-state record (simple atomicity, costly every-read
scans), a separately persisted live index (the baseline's crash windows), or
bounded ticket buckets containing both records and order. The new implementation
uses atomic buckets; token state uses a single atomic collection with a linear
hash scan. This prioritizes correctness over large-token-set lookup throughput.

The first build exposed unsupported `trim`, collection operations requiring
symbol bindings, and unbalanced parentheses; these were corrected without
compiler changes. The first frozen run passed 100/101; the debug-header check
had to precede method/auth checks on known routes. The independent evaluator
then caught a handcrafted debug failure: it now raises a genuine conversion
fault, caught and logged by the application.

The evaluator also reproduced corrupt-state acceptance and identified a crash
window in an intermediate three-write token design. That intermediate design was
replaced, rather than weakening the test: token records/order/revocation now
persist together. Full raw-document validation runs before database acquisition
and mutation, allowing inspectable data errors to precede missing database grants.
Moving bucket-key formatting out of each entity check restored default-budget
startup for 150 maximum-Unicode records.

Runtime repairs are narrowly demonstrated by retained tests: explicit listener
IP selection; pre-buffered read failures remain errors when `req_body` is read,
allowing route/method/auth checks to retain precedence; ignored GET/DELETE bodies
do not block serialized servers. Error IDs, safe body shape and access logging
survive truncated uploads. Body-validating handlers never receive partial bytes
as complete JSON. Generic handlers that ignore a POST body may ignore its framing
error too; no universal claim of preventing arbitrary handler side effects is made.

Root verification: frozen 101/101 and retained 11/11 pass. The last retained crash
test was strengthened after atomic token publication made deletion of an old
digest-index key vacuous. Python 3.14's shared harness could swallow BrokenPipe
then construct HTTPResponse(None) during intentional kills; crash workers now
use a separate transport that retains OSError rather than masking it. No
functional or durability assertion was removed. The historical failed traces
remain in the review narrative.

Independent review and committed clean-environment verification are recorded
separately. No historical accepted label is used as fresh evidence.
