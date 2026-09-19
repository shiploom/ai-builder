# Shiploom Core

Portable, zero-runtime AI software engineering layer: `Specifier → Implementer → Verifier` + deterministic orchestrator + human gates.

- Spec: `MASTER_SPEC.md`
- Build plan: `BUILD_PLAN.md`
- Core version: `core/VERSION`
- Schemas: `schemas/`
- Implementation: single Go binary, stdlib-only (`cmd/shiploom`, `internal/`)
- CLI: `shiploom` (Go binary, `npx -y @shiploom/cli`, or `sh scripts/build-go.sh ./shiploom`)
- Adapters: `adapters/{base,claude,opencode}/`
- Examples: `examples/`
- Tests: `tests/`
- Docs: `docs/` (sources of truth) + `site/` (Next.js + Fumadocs, `cd site && npm install && npm run dev`)

## Quickstart (Go single binary)

```bash
sh scripts/build-go.sh ./shiploom
./shiploom doctor
shiploom init --green --harness auto
shiploom validate --strict .
shiploom run greenfield-full-lite --resume
```

See `BUILD_PLAN.md` §9 for the 10-PR execution order. Start at Phase 0 PR1.
