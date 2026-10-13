<!-- STUB MODEL DRY RUN (tuning set, rep1). Not experiment evidence. -->
# live-v1 results (model: stub/dry-run, status: complete)

commit 11dcdfe8dde89a14374b9d6db63a75851ede0a6c · harness tree 7e0e29f578ac50c5fc322609d7abd3a3d384ab10 · howlframe f8d2292cd063265d8d8dd34f72bfeebb557792ac6ba9dead602c6bdaa5e2daf9 · prereg 1faa850222d7a516435b04eb4ca16f3df61847306975b4231de23b803aecd7ec

Arm A = HowlFrame, Arm B = JSON action plans. Computed from every recorded run; nothing excluded.

## Set: tuning — tuning corpus only (falsifiable-v0; see contamination disclosure)

**Decision: KILL** (delta 0.0pp <= 5pp)

Primary: paired success delta A−B = 0.0pp, 95% cluster-bootstrap CI [0.0, 0.0] over 30 families; McNemar exact p = 1.000 (only A 0, only B 0, both 5, neither 55).

| metric | HowlFrame (A) | JSON plans (B) |
|---|---|---|
| runs | 60 | 60 |
| success rate | 8.3% (5) | 8.3% (5) |
| refused up front | 1 | 1 |
| refused at runtime | 0 | 0 |
| escapes | 0 | 0 |
| deadline kills | 0 | 0 |
| model errors | 0 | 0 |
| repairs exhausted | 54 | 54 |
| mean repair rounds | 1.83 | 1.83 |
| mean repair diagnostic bytes | 134 | 163 |
| median model tokens (incl. cache) | 1335.0 | 1775.0 |

Review effort (secondary), paired, both succeeded: A 5 cells, B 5 cells

| median | A | B |
|---|---|---|
| cl100k tokens | 160.0 | 235.0 |
| lexical tokens | 125.0 | 119.0 |
| lines | 13.0 | 43.0 |
| bytes | 578.0 | 802.0 |
| gzip bytes | 295.0 | 307.0 |
| receipt effects | 4.0 | 4.0 |
| distinct effects | 4.0 | 4.0 |
| effects missing vs oracle | 0.0 | 0.0 |
| effects extra vs oracle | 0.0 | 0.0 |

Review effort (secondary), all final candidates: A 60 cells, B 60 cells

| median | A | B |
|---|---|---|
| cl100k tokens | 2.0 | 2.0 |
| lexical tokens | 2.0 | 2.0 |
| lines | 1.0 | 1.0 |
| bytes | 12.0 | 12.0 |
| gzip bytes | 36.0 | 36.0 |
| receipt effects | 0.0 | 0.0 |
| distinct effects | 0.0 | 0.0 |
| effects missing vs oracle | 1.0 | 1.0 |
| effects extra vs oracle | 0.0 | 0.0 |

### Where HowlFrame lost (B succeeded, A did not): 0


### Where HowlFrame won (A succeeded, B did not): 0

