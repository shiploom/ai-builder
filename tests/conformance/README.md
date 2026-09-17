# tests/conformance/ — harness compatibility profiles + fixtures

Per-harness `capabilities.json` (events supported, perms model, skill
fields honored/ignored, MCP transports) plus shared fixtures. Per
MASTER_SPEC §29 the full runner (`shiploom conformance --harness`,
recorded transcripts + nightly live-LLM lane, badges) is post-MVP;
PR9 delivers the profiles, the shared fixture
(`examples/greenfield-starter`), and generator conformance tests
(`tests/unit/test_adapters.py`):

- `base/` — portability floor: `AGENTS.md` facts only.
- `claude/` — `CLAUDE.md` + `.claude/skills/`; hook mapping post-MVP.
- `opencode/` — `.opencode/skills/` + base `AGENTS.md`; `opencode.json`
  deferred until pinned against vendor docs.
