# Post-MVP Plan (PR11–PR14 + backlog)

**Date:** 2026-09-17 · **Status:** PR11–PR15 complete; PR16 (MCP attestation groundwork) in progress
(Hosting still open — `npm run build` produces a static `out/` directory.)
**Decisions locked:** CLI completeness leads; stay on Python+uv (Go revisit only
at 10k-user scale per spec §2.4); no third harness; no enterprise pilot waiting
(demand-gated items stay unscheduled).

**Principles carried over:** stdlib-only runtime, offline-first, hermetic unit
tests, every behavior paired with negative tests, docs updated in-slice,
strict-validate green throughout.

## PR11 — CLI completeness (~1 slice)

Fill the spec'd surface with thin wrappers over existing modules:

1. **`trace REQ-001`** — wrapper over `validators/trace.py`: subgraph
   (requires/decided_by/implemented_by/tested_by/verified_by), human-readable
   default + `--json`. Unknown ID → exit 2.
2. **`budget [--set K=V]`** — read manifest budget limits/usage; `--set` updates
   limits (same `KEY=VALUE` grammar as `run --budget`, shared parser in
   `cli/run.py`). Token/spend stay operator-reported (metering needs harness
   integration — stated, not faked); wall-clock already enforced.
3. **`resume`** — first-class command (today resume is `run`'s implicit default):
   verifies checkpoints + manifest integrity, reports position
   (done/total, next step, pending gates), then advances like `run`. Gives AC4 a
   named entry point.
4. **`approvals --watch`** — poll loop listing pending gates with workflow
   context. Honest scope: full plain-language cards (rationale + rejected
   alternatives) need metadata the manifest doesn't store yet — MVP-lite lists
   pending gates with step/attempt/budget context; card enrichment is a schema
   follow-up, flagged in help text.
5. **`add <skill|workflow|hook|policy|adapter> <name>`** — install from local
   path or git URL + semver tag + JSON-schema validation before copy
   (per §33-D4 minus cosign: provenance recorded as `unsigned` with warning;
   threshold-sigs/transparency-log stay Advanced).

Exit codes follow 0/2/3/4/5. Tests in established `test_cli.py` style.

## PR12 — Adapter completion + conformance runner (~2 slices)

1. Claude `settings.json` (registry events → 28 hook events; unmappable keeps
   orchestrator-pre-check + documented-note pattern) + deny-by-default routing.
2. `opencode.json` — research spike first (pin against vendor docs), then minimal
   generation + per-skill permissions.
3. `shiploom conformance --harness [--record]` — deterministic runner over
   `tests/conformance/*/capabilities.json` + `examples/greenfield-starter`.
   Nightly live-LLM lane + badges need CI secrets/model budgets (infra-dependent).
4. No third harness.

## PR13 — Verification depth (~2 slices)

1. SAST + license gates in `cli/gates.py` (configured SAST, skip-if-absent;
   license from metadata vs allowlist, report-only first).
2. Vuln-DB dep audit (`osv-scanner` when present + lockfiles; offline fallback
   stays; needs a network-lane CI decision).
3. Mutation sampling, report-only (enforceable thresholds only after pilot escape
   data per §33-D2).
4. Browser/E2E oracle stays Advanced (needs Playwright harness + preview infra).

## PR14 — Releases, distribution, docs (~2 slices)

1. Semver policy for `core/VERSION` + `upgrade --dry-run/--rollback` + `pin`.
2. `pipx`/`uvx` primary; add `brew tap`. No Go binary, no `curl|sh` yet.
3. cosign/Sigstore + SBOM at publish time (sequenced after versioning).
4. Fumadocs site + landing + playground (docker-limited); consumer manual
   (J1–J4, policy cookbook, brownfield playbook) written here.
5. Cost/escape dashboard: schema + stub (thresholds need pilot data).

## Backlog (demand-gated)

- Team/enterprise packs, Cedar/Rego, SSO/RBAC, deploy matrix — pilot pull only.
- MCP attestation + rug-pull re-approval — needs attestation-protocol research.
- Advanced tier: invariant mining, cost optimizer, fleet sequencing, thin IDE
  approver, marketplace signing infra, i18n, drift bots.
- Deliberately never: IDE build, model routing, cloud sandbox, vector memory,
  server/DB/queue in core.

## Verification per PR

Hermetic unit tests, `validate --strict .` clean, secrets gate clean,
`ac-demo.sh` extended where the PR touches its path, AGENTS.md + module READMEs
updated in-slice.
