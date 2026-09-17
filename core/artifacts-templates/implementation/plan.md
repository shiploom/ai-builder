---
id: PLAN-006
kind: plan
title: "Implementation plan (template)"
status: proposed
provenance:
  - type: agent
    ref: "skills/implement-scoped-diff"
    confidence: medium
    date: 2026-09-17
links:
  requires: [REQ-001]
  decided_by: [ADR-001]
  implemented_by: []
  tested_by: []
  verified_by: []
owner: implementer
version: 1
---

# Implementation plan

> Minimal diff first. Scoped context only (module + direct callers +
> types). No silent debt paydown: update `brownfield/debt-register.md`
> instead of expanding scope.

## Task slices

| Slice | Files | Acceptance | Characterization tests |
|---|---|---|---|
| 1 | ... | ACC-001 | ... |

## Rollback

How each slice is reverted independently.

## Open questions
