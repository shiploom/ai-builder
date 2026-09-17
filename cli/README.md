# cli/ — `shiploom` command (Python+uv MVP)

Stdlib `argparse` only in MVP. Entry: `shiploom` → `cli.shiploom:main`.

Implemented (PR4): `install init validate status doctor audit`.
Added (PR6): `lock [--check] [--actor] [--json]` (hash-lock acceptance + oracles).
Added (PR7): `run <workflow> [--from STEP] [--only STEP] [--resume]
[--budget KEY=VALUE] [--json]` (idempotent stepper) + `approve <gate-id>
[--deny --reason]` (records human gates; policy enforced by `run` gates).
Added (PR8): `verify [--report] [--gates a,b] [--json]` (deterministic
gates + quality table; `--report` writes `verification/gate-report.json`).
Post-MVP (PR10 closed): `add upgrade trace budget resume approvals`
+ team/enterprise packs + `settings.json`/`opencode.json` mapping + Fumadocs site.
Added (PR9): `adapters --list | --generate <harness|all>` (base/claude/
opencode; idempotent, `DO NOT EDIT` headers).
Added (PR11): `trace <id> [path] [--json]` (trace subgraph + referenced-by),
`budget [--set K=V] [--json]` (manifest limits; token/spend operator-reported),
`resume [--budget K=V] [--json]` (position report + advance), `approvals
[--watch] [--interval S] [--json]` (pending gates; rationale cards need a
manifest schema follow-up), `add <kind> <name> --from PATH-OR-URL [--tag]
[--force]` (overlay install, strict-validated, unsigned provenance).
Added (PR12): `conformance --harness <base|claude|opencode|all> [--record] [--json]`
(deterministic profile checks incl. live guard behavior; records gitignored).

- `manifest.py` — orchestrator-owned manifest (atomic saves, kill-safe).
- `auditlog.py` — append-only hash-chained audit log + verify/export.
- `doctor.py` — offline compat checks (exit 0 ok, 5 on failure).
- `oracle.py` — acceptance discovery, vault checks, lock/check-lock.
- `run.py` — deterministic stepper: order, consumes/produces, gates,
  retries/replan, resume, wall-clock budgets, checkpoints.
- `gates.py` — gate runner: configured build/typecheck/lint/sast/test/contract,
  auto-detected pytest, built-in secrets scan + dep inventory (osv-scanner when
  present, else report-only) + lockfile license inventory (report-only) + AST
  mutation sampler (report-only) + compileall, 2× determinism check,
  verify-step checker.
- `adapters.py` — single-source core → harness files (`DO NOT EDIT` headers).
- `conformance.py` — deterministic harness checks (generate + profile asserts).
- `policy.py` — JSON-policy packs + hook matching (deny > require-approval
  > allow; conditional rules win ties). Policy gates deny with exit 3.
- `mcp.py` — capability → provider-chain resolution + quarantine flags.
- `shiploom.py` — parser + subcommands. Exit codes: 0/2/3/4/5 per AGENTS.md.
