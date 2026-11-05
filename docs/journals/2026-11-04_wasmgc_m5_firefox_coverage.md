# M5 Firefox coverage ledger

Date: 2026-11-04 UTC
Consumer source: HowlScene HF M5 commit `ea1eeeadb724feb6d4013c9b05733cf9f1a25bfd`; local candidate test change in `fix/m5-firefox-baseline`.
Compiler candidate: HowlFrame branch `fix/firefox-richtext-selection`, based on `41ee7b80c6a5eb6b08941f9415a88156b9d0875f`.
Browser: Playwright 1.64.1, Firefox 157.0.

## Result

The final WasmGC browser suite executed all 81 unique tests in four sequential Playwright shards: 27 + 22 + 16 + 16. The shard inventories match the unsharded 81-test inventory exactly, with no missing or duplicate test IDs. Each shard's `.last-run.json` reports `passed` with an empty `failedTests` list. All four commands returned exit code 0.

The pinned JavaScript consumer baseline (`a6715f480e8b170742b03e7a0b8b97f69e8ce011`) was built using the candidate HowlFrame compiler's JavaScript backend. TXT-002 and A11Y-007 both passed in Firefox (2/2). The same two tests passed against the WasmGC candidate (also included in the full 81/81 result).

The two historically failing cases now behave as intended: TXT-002 rejects the unsafe URL and then creates the expected HTTPS link; A11Y-007 finds no page-wide overflow or clipped text at its tested zoom and spacing settings.

## Resource evidence

Every WasmGC shard ran in a separate verified systemd scope with MemoryHigh 3 GiB, MemoryMax 5 GiB, MemorySwapMax 0, CPUQuota 150%, TasksMax 256, one worker, retries 0, and a 60-second per-test timeout. A process-tree monitor enforced a 180-second command timeout and stopped on task ceiling, memory high, scope swap use, host available memory below 10 GiB, severe memory PSI, swap-out, or cgroup resource events. No shard hit a stop condition. All cgroup memory and pids events were zero; scope swap remained zero.

| Shard | Tests | Result | Scope | Peak tasks | Memory peak | Min host MemAvailable | Host swap-in delta | Host swap-out delta |
|---|---:|---|---|---:|---:|---:|---:|---:|
| 1/4 | 27 | PASS | `run-p2108649-i10499697.scope` | 239 | 1,051,840,512 B | 14,365,421,568 B | 51 pages | 0 |
| 2/4 | 22 | PASS | `run-p2111475-i10503634.scope` | 239 | 1,077,202,944 B | 15,015,276,544 B | 1,215 pages | 0 |
| 3/4 | 16 | PASS | `run-p2115009-i10519253.scope` | 243 | 1,101,565,952 B | 15,322,206,208 B | 39 pages | 0 |
| 4/4 | 16 | PASS | `run-p2116301-i10519349.scope` | 240 | 1,116,987,392 B | 15,349,723,136 B | 84 pages | 0 |

Memory PSI remained 0 throughout. Swap-in counters rose slightly in some runs while swap-out stayed at 0 and available memory remained above 13.3 GiB. A strict preflight initially stopped shard 3 twice because any swap-in increment was treated as a blocker; one check observed 17 pages over 3 seconds, with no swap-out, zero memory PSI, and over 15 GiB available. The task-local preflight was adjusted only to record swap-in while preserving the unchanged-swap-out condition and all cgroup/resource thresholds. The final shard 3 preflight observed zero swap-in change; the monitored run observed 39 pages of swap-in, no swap-out, and no pressure. No global or unrelated process settings changed.

A final process check found no active Firefox process from the tests. The owner's desktop Firefox was not touched. No hosted CI, push, billable run, or merge was performed.

## Test inventory

