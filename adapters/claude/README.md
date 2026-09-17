# Adapter: claude

- `CLAUDE.md` from template (`{{projectName}}`, `{{coreVersion}}`, `{{workflow}}`).
- `.claude/skills/<name>/SKILL.md`: byte copies of `core/skills/*`
  (base-spec pure, zero fork) plus a `DO NOT EDIT` HTML comment inserted
  after the frontmatter block.
- `.claude/settings.json`: pinned `PreToolUse` hook wiring
  (`code.claude.com/docs/en/hooks`): `Edit|Write` matcher → generated
  `.claude/hooks/shiploom-guard.py` (stdlib python3, executable), which
  denies builder writes under `.shiploom/.oracle/` and stays silent
  otherwise. JSON carries no `DO NOT EDIT` key (vendor-schema safety);
  provenance lives here + the generation report — regenerate, don't hand-edit.
- `x-shiploom-harness` extras: none in MVP core skills; namespaced extras
  pass through untouched when present.

Unmappable (documented, never silent): deploy/migration/merge lifecycle
events have no clean tool-call mapping — the orchestrator's policy gates
enforce them (`run` exits 3 on deny). `settings.json` permission
allow/ask/deny blocks need a permissions-doc pin (post-MVP).
