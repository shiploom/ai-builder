# core/ — portable source (versioned in `VERSION`)

Single source of truth for adapters. Base-spec compatible; harness extras only under `x-shiploom-harness:`.

## Version policy (semver)

- `core/VERSION` is `MAJOR.MINOR.PATCH` (no `-draft` suffixes past 1.0.0).
- MAJOR: breaking contract change (schemas, CLI surface, manifest shape).
- MINOR: additive change (new commands, skills, adapters, gates).
- PATCH: fixes and doc-only changes.
- `pyproject.toml` tracks the same version (PEP 440). Projects pin it via
  `shiploom pin`; `shiploom upgrade` moves a project between versions with
  backup + rollback.

- `artifacts-templates/` — normative Markdown templates with frontmatter (§11.6). (PR3)
- `skills/` — 16 base-spec packs, catalog complete per spec §13
  (PR5 product trio; PR6 research + acceptance; PR8 build + verify;
  brownfield-map + impact-analysis; PR20: deployment-plan,
  post-deploy-validate, docs-generate, security-review, debug-triage,
  change-plan).
- `workflows/` — declarative `greenfield-full-lite`, `brownfield-fix`. (PR7)
- `hooks/` — `registry.json` default-deny entries (shipped; harness event mapping post-MVP).
- `policies/` — `default.json` pack (shipped; team/enterprise packs post-MVP).
- `roles/` — specifier / implementer / verifier packs. (PR5)
