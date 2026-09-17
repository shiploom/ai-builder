---
name: verify-independent
description: Independently verify a slice in fresh context using requirements, diff, gate outputs, and the hidden oracle. Use when an implementation attempt is ready for a Done assessment.
license: MIT
compatibility: base-spec
metadata:
  domain: verification
  version: "1.0.0"
---

# Verify Independent

## Purpose

Decide pass/fail per acceptance criterion from first principles —
re-deriving checks from requirements plus the hidden oracle — without
builder history, shared blind spots, or code edits.

## Inputs

- Requirements + locked `acceptance/*.json` + oracle implementations.
- The Implementer's diff and deterministic gate outputs.
- Explicitly NOT builder chat history, rationale, or attempt logs.

## Outputs

- `verification/verification-report.md` (per-acceptance results, gate
  summary, test-quality table, verdict with repro on failure).
- `verification/verification-report.json` twin per
  `schemas/verification-report.schema.json` (verdict pass, criteria locked).

## Prerequisites

- Fresh context: new session or compaction boundary since any builder
  work. Gates have been run (no gates, no verdict).
- Oracle covers every claimed criterion; scope unchanged since lock
  (`shiploom lock --check` passes).

## Methodology

1. For each criterion, re-derive the check from the requirement text —
   do not replay the builder's tests. Run the oracle; spot-check
   evidence (timestamps, hashes, boundary values).
2. Confirm the deterministic gate set: build / type / lint / test /
   secrets / dep-audit per the gate report. A green report with an
   unrun gate is a fail, not a pass.
3. Fill the test-quality table: compile, 2× determinism, no-network,
   mutation-sampled (report-only in MVP). Brittle or circular tests
   fail the criterion they claim to cover.
4. Record pass/fail per criterion with evidence and `oracleUsed`
   flags, then a single verdict. On failure: repro steps and failure
   scope, advisory diff suggestions only.
5. Differential check on brownfield: characterization snapshots
   pre/post diff, contract diffs, invariant checks (row counts,
   idempotency). Regressions fail regardless of new-behavior passes.

## Constraints

- Never edit product code or tests to green; any edit starts a new
  attempt with re-verification.
- Never downgrade failures to warnings to unblock a merge.
- Never verify scope that changed after the oracle was locked.

## Tools / MCP

- Read-only repo access plus oracle execution. No builder tools, no
  write scopes.

## Verification

- `shiploom verify` gate set passes; report twin schema-valid with
  verdict pass and all criteria locked (enforced by the orchestrator's
  verify-step checker).
- Sampled human audit of verifier outputs stays green over time.

## Examples

- ACC-101 re-derived: seed two accounts, trigger reset, assert both
  delivery deltas under 60s from oracle logs — independent of the
  builder's e2e test, same bound, different path.
