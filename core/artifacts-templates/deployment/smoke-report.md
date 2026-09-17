---
id: RPT-004
kind: report
title: "Smoke report (template)"
status: proposed
provenance:
  - type: agent
    ref: "skills/post-deploy-validate"
    confidence: medium
    date: 2026-09-17
links:
  requires: [DEP-001]
  decided_by: []
  implemented_by: []
  tested_by: []
  verified_by: []
owner: verifier
version: 1
---

# Smoke report

> Required before the prod gate. Preview- deploy checks: HTTP status,
> exit codes, seeded-data checks, environment validation (pinned
> versions, OS / shell).

## Checks

| Check | Command | Expected | Observed |
|---|---|---|---|
| health endpoint | ... | HTTP 200 | ... |

## Verdict

pass | fail (+ rollback trigger if applicable).
