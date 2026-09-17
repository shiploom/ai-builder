# Adapter: opencode

- `.opencode/skills/<name>/SKILL.md`: byte copies of `core/skills/*`
  plus a `DO NOT EDIT` HTML comment after frontmatter.
- Facts come from the root `AGENTS.md` (generate via the `base`
  adapter); OpenCode honors the base-spec subset and ignores unknown
  frontmatter fields, so no fork is needed.
- `opencode.json` generation is deferred until pinned against the
  OpenCode config docs (a wrong config is worse than none); per-skill
  permissions and plugin events post-MVP (PR10 closed).

Run `shiploom adapters --generate base` alongside `opencode` for the
complete setup.
