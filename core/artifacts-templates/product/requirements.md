---
id: REQ-001
kind: requirement
title: "Product requirements (template)"
status: proposed
provenance:
  - type: agent
    ref: "skills/product-definition"
    confidence: medium
    date: 2026-09-17
links:
  requires: [IDEA-001]
  decided_by: [ADR-001]
  implemented_by: []
  tested_by: []
  verified_by: []
owner: specifier
version: 1
---

# Product requirements

> Each requirement lists its Acceptance-IDs. Vague criteria ("fast",
> "secure") fail `validate --strict` unless paired with a measurable
> `howToVerify` in `acceptance/*.json`.

## Functional requirements

- FR1: ... (Acceptance: ACC-001)
- FR2: ... (Acceptance: ACC-002)

## Non-functional requirements

- NFR1: ... with numeric bound, e.g. "p95 latency under 300 ms at 100 rps"
- NFR2: ...

## MVP scope / future

Explicitly in MVP vs deferred, with a reason per deferred item.

## Success metrics

Numeric criteria checked post-deploy (can reopen requirements via supersede).

## Open questions

Mandatory until approved. Blocks build: steps may only consume approved artifacts.
