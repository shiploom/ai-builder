# tests/

- Go unit tests (`go test ./cmd/... ./internal/...`) — hermetic, no
  network: schemas/validators/policy/manifest/adapters (table-driven).
  (The Python `unit/` suite and `parity/` harness left with the
  implementation in v1.3.0.)
- `integration/` — `run` on fixtures via recorded harness transcripts + real toolchain gates.
- `conformance/` — per-harness `capabilities.json` + shared fixtures + expectations.
- `seeds/` — fault-injection: broken build, SQLi, secret, regression, flaky, hallucinated API, missing acceptance.
