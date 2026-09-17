---
id: REQ-101
kind: requirement
title: "Password reset requirements (fixture)"
status: approved
provenance:
  - type: agent
    ref: "skills/product-definition"
    confidence: high
    date: 2026-09-17
links:
  requires: [IDEA-101]
  decided_by: [ADR-101]
  implemented_by: []
  tested_by: [ACC-101, ACC-102]
  verified_by: [VR-101]
owner: specifier
version: 2
---

# Password reset requirements (fixture)

## Functional requirements

- FR1: A user with a valid account receives a reset email within 60
  seconds of requesting it. (Acceptance: ACC-101)
- FR2: Reset tokens are rejected after 15 minutes. (Acceptance: ACC-102)

## Non-functional requirements

- NFR1: Token entropy is at least 128 bits, generated with a
  cryptographically strong random source.

## Success metrics

Reset completion rate and median time-to-email, checked post-deploy.
