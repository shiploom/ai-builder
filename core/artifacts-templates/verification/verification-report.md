---
id: VR-001
kind: report
title: "Verification report (template)"
status: proposed
provenance:
  - type: agent
    ref: "skills/verify-independent"
    confidence: medium
    date: 2026-09-17
links:
  requires: [REQ-001]
  decided_by: [ADR-001]
  implemented_by: []
  tested_by: [ACC-001]
  verified_by: []
owner: verifier
version: 1
---

# Verification report

> Fresh-context assessment. The verifier cannot edit code: findings are
> advisory, and any edit starts a new attempt with re-verification.
> Machine-readable twin: `verification-report.json` per
> `schemas/verification-report.schema.json`.

## Per-acceptance results

| Acceptance | Result | Oracle used | Evidence |
|---|---|---|---|
| ACC-001 | pass | yes | oracle log hash ... |

## Gate summary

| Gate | Result |
|---|---|
| build | pass |
| secrets-scan | pass |

## Test-quality table

Compile / 2× determinism / no-network / mutation-sampled (report-only MVP).

## Verdict

pass | fail (+ repro steps and scope of failure).
