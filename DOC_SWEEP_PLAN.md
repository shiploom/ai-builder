# Doc-Sweep Plan: stale markers → as-built truth

**Date:** 2026-09-17 · **Status:** executed
**Goal:** eliminate the 9 doc-vs-reality contradictions from the deferred-items
inventory, so the tree stops promising PR10 work that already shipped (or didn't).
**Scope:** docs/comments only — zero behavior, schema, or test changes.
`MASTER_SPEC.md` (normative) untouched. BUILD_PLAN.md annotated in place (chosen
over appendix/frozen).

## Edits

1. `validators/README.md:8` — `policy_eval.py (PR10)` never existed; shipped as
   `cli/policy.py`. Reword to evaluator + hook matching + exit 3.
2. `core/README.md:6,8-9,11-14` — drop shipped "(PR10)" tags; consolidate the
   stale duplicate skills line; fix "`registry.json` + Markdown definitions"
   (only `registry.json` exists in `core/hooks/`).
3. `cli/manifest.py:6` — resume/replay no longer "arrives in PR7"; live in
   `cli/run.py` since PR7.
4. `validators/validate.py:14-15` — trace file writing no longer "deferred to
   PR4"; lives in `validators/trace.py --out`.
5. `validators/status.py:87-88` — budgets no longer "arrive with PR4"; merged by
   `shiploom status` when a manifest exists.
6. `cli/shiploom.py:408` + `cli/README.md:9` — approve "no policy eval until
   PR10" is misleading post-PR10: `run` enforces policy gates, `approve` is the
   human-override path.
7. `cli/README.md:12-13` — "Planned (PR10)" → post-MVP list (minus shipped items).
8. `BUILD_PLAN.md:144` (§5.7) — MVP adapter dirs claimed `settings.json` +
   `opencode.json`; annotate as-built PR10 deferral with reasons + README pointers.
   §5.9 left as-is (conditioned on future releases, not false).
9. Adapter docs — `adapters/README.md:9`, `adapters/claude/README.md:10-11`,
   `adapters/opencode/README.md:8-10`, `adapters/claude/CLAUDE.md.tmpl:26`
   ("## Hooks (PR10)") → post-MVP wording. Template ships to user projects, so
   the fix propagates on next `--generate`.
10. Conformance profiles — `tests/conformance/{claude,opencode}/capabilities.json`
    descriptions + `permissionsModel`, `tests/conformance/README.md:12` → post-MVP
    wording. Constraint: `harness` + `mustProduce` keys stay byte-identical
    (`test_conformance_profiles_valid_json` asserts them).

## Verification

1. `pytest tests/unit -q` — 190 green (no test asserts on changed strings).
2. `python validators/validate.py --strict .` — covers the `CLAUDE.md.tmpl` edit.
3. JSON parse check over `tests/conformance/*/capabilities.json`.
4. `ac-demo.sh` out of scope (doc-only; suite + strict suffice).

## Risks

- Whitespace-only edits caused line-join corruption 3× before: every edit uses
  multi-line anchors with real text changes + `ast.parse` check on touched `.py`.
- Stale `__pycache__` masked a corruption once: clear caches before the suite run.
