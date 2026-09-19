# Schemas (normative, MVP set)

JSON Schema draft 2020-12. Validated offline by `shiploom validate` (Go, stdlib-only).

| File | Validates | Spec |
|---|---|---|
| `artifact-frontmatter.schema.json` | YAML frontmatter of every artifact Markdown file | §11.1 |
| `acceptance.schema.json` | `acceptance/*.json` locked criteria + oracle refs | §11.2 |
| `skill.schema.json` | `SKILL.md` frontmatter (base-spec + `x-shiploom-harness` extras) | §11.3 |
| `workflow.schema.json` | `workflows/*.md` frontmatter (steps, gates, budgets, retries) | §11.4 |
| `hook.schema.json` | `hooks/registry.json` entries | §11.5 |
| `policy.schema.json` | JSON-policy packs (default-deny) | §33-D3, §22 |
| `mcp-registry.schema.json` | `./.shiploom/mcp-registry.json` (`cap.*` abstraction) | §11.12, §20 |
| `verification-report.schema.json` | `verification/verification-report.md` frontmatter + `verify --report` JSON | §17 |
| `trace-link.schema.json` | `trace.json` entries (`Requirement→…→Result` links) | §16 |

Rules:

- `$schema` MUST be `https://json-schema.org/draft/2020-12/schema`.
- `$id` MUST be `https://shiploom.dev/schemas/<name>`.
- `validate.py` implements the subset needed offline (types, enums, patterns, required, arrays). Full draft compliance is not required in MVP; unknown keywords are ignored with a warning, never an error.
