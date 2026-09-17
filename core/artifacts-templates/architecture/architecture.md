---
id: PLAN-003
kind: plan
title: "System architecture (template)"
status: proposed
provenance:
  - type: agent
    ref: "skills/architecture-design"
    confidence: medium
    date: 2026-09-17
links:
  requires: [REQ-001]
  decided_by: [ADR-001]
  implemented_by: []
  tested_by: []
  verified_by: []
owner: specifier
version: 1
---

# System architecture

> Cloud-agnostic capability blocks (compute / db / auth / queue / CDN);
> provider binding happens only in `deployment/*` via project MCP.

## Components

Services, data stores, queues, and trust boundaries between them.

## Data flow

Request lifecycle across components, including failure paths.

## API surface

Contracts first; import-graph / contract diffs fail verification on drift.

## Open questions

Mandatory until arch approval. Build cannot start on unapproved arch.
