# adapters/ — single-source `core/*` → harness files

MVP (PR9): `base` (root `AGENTS.md`), `claude` (`CLAUDE.md` +
`.claude/skills/`), `opencode` (`.opencode/skills/`, facts via base
`AGENTS.md`). PR12 adds pinned `.claude/settings.json` + guard hook and
`opencode.json` (instructions + permission defaults). Skills are base-spec
byte copies plus a `DO NOT EDIT` HTML comment after frontmatter
(frontmatter itself is untouched so base validators keep passing).

Still post-MVP: `settings.json` permission blocks (needs permissions-doc
pin), per-skill OpenCode granularity, Kiro/Copilot/Cursor adapters.
Per-harness READMEs document the gap — never silent.

`shiploom adapters --generate <harness|all>` is idempotent
(created/updated/unchanged report). Never hand-edit generated files.
