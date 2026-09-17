---
id: RES-003
kind: report
title: "Evidence log (template)"
status: proposed
provenance:
  - type: agent
    ref: "skills/market-research"
    confidence: medium
    date: 2026-09-17
links:
  requires: []
  decided_by: []
  implemented_by: []
  tested_by: []
  verified_by: []
owner: specifier
version: 1
---

# Evidence log

> Mandatory companion to every research artifact. Synthesizes `market.md`
> and `competitors.md`. Low-confidence load-bearing claims block auto-scope
> and require human confirmation.

## Evidence

| # | Claim | Grade | Source URL | Date | Confidence | Used by |
|---|---|---|---|---|---|---|
| E-001 | ... | Fact | https://... | 2026-09-17 | high | REQ-001 |
| E-002 | ... | Inferred | https://... | 2026-09-17 | medium | ADR-001 |
| E-003 | ... | Marketing | https://... | 2026-09-17 | low | — |

Grades: Fact / Inferred / Opinion / Marketing / Conflicting / Uncertain.

## Conflicts

Where sources disagree, what each says, and how the conflict was resolved
(or escalated to a human).

## Open questions

Claims still needing a second source.
