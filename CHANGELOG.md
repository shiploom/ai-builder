# Changelog

All notable changes, newest first. Version is `core/VERSION`
(semver since 1.0.0; pre-release series was `1.0.0-draft`).

## Unreleased — Windows safety + CI hygiene

- `windows` validator leg (gofmt/vet/test, smoke, AC demo, conformance
  all green on `windows-latest`): `os.Rename`-over-existing fixed in
  `manifest.Save`, LF enforced for `.go`/`.sh` via `.gitattributes`,
  native-path handling in `ac-demo.sh`.
- Dependabot for GitHub Actions; `NPM_TRUSTED_PUBLISHING` variable flow
  proven by the v1.3.0 OIDC publish.
- Release data bundle extended with `adapters/` + `examples/` (ships
  next tag); doctor reporting reworded (operator python3, compiled-in
  validators).

## 1.3.0 (2026-09-19)

Major: Python implementation removed — Go is the sole implementation.
Deleted `cli/`, `validators/`, `tests/unit/`, `tests/parity/`, and
`pyproject.toml`; `ac-demo.sh` is Go-only (jq-based config patching,
shell test gates, 54/54); CI runs no Python implementation steps
(`pytest` kept only as ambient operator tooling for auto-detected test
gates, like node); `version-check.sh` asserts tag/core/npx/GoCLI;
`install` copies `{core, schemas}`; dashboard collector ported to
`scripts/collect-dashboard.sh`. Dual-ship gates stay green throughout.

## 1.2.0 (2026-09-19)

Minor: Python fallback deprecated (removal v1.3.0). The dormant
deprecation notice is now live for interactive TTY runs; docs mark the
fallback deprecated. Dual-ship continues: full parity 77/77, version
gate green on both CLIs.

## 1.1.1 (2026-09-18)

Patch: ship the Go release binaries the npx installer downloads.
`@shiploom/cli@1.1.0` was published against the v1.1.0 GitHub release,
which predates the Go build matrix and carries no `shiploom-*` binaries,
so its postinstall download 404s. v1.1.1 re-tags with the full matrix.

## PR42 — npx tool-data bundle (unreleased)

- Release attaches `shiploom-data-<version>.tar.gz`; the npx
  postinstall downloads + extracts it into `vendor/` (system `tar`
  required) so the installed binary resolves schemas, workflows,
  policies, hooks, skills, and roles exe-relative. Verified from an
  alien directory with no env vars (`validate`/`status`/`run` green).

## PR41 — CI warning cleanup + release fixes (unreleased)

- `release.yml` Checksums `cd ..` bug (subshell keeps workspace CWD);
  asset globs exclude the `go-bin/` directory; `actions: read` for
  artifact download; fail-fast binary guard.
- All workflows: actions on current majors, runners pinned to
  `ubuntu-24.04`, `cache: false` on setup-go (stdlib-only, no `go.sum`).
  Zero annotations on validator/docs/release runs.

## PR40 — npm trusted publishing (unreleased)

- `npm-publish` job in `release.yml`: OIDC trusted publishing
  (`--provenance --access public`), gated on the
  `NPM_TRUSTED_PUBLISHING` repo variable until the package settings on
  npmjs.com register this repo + workflow. Works around npm retiring
  new TOTP 2FA while the CLI still demands an OTP (`EOTP`).

## PR37 — P6 dormant Python deprecation notice (unreleased)

- `_maybe_deprecation_notice()` in `cli/shiploom.py`: stderr-only,
  version-gated (`core/VERSION >= 1.2.0`) and TTY-gated
  (`sys.stderr.isatty()`), removal v1.3.0. Silent under piped parity
  harness (77/77 unaffected); force-fire unit tests.

## PR36 — P6 version-parity gate (unreleased)

- `scripts/version-check.sh`: tag == `core/VERSION` == `pyproject.toml`
  == `wrappers/npx/package.json` == the `core X` field of both
  `--version` outputs, fail-closed; wired into `release.yml` replacing
  the inline tag check; static half unit-tested.

