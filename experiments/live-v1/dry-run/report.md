# live-v1 results (model: stub/dry-run, status: complete)

generated from commit bd2190d7b3656eddcbe4c3130f50248d92cd38a8 · harness tree 752aa697180e807134c0422784561433b67eb2f9 · howlframe 8dfefec4346a8191c57125caf48a23cf14eeb338ff5aa2ba401798f44c1c2b45 · prereg ec91c50911660bd1885f2ef4f547425626fd428b8fe612a54497b60dbbaf90f5

Arm A = HowlFrame, Arm B = JSON action plans. Computed from every recorded run; nothing excluded.

## Set: blind — blind set (live-v1-blind)

_Stub dry run: no decision is stated._

Primary: paired success delta A−B = 0.0pp, 95% cluster-bootstrap CI [0.0, 0.0] over 27 families; McNemar exact p = 1.000 (only A 0, only B 0, both 0, neither 27).

| metric | HowlFrame (A) | JSON plans (B) |
|---|---|---|
| runs | 27 | 27 |
| success rate | 0.0% (0) | 0.0% (0) |
| refused up front | 0 | 0 |
| refused at runtime | 0 | 0 |
| escapes | 0 | 0 |
| in-grant forbidden (fail, not KILL) | 0 | 0 |
| deadline kills | 0 | 0 |
| model errors | 0 | 0 |
| repairs exhausted | 27 | 27 |
| mean repair rounds | 2.00 | 2.00 |
| mean repair diagnostic bytes | 146 | 178 |
| median model tokens (incl. cache) | 2100.0 | 1640.0 |

Review effort (secondary), paired, both succeeded: A 0 cells, B 0 cells

| median | A | B |
|---|---|---|
| cl100k tokens | n/a | n/a |
| lexical tokens | n/a | n/a |
| lines | n/a | n/a |
| bytes | n/a | n/a |
| gzip bytes | n/a | n/a |
| receipt effects | n/a | n/a |
| distinct effects | n/a | n/a |
| effects missing vs oracle | n/a | n/a |
| effects extra vs oracle | n/a | n/a |

Review effort (secondary), all final candidates: A 27 cells, B 27 cells

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


## Set: tuning — tuning corpus only (falsifiable-v0; see contamination disclosure)

_Stub dry run: no decision is stated._

Primary: paired success delta A−B = 0.0pp, 95% cluster-bootstrap CI [0.0, 0.0] over 30 families; McNemar exact p = 1.000 (only A 0, only B 0, both 5, neither 55).

| metric | HowlFrame (A) | JSON plans (B) |
|---|---|---|
| runs | 60 | 60 |
| success rate | 8.3% (5) | 8.3% (5) |
| refused up front | 1 | 1 |
| refused at runtime | 0 | 0 |
| escapes | 0 | 0 |
| in-grant forbidden (fail, not KILL) | 0 | 0 |
| deadline kills | 0 | 0 |
| model errors | 0 | 0 |
| repairs exhausted | 54 | 54 |
| mean repair rounds | 1.83 | 1.83 |
| mean repair diagnostic bytes | 134 | 163 |
| median model tokens (incl. cache) | 2235.0 | 1775.0 |

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

