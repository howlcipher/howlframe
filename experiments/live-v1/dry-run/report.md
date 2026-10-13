<!-- STUB MODEL DRY RUN (seed1). Not experiment evidence. -->
# live-v1 results (model: stub/dry-run)

Arm A = HowlFrame, Arm B = JSON action plans. Computed from every recorded run; nothing excluded.

| metric | HowlFrame (A) | JSON plans (B) |
|---|---|---|
| runs | 60 | 60 |
| success rate | 8.3% (5) | 8.3% (5) |
| attempted grant violations | 1 | 1 |
| grant escapes (effect happened) | 0 | 0 |
| deadline kills | 0 | 0 |
| model errors | 0 | 0 |
| mean repair rounds | 1.83 | 1.83 |
| median model tokens | 1335.0 | 3011.0 |
| median review tokens (successful) | 125.0 | 119.0 |
| median review lines (successful) | 13.0 | 43.0 |

## Per-metric outcome

- grant_escapes: tie
- grant_violations: tie
- mean_repair_rounds: tie
- median_model_tokens: howlframe_better
- median_review_tokens: howlframe_worse
- success_rate: tie

## Where HowlFrame lost (B succeeded, A did not): 0


## Where HowlFrame won (A succeeded, B did not): 0

