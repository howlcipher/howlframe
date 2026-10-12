# Browser capability dogfood

HowlBoard backend tests passed, but the browser emitted no API requests and displayed a connection error. The generated fetch grant helper referenced process.env in a browser.

Before: five browser-like Node VM regressions failed with process undefined, including the explicit network-grant case.
After: host browser grants are explicit strings; missing, invalid and unrelated grants deny before fetch. Node still reads its environment. The bootstrap catcher reports the original error in browser contexts.

Test impact: existing Node fetch and other capability tests retain coverage. New generated-code tests execute with no process/require browser globals, covering allowed fetch and four denials. Full repository tests/build/vet and real HowlBoard browser acceptance are recorded in the mission evidence before publication.
