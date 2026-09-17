# Shiploom Core

Portable, zero-runtime AI software engineering layer: `Specifier → Implementer → Verifier` + deterministic orchestrator + human gates.

- Spec: `MASTER_SPEC.md`
- Build plan: `BUILD_PLAN.md`
- Core version: `core/VERSION`
- Schemas: `schemas/`
- Validators: `validators/` (Python stdlib only — no third-party runtime deps)
- CLI: `cli/` (`pipx install .` / `uvx shiploom`, entry `shiploom`)
- Adapters: `adapters/{base,claude,opencode}/`
- Examples: `examples/`
- Tests: `tests/`
- Docs: `docs/` (sources of truth) + `site/` (Next.js + Fumadocs, `cd site && npm install && npm run dev`)

## Quickstart (MVP, Python+uv)

```bash
uv venv && uv pip install -e ".[dev]"
shiploom doctor
shiploom init --green --harness auto
shiploom validate --strict .
shiploom run greenfield-full-lite --resume
```

See `BUILD_PLAN.md` §9 for the 10-PR execution order. Start at Phase 0 PR1.
