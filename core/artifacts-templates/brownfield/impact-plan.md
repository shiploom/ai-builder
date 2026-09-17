---
id: PLAN-008
kind: plan
title: "Impact plan (template)"
status: proposed
provenance:
  - type: agent
    ref: "skills/impact-analysis"
    confidence: medium
    date: 2026-09-17
links:
  requires: [RPT-005]
  decided_by: []
  implemented_by: []
  tested_by: []
  verified_by: []
owner: specifier
version: 1
---

# Impact plan

> Scoped task context: module + direct callers + types only. Muted-repo
> masks (`archive/`, generated code) prevent debt extrapolation.

## Change surface

Files, callers, types, migrations touched, with blast radius per item.

## Characterization tests

Pre-change behavior snapshots captured before any edit (mandatory when
regression coverage is thin; residual risk noted for human sign-off).

## Rollback

## Open questions
