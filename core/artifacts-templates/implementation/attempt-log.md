---
id: RPT-003
kind: report
title: "Implementation attempt log (template)"
status: proposed
provenance:
  - type: agent
    ref: "skills/implement-scoped-diff"
    confidence: medium
    date: 2026-09-17
links:
  requires: [REQ-001]
  decided_by: []
  implemented_by: []
  tested_by: []
  verified_by: []
owner: implementer
version: 1
---

# Implementation attempt log

> Append-only per attempt. Builder output is `attempt`, never `result`.
> The verifier never sees this file (fresh context), only the diff.

## Attempt 1

Date, slice, diff summary, gate outputs, what failed and why.

## Attempt 2 (retry ≤4, then mandatory replan)
