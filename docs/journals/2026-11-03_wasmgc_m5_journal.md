
## Final focused rerun after host cleanup

- The rich-text editor marker was changed from an internal DOM attribute to a private `WeakSet`; no application-visible DOM marker is emitted. Re-ran the full richtext unit suite (`^TestRichtext`): PASS, exit 0, 2.918s; verified 3 GiB/5 GiB/no-swap/150%/128-task scope, peak tasks 28, memory peak 143,814,656 bytes, zero pids/memory/swap events. Log `/tmp/hf-richtext-unit-test.log`.
- Rebuilt HowlFrame compiler after that final change: PASS, exit 0; verified 3 GiB/5 GiB/no-swap/150%/128-task scope, peak tasks 11, memory peak 131,280,896 bytes, zero pids/memory events and no swap. Log `/tmp/hf-richtext-build-final2.log`.
- Rebuilt site and editor WasmGC outputs using the rebuilt compiler: PASS, exit 0; verified same limits, peak tasks 7, memory peak 24,899,584 bytes, zero pids/memory events and no swap. Log `/tmp/hs-build-richtext-editor-final2.log`; output confirmed the private `WeakSet` tracking code.
- Final combined focused Chromium invocation against that exact output: TXT-002 and A11Y-007 both passed, 2/2 with one worker, Playwright exit 0 and scope exit 0. Verified 3 GiB/5 GiB/no-swap/150%/256-task scope and child membership. Peak tasks 83, memory peak 522,608,640 bytes, zero swap, pids events, or memory events. Log `/tmp/hs-final-focused-chromium.log`; exit marker `/tmp/hs-final-focused-chromium.exit`.
- Firefox remains unverified for the final change. No desktop Firefox interaction, hosted CI, push, billing, commit, or merge occurred. M5 remains in progress.