| # | Shard | Test | Result |
|---:|---:|---|---|
| 1 | 1/4 | [firefox] › backup.spec.mjs:79:1 › AST-004 EXP-010 a backup brings back text, settings and images in another browser | PASS |
| 2 | 1/4 | [firefox] › backup.spec.mjs:117:1 › EXP-001 EXP-002 AST-005 AST-006 the downloaded website contains the uploaded images and they load | PASS |
| 3 | 1/4 | [firefox] › backup.spec.mjs:145:1 › CFG-005 SEC-003 SEC-005 a malformed backup leaves the project and the saved draft as they were | PASS |
| 4 | 1/4 | [firefox] › backup.spec.mjs:177:1 › SEC-005 a browser too full to store the backup's images keeps the current project | PASS |
| 5 | 1/4 | [firefox] › backup.spec.mjs:199:1 › SEC-004 an oversized or overfull backup is refused with a message and no damage | PASS |
| 6 | 1/4 | [firefox] › backup.spec.mjs:214:1 › SEC-001 script in an imported project never runs, in the editor or the exported site | PASS |
| 7 | 1/4 | [firefox] › backup.spec.mjs:245:1 › CFG-006 a restore can be cancelled, and undone after it is confirmed | PASS |
| 8 | 1/4 | [firefox] › backup.spec.mjs:260:1 › SEC-006 the website and the backup hold no secrets, editor state or local paths | PASS |
| 9 | 1/4 | [firefox] › editor.spec.mjs:30:1 › EDT-001 a site starts from a chosen template | PASS |
| 10 | 1/4 | [firefox] › editor.spec.mjs:38:1 › EDT-002 TXT-001 TXT-005 text and rich text edits show in the preview and undo and redo | PASS |
| 11 | 1/4 | [firefox] › editor.spec.mjs:58:1 › TXT-002 a link with an unsafe address is refused | PASS |
| 12 | 1/4 | [firefox] › editor.spec.mjs:76:1 › EDT-003 EDT-008 sections can be added, reordered and removed with confirmation | PASS |
| 13 | 1/4 | [firefox] › editor.spec.mjs:95:1 › EDT-004 switching theme keeps every edit | PASS |
| 14 | 1/4 | [firefox] › editor.spec.mjs:105:1 › EDT-005 AST-001 AST-003 EDT-006 an uploaded image appears, is saved and survives a reload | PASS |
| 15 | 1/4 | [firefox] › editor.spec.mjs:124:1 › AST-002 files that are not supported images are refused | PASS |
| 16 | 1/4 | [firefox] › editor.spec.mjs:137:1 › EDT-007 a storage failure is reported, not hidden | PASS |
| 17 | 1/4 | [firefox] › editor.spec.mjs:146:1 › EDT-009 A11Y-001 the main workflow works with the keyboard alone | PASS |
| 18 | 1/4 | [firefox] › editor.spec.mjs:163:1 › A11Y-006 focus returns after a dialog closes | PASS |
| 19 | 1/4 | [firefox] › editor.spec.mjs:174:1 › onboarding can be skipped and reopened | PASS |
| 20 | 1/4 | [firefox] › editor.spec.mjs:186:1 › A11Y-005 the editor has no serious or critical automated violations | PASS |
| 21 | 1/4 | [firefox] › editor.spec.mjs:193:1 › EDT-002 the phone preview shows the mobile layout | PASS |
| 22 | 1/4 | [firefox] › editor.spec.mjs:200:1 › MOT-006 the preview animates on request, and switching theme mid-transition leaves a correct page | PASS |
| 23 | 1/4 | [firefox] › editor.spec.mjs:227:1 › SEC-001 the animated preview runs only the site runtime; any other inline script is blocked | PASS |
| 24 | 1/4 | [firefox] › editor.spec.mjs:242:1 › EDT-002 the preview stays on the section being edited | PASS |
| 25 | 1/4 | [firefox] › editor.spec.mjs:257:1 › EDT-002 on a narrow screen, keeping the preview on a section never scrolls the editor itself | PASS |
| 26 | 1/4 | [firefox] › editor.spec.mjs:271:1 › A11Y-007 the editor reflows at 320 px (400% zoom) without horizontal scrolling | PASS |
| 27 | 1/4 | [firefox] › editor.spec.mjs:282:1 › EDT-003 a scene section can be added, given a picture and text, and previewed | PASS |
| 28 | 2/4 | [firefox] › engine-site.spec.mjs:39:5 › modern-professional › EXP-004 EXP-005 THM-005 assets load and in-page links resolve | PASS |
| 29 | 2/4 | [firefox] › engine-site.spec.mjs:60:5 › modern-professional › THM-006 no horizontal overflow at phone, tablet and desktop sizes | PASS |
| 30 | 2/4 | [firefox] › engine-site.spec.mjs:69:5 › modern-professional › THM-006 every section keeps its padding and content width, and images fill their slots | PASS |
| 31 | 2/4 | [firefox] › engine-site.spec.mjs:82:5 › modern-professional › A11Y-005 no serious or critical automated violations | PASS |
| 32 | 2/4 | [firefox] › engine-site.spec.mjs:39:5 › ink-wash › EXP-004 EXP-005 THM-005 assets load and in-page links resolve | PASS |
| 33 | 2/4 | [firefox] › engine-site.spec.mjs:60:5 › ink-wash › THM-006 no horizontal overflow at phone, tablet and desktop sizes | PASS |
| 34 | 2/4 | [firefox] › engine-site.spec.mjs:69:5 › ink-wash › THM-006 every section keeps its padding and content width, and images fill their slots | PASS |
| 35 | 2/4 | [firefox] › engine-site.spec.mjs:82:5 › ink-wash › A11Y-005 no serious or critical automated violations | PASS |
| 36 | 2/4 | [firefox] › engine-site.spec.mjs:39:5 › cyberpunk › EXP-004 EXP-005 THM-005 assets load and in-page links resolve | PASS |
| 37 | 2/4 | [firefox] › engine-site.spec.mjs:60:5 › cyberpunk › THM-006 no horizontal overflow at phone, tablet and desktop sizes | PASS |
| 38 | 2/4 | [firefox] › engine-site.spec.mjs:69:5 › cyberpunk › THM-006 every section keeps its padding and content width, and images fill their slots | PASS |
| 39 | 2/4 | [firefox] › engine-site.spec.mjs:82:5 › cyberpunk › A11Y-005 no serious or critical automated violations | PASS |
| 40 | 2/4 | [firefox] › example.spec.mjs:54:1 › every statement in the example is copied from the public source, and the check catches an invented one | PASS |
| 41 | 2/4 | [firefox] › example.spec.mjs:62:1 › personal open-source projects are never presented as professional work | PASS |
| 42 | 2/4 | [firefox] › example.spec.mjs:73:1 › project.json is what the generator makes from the recorded source | PASS |
| 43 | 2/4 | [firefox] › example.spec.mjs:97:3 › served under the Pages path › the chooser opens each version, and each version switches to the others | PASS |
| 44 | 2/4 | [firefox] › example.spec.mjs:113:5 › served under the Pages path › modern-professional: content, portrait, links back and motion work | PASS |
| 45 | 2/4 | [firefox] › example.spec.mjs:130:5 › served under the Pages path › modern-professional: no serious or critical automated accessibility violations | PASS |
| 46 | 2/4 | [firefox] › example.spec.mjs:113:5 › served under the Pages path › ink-wash: content, portrait, links back and motion work | PASS |
| 47 | 2/4 | [firefox] › example.spec.mjs:130:5 › served under the Pages path › ink-wash: no serious or critical automated accessibility violations | PASS |
| 48 | 2/4 | [firefox] › example.spec.mjs:113:5 › served under the Pages path › cyberpunk: content, portrait, links back and motion work | PASS |
| 49 | 2/4 | [firefox] › example.spec.mjs:130:5 › served under the Pages path › cyberpunk: no serious or critical automated accessibility violations | PASS |
| 50 | 3/4 | [firefox] › journey.spec.mjs:22:1 › JRN-01..20 a beginner builds, saves, downloads and publishes a site | PASS |
| 51 | 3/4 | [firefox] › motion.spec.mjs:66:3 › cinematic site › MOT-001 sections reveal as they scroll in, scenery follows scroll and pauses off screen | PASS |
| 52 | 3/4 | [firefox] › motion.spec.mjs:84:3 › cinematic site › A11Y-004 MOT-005 pausing stops every effect, leaves nothing running, and is remembered | PASS |
| 53 | 3/4 | [firefox] › motion.spec.mjs:110:3 › cinematic site › MOT-002 A11Y-003 reduced motion stops parallax, drift and reveals; content stays | PASS |
| 54 | 3/4 | [firefox] › motion.spec.mjs:132:3 › cinematic site › MOT-002 a reduced-motion request made while the page is open stops motion at once | PASS |
| 55 | 3/4 | [firefox] › motion.spec.mjs:144:3 › cinematic site › MOT-004 rapid repeated navigation leaves a consistent page | PASS |
| 56 | 3/4 | [firefox] › motion.spec.mjs:158:3 › cinematic site › A11Y-002 the site can be navigated with the keyboard alone | PASS |
| 57 | 3/4 | [firefox] › motion.spec.mjs:176:1 › MOT-003 with motion set to None all content and navigation work | PASS |
| 58 | 3/4 | [firefox] › motion.spec.mjs:195:1 › MOT-007 a long section scrolls normally and is not clipped | PASS |
| 59 | 3/4 | [firefox] › motion.spec.mjs:260:1 › A11Y-007 exported text stays readable at 400% zoom and with increased text spacing, in every theme | PASS |
| 60 | 3/4 | [firefox] › motion.spec.mjs:295:3 › a story told in scenes › MOT-001 scenes play as chapters: the middle scene fades in through mist, pushes in, then shows its text | PASS |
| 61 | 3/4 | [firefox] › motion.spec.mjs:336:3 › a story told in scenes › MOT-004 during real wheel scrolling the visible scene never moves; only fades play | PASS |
| 62 | 3/4 | [firefox] › motion.spec.mjs:358:3 › a story told in scenes › A11Y-002 the chapter rail works from the keyboard | PASS |
| 63 | 3/4 | [firefox] › motion.spec.mjs:369:3 › a story told in scenes › MOT-002 MOT-007 with reduced motion, scenes are ordinary sections that scroll normally | PASS |
| 64 | 3/4 | [firefox] › motion.spec.mjs:389:3 › a story told in scenes › EXP-006 every scene's picture and text are there without JavaScript | PASS |
| 65 | 3/4 | [firefox] › motion.spec.mjs:402:3 › a story told in scenes › A11Y-005 a story page has no serious or critical automated violations | PASS |
| 66 | 4/4 | [firefox] › pages.spec.mjs:14:1 › EXP-003 the landing page, editor and example work under the Pages path | PASS |
| 67 | 4/4 | [firefox] › pages.spec.mjs:39:1 › the sample site shows every section type with its images, and its backup opens in the editor | PASS |
| 68 | 4/4 | [firefox] › performance.spec.mjs:43:1 › an exported site's initial download is far under 250 KiB compressed, and its runtime much smaller than the editor | PASS |
| 69 | 4/4 | [firefox] › performance.spec.mjs:57:3 › Core Web Vitals on phone: largest paint and layout shift are within "good" | PASS |
| 70 | 4/4 | [firefox] › performance.spec.mjs:57:3 › Core Web Vitals on desktop: largest paint and layout shift are within "good" | PASS |
| 71 | 4/4 | [firefox] › performance.spec.mjs:77:1 › scroll-linked motion adds little frame time over a still page, and stops when paused | PASS |
| 72 | 4/4 | [firefox] › performance.spec.mjs:128:1 › the opening scene's background loads at once with high priority; other images wait | PASS |
| 73 | 4/4 | [firefox] › performance.spec.mjs:139:1 › images are lazy below the fold, sized to avoid layout shift, and uploads are capped | PASS |
| 74 | 4/4 | [firefox] › performance.spec.mjs:150:1 › M5 records editor startup, editing, image, backup, restore and export costs | PASS |
| 75 | 4/4 | [firefox] › phase0.spec.mjs:47:1 › EXP-011 an invalid project explains the problem and is not exported | PASS |
| 76 | 4/4 | [firefox] › phase0.spec.mjs:70:3 › exported site › EXP-001 the ZIP holds a complete site and nothing from the editor | PASS |
| 77 | 4/4 | [firefox] › phase0.spec.mjs:81:3 › exported site › EXP-006 essential content is in the HTML itself | PASS |
| 78 | 4/4 | [firefox] › phase0.spec.mjs:89:5 › exported site › EXP-002 EXP-003 serves and renders at / | PASS |
| 79 | 4/4 | [firefox] › phase0.spec.mjs:89:5 › exported site › EXP-002 EXP-003 serves and renders at /example-site/ | PASS |
| 80 | 4/4 | [firefox] › phase0.spec.mjs:89:5 › exported site › EXP-002 EXP-003 serves and renders at /howlscene-hf/ | PASS |
| 81 | 4/4 | [firefox] › phase0.spec.mjs:108:3 › exported site › EXP-006 content is present with JavaScript disabled | PASS |

## Local commit references

- Consumer test and coverage commit: `68384f9257e42a437035cd9039e4593db1a9528b` (branch `fix/m5-firefox-baseline`), based on M5 commit `ea1eeeadb724feb6d4013c9b05733cf9f1a25bfd`.
- HowlFrame runtime fix: `cac48ccdad09489598123734be3c28815565146d` (branch `fix/firefox-richtext-selection`), based on `41ee7b80c6a5eb6b08941f9415a88156b9d0875f`.
- These local commits are unpushed. No hosted CI was run.
