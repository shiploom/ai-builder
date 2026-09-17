---
id: DEP-001
kind: plan
title: "Deployment plan (template)"
status: proposed
provenance:
  - type: agent
    ref: "skills/deployment-plan"
    confidence: medium
    date: 2026-09-17
links:
  requires: [REQ-001]
  decided_by: [ADR-001]
  implemented_by: []
  tested_by: []
  verified_by: []
owner: human
version: 1
---

# Deployment plan

> Provider binding happens here, not in core. Human approval required
> before prod (policy gate + recorded `approve`).

## Target

Local / Docker / K8s / cloud module + environment diff.

## Cost and blast radius

Estimated spend, affected data, downtime window.

## Rollback

Exact reversal steps, tested before the prod gate.

## Open questions
