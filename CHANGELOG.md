# Changelog

All notable changes, newest first. Version is `core/VERSION`
(semver since 1.0.0; pre-release series was `1.0.0-draft`).

## PR23 — Go port P1: scaffold + parity harness (unreleased)

- `go.mod` (`github.com/shiploom/ai-builder`, stdlib-only), `cmd/shiploom`,
  `internal/cli` (20-command table, exit-code contract, argparse-mirroring
  errors), `internal/version` (ldflags-stamped core version).
- `tests/parity/run.sh` golden harness (normalized stdout/exit comparison,
  needle matching) with version/no-args/bad-command cases; `scripts/build-go.sh`
  stamped builder; CI `go-port` job (gofmt/vet/test/parity).

## 1.1.0 (2026-09-17)

Additive minor: CLI completeness (PR11), adapter configs + conformance runner
(PR12), verification depth (PR13), release train + docs (PR14), docs site +
Pages hosting (PR15, PR19), MCP attestation groundwork (PR16),
characterization snapshots (PR17), team pack template (PR18), skill catalog
completion (PR20), deploy matrix + DAST (PR21), and the 3.9 CI fallback fix.
Details per PR below; 251 tests green on 3.9+3.13, ac-demo 54/54.

## PR20 — Skill catalog completion (1.1.0)

- Six remaining spec §13 packs: `deployment-plan`, `post-deploy-validate`,
  `docs-generate`, `security-review`, `debug-triage`, `change-plan` —
  base-spec pure, contract-tested. Catalog now 16/16.

## PR21 — Deploy matrix + DAST gate (1.1.0)

- `examples/deploy/{docker,k8s,aws,preview}/` starters (non-root images,
  probed K8s manifests, default-deny netpol, Terraform stub with spend-gate
  workflow, preview presets + smoke checklist).
- `dast` gate: project-configured command, skip-if-absent (a runner cannot
  assume a live target — same rationale as SAST).

## PR19 — GitHub Pages hosting (1.1.0)

- `PAGES_BASE_PATH`/`NEXT_PUBLIC_BASE_PATH`-aware Next config + fumadocs
  `baseUrl`; landing links via `next/link` so prefixes apply on deploy.
- `.github/workflows/docs-site.yml`: drift gate → prefixed static build →
  Pages deploy (needs one manual step: Pages source = GitHub Actions).

## PR18 — Team policy pack template (1.1.0)

- `examples/team-policy-pack/`: stricter overlay demo (named all-env migration
  deny, actor-conditional deploys, owned allow-list) installable via
  `shiploom add policy team`, wired per-step with `policy:`.
- Cookbook section documents the three team moves; enterprise SSO/RBAC/audit
  export stay demand-gated.

## PR17 — Characterization capture/diff (1.1.0)

- `shiploom characterize --capture NAME [--command CMD] | --diff NAME | --list`:
  behavior snapshots under `./.shiploom/characterization/` (command, exit,
  output hash, capped tail). Changed snapshots warn (review at
  merge-approval), tool errors fail; names restricted against traversal.
- `regression-verify`/`verify` diff snapshots automatically; `run` surfaces
  checker warnings in human and JSON reports.

## PR16 — MCP attestation groundwork (1.1.0)

- Optional `servers[].attestation` in the registry schema (`sigstore` /
  `pinned-digest` / `tofu` + digest/signer/timestamp; backward compatible).
- Honest statuses: `attested:true` without evidence reports as **unverified**,
  never trusted. `resolve()` exposes per-provider status; `doctor` warns on
  unprovenanced servers without failing.
- `docs/mcp-attestation.md` design note (threat model, method bars, deferred
  automated verification + rug-pull re-approval).

## PR15 — Docs site scaffold (1.1.0)

- `site/` (Next.js + Fumadocs, static export): landing page + `/docs`
  generated from `docs/` via `scripts/sync-docs.mjs` (frontmatter injection,
  JSX-safe escaping, `--self-test`, CI drift gate `--check`).
- Pinned against installed APIs (provider/next, layouts/docs, mdx config);
  system font stack (no build-time font downloads).

## PR14 — Releases, distribution, docs (1.1.0)

- 1.0.0: `core/VERSION` + `pyproject.toml` bumped (semver policy in
  `core/README.md`); MVP exit declared.
