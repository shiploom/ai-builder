# AGENTS.md — Shiploom Core repo conventions

> Single source of truth: `MASTER_SPEC.md` + `BUILD_PLAN.md` + `core/VERSION`. Code is truth; artifacts must not diverge from it.

## Facts

- Stack: Python + uv, `requires-python >=3.9`. Validators in `validators/` are **stdlib-only** (no `jsonschema`, no `yaml` at runtime).
- CLI in `cli/` uses stdlib `argparse` only in MVP. Entry point: `shiploom` → `cli.shiploom:main`.
- Schemas in `schemas/` are JSON Schema draft 2020-12. Nine normative files (see `schemas/README.md`).
- Adapters: single-source `core/*` → generated harness files. Never hand-edit generated files (header `DO NOT EDIT`).
- Oracle vault `./.shiploom/.oracle/` is gitignored, `0700`, never fed to builder contexts.

## Conventions

- Every artifact Markdown file carries YAML frontmatter per `schemas/artifact-frontmatter.schema.json` (`id/kind/title/status/provenance/links/owner/version`).
- Traceability `Requirement → Decision → Implementation → Test → Verification Result` via `links{}` + generated `trace.json`.
- Evidence grading mandatory in research artifacts: Fact / Inferred / Opinion / Marketing / Conflicting / Uncertain + source URL + date + confidence.
- Exit codes: 0 pass, 2 validation fail, 3 policy deny, 4 budget exceeded, 5 harness mismatch.
- Secrets: never in configs/artifacts/logs. Env/keychain refs only (`env:NAME`). `scan-secrets` gate blocks commits with patterns.

## Commands

- `shiploom validate [--strict] [path]` — schemas + frontmatter + links
- `shiploom status [--json]` — manifest + artifact states + budgets
- `shiploom doctor` — harness + MCP + toolchain compat (offline)
- `shiploom lock [--check]` — hash-lock acceptance + oracles
- `shiploom run <workflow> [--from/--only/--resume] [--budget K=V]` — idempotent stepper
- `shiploom approve <gate-id> [--deny --reason]` — record human gates
- `shiploom verify [--report] [--gates a,b]` — deterministic gates + quality table
- `shiploom adapters --list | --generate <harness|all>` — harness file generation
- `shiploom trace <id> [path]` — trace subgraph + referenced-by
- `shiploom budget [--set K=V]` — manifest budget limits
- `shiploom resume` — position report + advance bound workflow
- `shiploom approvals [--watch]` — pending human gates
- `shiploom add <kind> <name> --from SRC [--tag]` — overlay content packs
- `shiploom conformance --harness <name|all> [--record]` — deterministic harness checks
- `shiploom pin [--check]` — reproducibility pin + drift check
- `shiploom upgrade [--dry-run] [--rollback]` — move project core version
- `shiploom characterize --capture NAME [--command CMD] | --diff NAME | --list` — behavior snapshots
- `pytest tests/unit -q` — hermetic unit tests (no network)

## Nested scopes

- `core/`: portable source — base-spec compatible, no harness-specific fields outside `x-shiploom-harness:`.
- `adapters/`: harness deltas live here only, never by forking skills.
- `./.shiploom/`: orchestrator-owned (manifest/audit). Tools never write artifacts except via orchestrator-validated transitions.
