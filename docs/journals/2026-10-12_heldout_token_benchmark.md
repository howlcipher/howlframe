# Held-out token benchmark (2026-10-12)

## Goal

Test whether the 0.983x-against-Python result from #135 holds on tasks the
built-ins were not shaped for (Lain's rules). No language changes.

## Freeze

The spec commit is **`f510cd0199326e4aacf206e0b08981978808b042`**:
"heldout-v0: freeze 32 held-out task specs (no solutions)". It was made
before any reference was written or anything was measured.

| File | SHA-256 |
|---|---|
| tasks/README.md | `4441e1e5c4e906a55fe595c382a7d2ef784045e9c1b1230f38887d70bf8318f1` |
| tasks/build_specs.py | `dcea7d494d8b146ae032456b1f8822364d488c3d8596f7d829aecb21a75148f8` |
| tasks/manifest.json | `962fa8198b11ac3ff1066abf35be6dcb87ad8a852574edfc46fee17a0ab9e67e` |

`TestHeldoutSpecsFrozen` enforces these hashes.

## Built after the freeze

- `cmd/heldoutverify`:
  - runs each reference in a fresh sandbox;
  - HowlFrame uses a governed build, `-run-bc` and scoped `filesystem:read`/`write` grants;
  - requires the file set to be exactly the inputs plus the output, the inputs unchanged, and the output matching.
- `count_tokens.py` imports v0's stripping and offline guard unchanged.
- `run.sh`, `gaps.json` and `verification_test.go`.
- 32 references each in Python, JS and Go, and 30 in HowlFrame. The other 2 are recorded gaps.

## Result

| | HowlFrame / Python, median per-task ratio | Wins / ties / losses |
|---|---|---|
| cl100k_base | **1.309x** | 0 / 1 / 29 |
| o200k_base | **1.299x** | 0 / 1 / 29 |

Against JS: 1.087x / 1.016x. Against Go: 0.499x / 0.493x.

**The v0 result (0.983x) does not generalize.** CSV is worst at 1.84x and reshape best at 1.08x.

## Gaps and findings

- **h31/h32:** no directory walking. h32 would also need string ordering.
- **Sugar not expanded in some arguments:** sugar inside `write_file`/`to_lower`
  arguments fails to lower, because those parents are missing from the sugar contexts.
- **`or` is not short-circuit.**
- **`map_get` on a missing key returns `""`.**
- **Value semantics:** lists taken out of a map must be written back.

## Next (not done here, needs its own poll)

These are the hot spots, but adding them would again shape features on a
corpus. Any language work should be judged on a *new* held-out set.

- `index_of`/slice;
- counting or default `map_get`;
- `map`/`filter`/`sum`/`min`/`max`/`reverse`;
- dir listing.

There were no live LLM runs and no H1 decision; #90 is unchanged, and so are HFIR and `-compile-bc`.
