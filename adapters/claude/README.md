# Adapter: claude

- `CLAUDE.md` from template (`{{projectName}}`, `{{coreVersion}}`, `{{workflow}}`).
- `.claude/skills/<name>/SKILL.md`: byte copies of `core/skills/*`
  (base-spec pure, zero fork) plus a `DO NOT EDIT` HTML comment inserted
  after the frontmatter block.
- `x-shiploom-harness` extras: none in MVP core skills; namespaced extras
  pass through untouched when present.

Unmappable in MVP (documented, never silent): hook-event mapping and
`settings.json` permissions deferred post-MVP (PR10 closed); until then
`doctor` + orchestrator pre-checks enforce gates. Progressive
enhancement (`model/effort/allowed-tools/...`) stays adapter-isolated.
