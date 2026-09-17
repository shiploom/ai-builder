# adapters/ — single-source `core/*` → harness files

MVP (PR9): `base` (root `AGENTS.md`), `claude` (`CLAUDE.md` +
`.claude/skills/`), `opencode` (`.opencode/skills/`, facts via base
`AGENTS.md`). Skills are base-spec byte copies plus a `DO NOT EDIT`
HTML comment after frontmatter (frontmatter itself is untouched so
base validators keep passing).

Deferred post-MVP (PR10 closed) with hooks/policy packs: Claude `settings.json`
hook mapping, OpenCode per-skill permissions / `opencode.json`
(pinned against vendor docs first), per-harness READMEs document the
gap — never silent.

`shiploom adapters --generate <harness|all>` is idempotent
(created/updated/unchanged report). Never hand-edit generated files.
