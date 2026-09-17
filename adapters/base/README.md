# Adapter: base

Generates the portable `AGENTS.md` facts file consumed by every harness
(Codex, Jules, Aider, Copilot, Cursor, OpenCode, ...). No skills, no
permissions, no hooks — facts only.

- Template: `AGENTS.md.tmpl` with `{{projectName}}`, `{{coreVersion}}`,
  `{{workflow}}` vars (Mustache-free substitution).
- Output: `./AGENTS.md` with a `DO NOT EDIT` header.
- No unmappable concepts: this adapter is the portability floor.
