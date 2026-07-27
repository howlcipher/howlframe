# Auto Patching Loop (#59)

**Date**: 2026-07-27

## Objective

Implement improvement #59 as a bounded, opt-in observer workflow that reacts
to `crash.json`, asks the configured local LLM for a complete replacement
candidate for one explicitly configured `.zero` source file, verifies the
candidate in an isolated temporary project copy, atomically installs it only
after the configured test command succeeds, and restarts the configured
service only after installation.

## Architecture Decision

Three approaches were evaluated:

1. Whole-file replacement in place is simple, but has a large failure window
   and weak rollback behavior.
2. Unified diff application is auditable, but introduces patch parser
   complexity and makes path traversal or multi-file edits easier to request
   accidentally.
3. A complete replacement candidate verified in a temporary project copy
   constrains the model to one file, keeps the live source untouched during
   verification, and permits an atomic final write.

Use option 3. Commands are explicit argument vectors, never shell strings.
Source and crash paths must resolve inside the configured project root.
Malformed model output, failed tests, or failed installation must leave the
live source unchanged. Restart is permitted only after successful tests and
installation.

## Test First Contract

Add isolated Python tests for:

1. Rejecting paths outside the project root.
2. Rejecting malformed model output without changing the source.
3. Preserving the source and suppressing restart when candidate tests fail.
4. Installing the candidate and restarting exactly once when tests pass.

## Delegation

Pending. Use a live non-Claude Antigravity model after the failing tests and
this journal are committed. The delegate edits implementation and
documentation only; verification and commits remain in this session.

## Next Step

Add the failing unit tests, commit this milestone with a signed commit, then
delegate the observer implementation against the test contract.
