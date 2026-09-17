---
name: change-plan
description: Plan the minimal brownfield diff with blast radius, characterization coverage, and independent rollback. Use after impact analysis approves, before any edit.
license: MIT
compatibility: base-spec
metadata:
  domain: brownfield
  version: "1.0.0"
---

# Change Plan

## Purpose

Turn an approved impact analysis into an executable, minimal change plan
whose every touched behavior is characterized and independently
revertible.

## Inputs

- Approved `brownfield/impact-plan.md` (surface, callers, types,
  migrations, blast radius).
- Locked acceptance for the touched behavior.
- Current characterization snapshots (or explicit residual-risk notes).

## Outputs

- `implementation/plan.md` slice: ordered edits, per-item tests,
  per-item rollback, stop conditions.

## Prerequisites

- Human plan approval on the impact analysis. Planning on unmapped
  areas stops and maps first.

## Methodology

1. Order edits dependency-first; each item independently revertible
   (one revert commit per item, verified rollback, no flag-day chains).
2. Attach characterization coverage per item: pre-change snapshot
   captured, or residual-risk note accepted at the human gate.
3. Keep the plan minimal: one behavior per item; cross-cutting refactors
   split into their own approved slices (no stowaways, no silent debt
   paydown — debt goes to the register).
4. State stop conditions up front (what observation halts the plan and
   triggers replan: contract drift, wider blast radius, missing oracle).
5. Size retries: the plan must fit the ≤4 implement→verify budget or be
   re-sliced before starting.

## Constraints

- No edits during planning; the plan is reviewed before the Implementer
  starts.
- Scope creep absorbs nothing: new findings during execution replan
  through approval, never inline expansion.

## Tools / MCP

- Read-only repo tools plus test runner for snapshot capture; symbol
  graph over file lists for caller verification.

## Verification

- Every plan item traces to acceptance; every touched behavior has a
  snapshot or an accepted risk note; rollbacks are per-item and tested.

## Examples

- Pagination off-by-one: item 1 captures snapshots, item 2 fixes the
  offset with boundary tests, item 3 updates callers; each reverts alone.
