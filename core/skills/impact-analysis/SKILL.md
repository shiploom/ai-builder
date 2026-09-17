---
name: impact-analysis
description: Analyze blast radius, callers, types, migrations, and rollback for a scoped brownfield change. Use when a repo map exists and a change plan must precede any edit.
license: MIT
compatibility: base-spec
metadata:
  domain: brownfield
  version: "1.0.0"
---

# Impact Analysis

## Purpose

Produce `brownfield/impact-plan.md`: the minimal change surface with
blast radius per item, mandatory characterization tests, and rollback —
approved before any edit.

## Inputs

- Approved `brownfield/repo-map.md` (low-confidence areas narrow scope).
- Change request: what behavior must differ, with acceptance IDs.

## Outputs

- `brownfield/impact-plan.md` per core template (surface, callers,
  types, migrations, characterization tests, rollback, questions).

## Prerequisites

- Repo map approved. Analysis on an unmapped area stops and maps first.

## Methodology

1. Enumerate touched files, direct callers, shared types, and
   migrations. Size the blast radius per item (rows, callers, flags).
2. Capture pre-change characterization snapshots for every touched
   behavior before any edit. Where regression coverage is thin, state
   the residual risk plainly for human sign-off.
3. Prefer contract stubs where DI or reflection hides the true call
   graph; verify against the live runtime with seeded data over
   interface claims.
4. Write per-item rollback that is independently reversible.
5. Keep the plan minimal: cross-cutting refactors become their own
   approved slices, never stowaways.

## Constraints

- No edits during analysis (plan first, implement after approval).
- No scope creep into adjacent debt; debt goes to the register.
- Human plan approval required before the Implementer starts.

## Tools / MCP

- Read-only repo tools plus the test runner for characterization
  capture (snapshots committed alongside the plan).

## Verification

- `shiploom validate --strict` passes on the impact plan.
- Every touched behavior has a characterization test or an explicit
  residual-risk note accepted by the human gate.

## Examples

- Pagination fix touches `list_users` + 3 callers: snapshots capture
  current off-by-one behavior first, plan reverts per caller, rollback
  is a single revert commit.
