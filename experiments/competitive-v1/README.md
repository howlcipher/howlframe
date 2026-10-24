# competitive-v1

Preregistered design for comparing AI-agent software delivery in HowlFrame against Python, Go and JavaScript.
**Draft (Revision 3), not frozen; no trial has run, and no paid model call has been made.** The protocol is
`HOWLFRAME_COMPETITIVE_PROTOCOL.md`; `parameters.json` holds the values to be fixed at freeze, with `null` for everything
not yet decided. `harness/` is the experiment-only infrastructure (proxy, jails, runner, scorer, draw), validated against
mock providers only; `prompts/agent_task.md` is the single prompt template for every arm. Results, when they exist, go to
`HOWLFRAME_COMPETITIVE_RESULTS.md` in this directory, including negative and incomplete outcomes.

This differs from `experiments/live-v1` (PR #178), which compares HowlFrame with a JSON action-plan representation
on filesystem tasks, not with general-purpose languages.
