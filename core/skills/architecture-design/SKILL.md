---
name: architecture-design
description: Design system architecture via options matrix, ADRs, HLD/LLD with numeric NFR budgets. Use when requirements are approved and implementation needs a buildable, verifiable plan.
license: MIT
compatibility: base-spec
metadata:
  domain: architecture
  version: "1.0.0"
---

# Architecture Design

## Purpose

Produce `architecture/*` (ADRs, architecture, HLD, LLD) that an
Implementer can slice and a Verifier can drift-check against the code.

## Inputs

- Approved `product/requirements.md` (FR/NFR with numbers).
- Brownfield: `brownfield/repo-map.md` + `brownfield/impact-plan.md`.
- Team skill, cost, and ops constraints from the human arch gate.

## Outputs

- `architecture/decisions.md` (ADRs: Context / Options / Chosen /
  Rejected / Consequences).
- `architecture/architecture.md`, `architecture/hld.md`,
  `architecture/lld.md` per core templates.

## Prerequisites

- Requirements approved. Unapproved requirements mean unbuildable arch;
  stop and escalate instead of guessing.

## Methodology

1. List NFRs and constraints, then build a cloud-agnostic options
   matrix (compute / db / auth / queue / CDN as capability blocks).
2. Propose 2–3 stack options with trade-offs (team skill, NFR fit,
   cost, ops burden). The human picks; core never enforces a stack.
3. Record each decision as an ADR with rejected alternatives and
   consequences (auditable without replaying chat).
4. Allocate numeric NFR budgets per component (latency / cost /
   availability must sum correctly across the call path).
5. Define module map, dependency directions, API contracts, error
   conventions, and observability stubs. Contract drift fails
   verification, so contracts are written before code.
6. Keep `## Open questions` non-empty until human arch approval.

## Constraints

- No provider binding outside `deployment/*` (no AWS/GCP/Azure SDK
  names in arch; bind at the edge via project MCP).
- No implementation diffs; no code in architecture artifacts.
- No silent NFR weakening: missed budgets replan through re-approval.

## Tools / MCP

- Read-only repo access for brownfield grounding (symbol graph over
  file lists). Quarantined `cap.web.search` for versioned docs only,
  pinned by version and cited.

## Verification

- `shiploom validate --strict` passes on `architecture/*`.
- Every ADR has all five sections; every NFR has a budget allocation.
- Human arch approval recorded before any Implementer starts.

## Examples

- ADR-101: "Token delivery via existing SMTP relay" — context cites
  measured p99 delivery; options matrix rejects a new vendor on cost
  without evidence of need; consequence pins re-approval on switch.
