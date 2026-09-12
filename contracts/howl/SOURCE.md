# Vendored `howl.*` contract schemas

The five `.schema.json` files in this directory are vendored, byte-for-byte
copies of the generated JSON Schema published by HowlDream. They are the
authoritative shape of the `howl.*` ecosystem envelopes this repo consumes
and produces (`howl.candidate/v1` in, `howl.assessment/v1` out).

**Source repository:** https://github.com/howlcipher/howldream
**Pinned commit:** `bd10b18b7182e5216b494d728d015dbbba9b32d0`
**Source path:** `schemas/*.schema.json`
**Vendored:** 2026-09-12

## Why vendored, not fetched at build/runtime

See `howldream`'s `schemas/README.md` for the full comparison of distribution
models. Summary: all producing/consuming repositories are owned by the same
org with full git access, so a pinned, vendored copy (re-vendored as a
deliberate, visible diff) is the simplest architecture justified by current
scale — no live network fetch, no new shared package/repo.

## How to re-vendor

```sh
cp /path/to/howldream/schemas/howl.*.schema.json contracts/howl/
```

Then update the pinned commit above to the exact `howldream` commit the
copied files came from. Re-vendoring is a deliberate act, not automatic —
bumping the pin should be its own visible diff, so a schema change is never
silently absorbed.

## What this proves, and what it doesn't

`contract_test.go` in this package validates real envelope fixtures against
these vendored schemas using a pure-Go JSON Schema validator
(`github.com/santhosh-tekuri/jsonschema/v5`) — **no Python, no `howldream`
import, no network access at test time.** This is the cross-language contract
test referenced in `howldream/issues.md` item 1: proof that the schema is
useful to a consumer that has never seen HowlDream's Python implementation.

As documented in `howldream`'s `AUTHORITY_INVARIANT.md`, schema validation
proves an envelope is *shaped* correctly — it does not prove the envelope's
claims are true. HowlFrame's own `apps/candidate_evaluator` is the runtime
control that independently evaluates candidate claims; it does not trust a
candidate's self-reported `trust`/`status`/`disposition` fields.
