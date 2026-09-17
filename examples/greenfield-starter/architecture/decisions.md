---
id: ADR-101
kind: decision
title: "ADR-101: Token delivery via existing SMTP relay"
status: approved
provenance:
  - type: human
    ref: "scope-approval:2026-09-17"
    confidence: high
    date: 2026-09-17
links:
  requires: [REQ-101]
  decided_by: []
  implemented_by: []
  tested_by: []
  verified_by: []
owner: specifier
version: 1
---

# ADR-101: Token delivery via existing SMTP relay

## Context

Single-region MVP, existing relay with measured p99 delivery under 20
seconds over the last 30 days.

## Options

| Option | Cost | Ops burden | Rejected because |
|---|---|---|---|
| A: existing SMTP relay | sunk | none | — (chosen) |
| B: new email vendor | per-email fees + contract | DKIM warm-up | unjustified for one slice |

## Chosen

Option A, with delivery-time acceptance ACC-101 guarding the assumption.

## Rejected

Option B: cost without evidence of need.

## Consequences

Vendor switch requires a new ADR plus re-approval before build.
