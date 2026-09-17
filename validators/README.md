# validators/ — stdlib-only offline validators

**Constraint:** Python standard library only. No `jsonschema`, no `yaml` at runtime (MASTER_SPEC §11.2, AGENTS.md).

- `validate.py` (PR2): schemas + frontmatter + links → stdout `{ok, errors[], warnings[]}`, exit `0/2`.
- `trace.py` (PR3): trace index `Requirement→…→Result` → stdout `{ok, trace, errors, warnings}`, exit `0/2`; `--out FILE` writes `trace.json` (schema-checked).
- `status.py` (PR3): read-only status (artifacts + counts + trace summary) → human text default, `--json` for scripting. Budgets/gates are manifest-owned (merged by `shiploom status` when a manifest exists).
- `cli/policy.py` (PR10, shipped): JSON-policy evaluator + hook matching (deny > require-approval > allow) → policy gates deny with exit `0/3` semantics (`0` allow/pass, `3` deny).
- Frontmatter parsed with a minimal stdlib YAML-subset reader (flat `key: value`, lists, nested one-level maps) — full YAML via `pyyaml` is dev-only for tests, never at runtime.
