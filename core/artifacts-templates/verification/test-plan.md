---
id: PLAN-007
kind: plan
title: "Test plan (template)"
status: proposed
provenance:
  - type: agent
    ref: "skills/verify-independent"
    confidence: medium
    date: 2026-09-17
links:
  requires: [REQ-001]
  decided_by: []
  implemented_by: []
  tested_by: []
  verified_by: []
owner: verifier
version: 1
---

# Test plan

> Authored from requirements, never from builder output. Covers the
> deterministic gates (build / type / lint / unit / contract / secrets /
> dep-audit) plus oracle checks per acceptance criterion.

## Gate matrix

| Gate | Command | Expected | Timeout |
|---|---|---|---|
| build | ... | exit 0 | 300s |
| secrets-scan | ... | exit 0, no findings | 120s |

## Oracle checks

| Acceptance | Oracle | Expected |
|---|---|---|
| ACC-001 | oracle/ACC-001.sh | exit 0 |

## Open questions
