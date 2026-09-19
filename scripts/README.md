# scripts/ — repo tooling (render, lint-spelling, check-links)

- `ac-demo.sh` — acceptance demo (AC1–AC6, BUILD_PLAN PR10), Go-only.
  Runs the real CLI in scratch dirs: `sh scripts/ac-demo.sh`
  (needs `jq`; `SHIPLOOM_GO_BIN` overrides the binary, else it is built;
  node leg skips if unavailable).
- `collect-dashboard.sh` — cost/escape dashboard collector (contract:
  `docs/cost-dashboard.md`): `sh scripts/collect-dashboard.sh [dir] [--json]`.
