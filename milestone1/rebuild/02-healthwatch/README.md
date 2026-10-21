# Fresh healthwatch candidate

This implementation was written from the frozen [requirements](../../02-healthwatch/requirements/REQUIREMENTS.md) without reading the previous healthwatch source. It executes entirely as HowlFrame bytecode, with no external runtime helper.

From the repository root:

```bash
go build -o /tmp/healthwatch-hf howlframe.go
/tmp/healthwatch-hf build milestone1/rebuild/02-healthwatch/src/healthwatch.howl -o /tmp/healthwatch-fresh.hfbc
HF_BIN=/tmp/healthwatch-hf APP_ARTIFACT=/tmp/healthwatch-fresh.hfbc python3 milestone1/02-healthwatch/tests/acceptance/run_acceptance.py milestone1/02-healthwatch/tests/acceptance/launcher.example.sh -v
```

The acceptance suite is the original frozen suite. This candidate does not change its inputs or assertions. See [JOURNAL.md](JOURNAL.md) for provenance and verification. Acceptance remains subject to independent review; passing tests alone does not establish every possible behavior.

Additional checks: `python3 milestone1/rebuild/02-healthwatch/tests/supplemental.py` (uses the same `HF_BIN` and `APP_ARTIFACT`).
