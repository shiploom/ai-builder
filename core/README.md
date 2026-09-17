# core/ — portable source (versioned in `VERSION`)

Single source of truth for adapters. Base-spec compatible; harness extras only under `x-shiploom-harness:`.

- `artifacts-templates/` — normative Markdown templates with frontmatter (§11.6). (PR3)
- `skills/` — 10 base-spec `SKILL.md` packs, catalog complete (shipped list below).
- `workflows/` — declarative `greenfield-full-lite`, `brownfield-fix`. (PR7)
- `hooks/` — `registry.json` default-deny entries (shipped; harness event mapping post-MVP).
- `policies/` — `default.json` pack (shipped; team/enterprise packs post-MVP).
- `roles/` — specifier / implementer / verifier packs. (PR5)
- `skills/` — shipped: idea-shaping, product-definition,
  architecture-design (PR5); market-research, competitor-teardown,
  spec-to-acceptance (PR6); implement-scoped-diff, verify-independent
  (PR8); brownfield-map, impact-analysis (PR10). Catalog complete.