## PR35 — P5 distribution (unreleased)

- `go-verify` + 5-platform `go-build` matrix in `release.yml`
  (checksums + Go module manifest attached); Go source-build brew
  formula; `curl|sh` installer (`scripts/install.sh`); thin npx
  launcher (`wrappers/npx`, `@shiploom/cli`); `docs/install.md`
  rewrite (Go binary primary, Python fallback).

## PR34 — AC demo under Go (unreleased)

- `SHIPLOOM_GO_BIN` mode in `scripts/ac-demo.sh` (every CLI step via the
  Go binary; `trace REQ-001` covers the trace-offline step); 54/54 pass
  with the dev interpreter first on PATH.

## PR33 — Go port P4 slice 8: install + init (unreleased)

- `install` (version gate, strict pre-copy validation, offline copy,
  receipt) and `init` (scaffold, 0700 vault, genesis, seeds) ported;
  CWD symlink resolution matches `Path.cwd()`; harness scratch dirs
  canonicalized (`pwd -P`). All 20 commands wired; parity 77/77.

## PR32 — Go port P4 slice 7: conformance (unreleased)

- `internal/conformance` (scratch-generate, profile checks, guard replay,
  `run_all` + record) + `conformance` command; reports carry no timings
  so human and JSON outputs are parity-safe.

## PR31 — Go port P4 slice 6: adapters + add (unreleased)

- `internal/adapters` (templates, skill headers, write journal) and
  `internal/add` (overlay packs, strict validation + rollback,
  unsigned provenance) + commands. `jsoncanon.MarshalLine` fixes the
  audit-log line format to match `json.dump` defaults (hashes unchanged).

## PR30 — Go port P4 slice 5: pin + upgrade (unreleased)

- `pin` (reproducibility pin, drift check) and `upgrade` (dry-run,
  backup + rollback, validation gate) ported; `[Errno 2]` synthesis
  extended to audit/doctor/upgrade/pin reads.

## PR29 — Go port P4 slice 4: doctor (unreleased)

- `internal/doctor` (toolchain, harness, project checks, exit 5) +
  `doctor` command with human/JSON parity.

## PR28 — Go port P4 slice 3: audit + characterize (unreleased)

- `audit` (`--export json|md`) and `characterize`
  (`--capture`/`--diff`/`--list`) ported; snapshot-name quoting and
  missing-file errors fixed to `PyRepr`/`[Errno 2]` parity.

## PR27 — Go port P4 slice 2: trace + approve + budget + resume (unreleased)

- `trace` (sorted relations/incoming), `approve` (gates, deny/reason),
  `budget` (`--set`, setdefault semantics), `resume` (position +
  stepper) ported with byte parity.

## PR26 — Go port P4 slice 1: lock + verify (unreleased)

- `lock` (vault 0700, hash compare) and `verify` (`--report`/`--gates`,
  quality table) ported; `gates.toReportMap` fixed to
  jsoncanon-compatible types. Parity 41/41.

## PR25 — Go port P3: orchestrator + read surfaces (unreleased)

- `internal/workflow` (overlay discovery, `uses` resolution, globs),
  `internal/run` stepper (gates, retries/replan, checkpoints, exits
  0/2/3/4), `internal/trace`/`status`/`approvals` libraries +
  `run`/`status`/`approvals` commands with stateful parity fixtures.
  Parity 32/32.

## PR24 — Go port P2: validate + leaf libraries (unreleased)

- `internal/jsoncanon` (byte-exact Python JSON shapes), `internal/validate`
  (schema subset, frontmatter parser, dispatch, wired `validate` command),
  `internal/manifest`, `internal/auditlog`, `internal/mcp`, `internal/policy`.
- Cross-implementation audit chains verified both directions (incl. mutual
  tamper detection); parity cases extended (strict/relaxed/warn/fail) with
  hermetic tmp-fixture support in the harness.

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
