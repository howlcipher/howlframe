# Relative store atomic replacement

Board make test with current merged Frame425b1ae fails seed500. A public isolated
server repro reports STORE_PERSISTENCE_WRITE_FAILED: rename /tmp/... to relative
howlboard_missions.json returns EXDEV on the mission filesystem. filepath.Split
returns an empty directory for a basename; os.CreateTemp("") selects system temp.

Use "." for that empty-directory case, preserving existing synchronized sibling
write/rename and cleanup. Copying after EXDEV would sacrifice atomicity; forcing
applications to use absolute paths would leave the public relative-store interface
broken. No capability or store-domain change is needed.

Regression replaces an existing bare relative store twice while all system-temp
variables point to a nonexistent directory. It fails before correction and passes
after. Existing store suite, vet/build and actual Board seed/lifecycle retest are
required, plus full supported CI. No persistent service or deployed store is touched.
