# Adapter: opencode

- `.opencode/skills/<name>/SKILL.md`: byte copies of `core/skills/*`
  plus a `DO NOT EDIT` HTML comment after frontmatter.
- Facts come from the root `AGENTS.md` (generate via the `base`
  adapter); OpenCode honors the base-spec subset and ignores unknown
  frontmatter fields, so no fork is needed.
- `opencode.json`: pinned against `opencode.ai/docs/config`
  (`$schema: https://opencode.ai/config.json`): `instructions:
  ["AGENTS.md"]` + `permission: {edit: ask, bash: ask}` (least-privilege
  defaults; OpenCode itself defaults to allow-all). No marker key inside
  (vendor-schema safety) — regenerate, don't hand-edit.
- Per-skill permission granularity does not exist in OpenCode's schema
  (only per-agent `tools`); global defaults above + orchestrator
  deny-means-deny cover MVP. Skill-scoped agents on demand: post-MVP.

Run `shiploom adapters --generate base` alongside `opencode` for the
complete setup.