- `shiploom --version`, `pin [--check]` (lock.json + drift), `upgrade
  [--dry-run] [--rollback]` (backup, compat check, auto-rollback on
  failed validation).
- Tag-triggered release workflow (version gate, full validation, sdist +
  wheel + CycloneDX SBOM, GitHub release); manual cosign step documented.
- Brew formula template (filled at first release); install + release docs;
  consumer manual (J1–J4), policy cookbook, brownfield playbook,
  cost-dashboard contract + collector stub.
- `scripts/playground.sh` (docker-gated AC demo on bare images).

## PR13 — Verification depth (2026-09-17)

- `sast` gate (project-configured command, skip-if-absent like build/lint).
- `license` gate: lockfile license inventory vs allowlist, report-only.
- `depAudit`: osv-scanner when present + lockfiles exist (fails on vulns),
  else inventory + pin check as before.
- `mutation` gate: deterministic AST candidate sampler, report-only
  (enforceable thresholds wait on pilot escape data per §33-D2).

## PR12 — Adapter completion + conformance runner (1.1.0)

- `.claude/settings.json`: pinned `PreToolUse` wiring (vendor docs
  `code.claude.com/docs/en/hooks`) to a generated stdlib guard hook that
  denies oracle-vault writes and stays silent otherwise (executable bit set).
- `opencode.json`: pinned against `opencode.ai/docs/config` (`$schema`,
  `instructions: ["AGENTS.md"]`, `permission: {edit: ask, bash: ask}`).
  No marker keys inside generated JSON (vendor-schema safety); provenance in
  adapter READMEs + generation reports.
- `shiploom conformance --harness <base|claude|opencode|all> [--record]`:
  deterministic profile checks incl. live guard behavior; version-drift
  detection (adapter v1.1.0 vs profiles); records gitignored.
- Per-skill OpenCode granularity not in vendor schema (per-agent `tools`
  only): global defaults ship, skill-scoped agents stay post-MVP.
- CI fix: shared `tests/unit/_stdlib.py` fallback (a drifted per-test 3.9
  fallback failed the 3.9 leg; single canonical set now).

## PR11 — CLI completeness (`281fa17`, 2026-09-17)

- New commands: `trace <id>` (subgraph + referenced-by), `budget [--set K=V]`
  (manifest limits; token/spend operator-reported), `resume` (position report +
  advance bound workflow), `approvals [--watch]` (pending gates; rationale cards
  need a manifest schema follow-up), `add <kind> <name> --from SRC [--tag]`
  (overlay install, strict-validated with rollback, unsigned provenance).
- 205 unit tests green (15 new in `tests/unit/test_commands.py`).

## MVP — Portable verification layer (`a7ae419`, 2026-09-17)

PR1–PR10 + doc-sweep, 190 tests green. Specifier/Implementer/Verifier +
deterministic orchestrator + human gates; zero-runtime stdlib-only Python.

- **Contracts:** 9 JSON Schemas (draft 2020-12); stdlib-only validators
  (`validate`, `trace`, `status`); 21 artifact templates; greenfield-starter
  fixture with full IDEA→REQ→ADR→ACC→VR trace chain.
- **CLI:** `install init validate status doctor audit lock run approve verify
  adapters` (exit codes 0/2/3/4/5).
- **Trust core:** hash-locked acceptance + 0700 oracle vault, hash-chained audit
  log, atomic manifest, default-deny JSON policy (policy gates deny with
  exit 3), MCP capability registry, stdlib secret scanner, dep inventory,
  deterministic gates + 2× flake check + quality table.
- **Portability:** base/Claude/OpenCode adapters (idempotent, `DO NOT EDIT`
  headers); 8-skill catalog + brownfield-map + impact-analysis; 3 role packs;
  greenfield-full-lite + brownfield-fix workflows with resume/budgets/retries.
- **Proof:** `scripts/ac-demo.sh` 52/52 (AC1 on 2 stacks × 2 harnesses,
  AC2–AC6 incl. kill -9 resume and policy-deny negatives).
- **Deliberately deferred:** `add/upgrade/trace/budget/resume/approvals`
  (→PR11, done), `settings.json`/`opencode.json` mapping, SAST/license gates,
  vuln-DB audit, Go binary, docs site — see `POST_MVP_PLAN.md`.
