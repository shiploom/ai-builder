# Changelog

All notable changes, newest first. Version is `core/VERSION`
(`1.0.0-draft` throughout the MVP series).

## PR13 — Verification depth (unreleased)

- `sast` gate (project-configured command, skip-if-absent like build/lint).
- `license` gate: lockfile license inventory vs allowlist, report-only.
- `depAudit`: osv-scanner when present + lockfiles exist (fails on vulns),
  else inventory + pin check as before.
- `mutation` gate: deterministic AST candidate sampler, report-only
  (enforceable thresholds wait on pilot escape data per §33-D2).

## PR12 — Adapter completion + conformance runner (unreleased)

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
