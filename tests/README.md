# tests/

- `unit/` — hermetic, no network: schemas/validators/policy/manifest/adapters (table-driven).
- `integration/` — `run` on fixtures via recorded harness transcripts + real toolchain gates.
- `conformance/` — per-harness `capabilities.json` + shared fixtures + expectations.
- `seeds/` — fault-injection: broken build, SQLi, secret, regression, flaky, hallucinated API, missing acceptance.
