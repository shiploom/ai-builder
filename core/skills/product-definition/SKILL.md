---
name: product-definition
description: Define personas, user flows, FR/NFR, MVP vs future scope, acceptance IDs, and success metrics. Use when turning shaped ideas or research into product requirements.
license: MIT
compatibility: base-spec
metadata:
  domain: product
  version: "1.0.0"
---

# Product Definition

## Purpose

Produce `product/*` artifacts precise enough that architecture can be
decided and acceptance locked without re-interpreting intent.

## Inputs

- Approved `idea.md` plus `research/*` (market, competitors, evidence).
- Brownfield: `brownfield/repo-map.md` for existing capabilities.

## Outputs

- `product/requirements.md` (FR/NFR, MVP/future, success metrics).
- `product/personas.md`, `product/user-flows.md`, `product/feature-plan.md`.
- Every FR cites Acceptance-IDs (ACC-xxx drafted alongside, locked later
  by `spec-to-acceptance`).

## Prerequisites

- `idea.md` exists. Load-bearing research claims carry evidence grades;
  ungraded claims are treated as opinions.

## Methodology

1. Write personas: goals, constraints, technical comfort, and what each
   persona is allowed to approve (scope / spend / prod).
2. Write user flows per FR: numbered steps, entry point, happy path,
   error paths, and approval gates encountered.
3. Write FRs as testable statements, each with Acceptance-IDs. Write
   NFRs with numeric bounds (latency, throughput, cost, availability).
4. Split MVP vs future with a reason per deferred item. Anti-pattern:
   competitor-feature dumping — every feature traces to a persona need.
5. Set numeric success metrics to be checked post-deploy (these can
   reopen requirements via supersede, never silent edit).
6. Keep `## Open questions` non-empty until human scope approval.

## Constraints

- No architecture decisions (options belong in ADRs, choices to humans).
- No vague criteria: "fast", "secure", "scalable" must carry numbers or
  move to open questions (`validate --strict` enforces this at lock).
- No orphan slices in the feature plan: each traces to a requirement.

## Tools / MCP

- None required. Quarantined `cap.web.search` only to confirm stated
  pricing or platform limits, cited as evidence.

## Verification

- `shiploom validate --strict` passes on `product/*`.
- Every FR has ≥1 Acceptance-ID; every NFR has a number.
- MVP/future split reviewed by the human scope gate.

## Examples

- FR1: "A user with a valid account receives a reset email within 60
  seconds of requesting it. (Acceptance: ACC-101)" — testable, timed,
  with an owner criterion.
