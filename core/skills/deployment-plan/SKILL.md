---
name: deployment-plan
description: Produce an environment-parameterized deployment plan with cost, blast radius, rollback, and smoke criteria. Use when architecture is approved and a slice is ready to leave the build environment.
license: MIT
compatibility: base-spec
metadata:
  domain: deployment
  version: "1.0.0"
---

# Deployment Plan

## Purpose

Turn approved architecture into a per-target deployment plan that names
cost, blast radius, rollback, and smoke criteria before anything ships.

## Inputs

- Approved `architecture/*` (capability blocks: compute/db/auth/queue/CDN).
- Target environment(s): local, Docker, Kubernetes, AWS, or preview host.
- Deployment approver constraints (Ops persona: downtime window, budget).

## Outputs

- `deployment/<target>-plan.md` (arch ref, env diff, cost, risks,
  rollback, smoke) per `core/artifacts-templates/deployment/`.

## Prerequisites

- Human arch approval recorded. Plans on unapproved arch bake drift in.
- Acceptance locked; deploy gates verify against locked criteria only.

## Methodology

1. Bind capability blocks to the target provider only at the edge, via
   project-scoped MCP — never leak provider SDKs or names into core arch.
2. Write the environment diff (current vs target: versions, OS/shell,
   secrets refs, network egress, build limits).
3. Cost the plan (compute, transfer, per-seat/usage fees) and size the
   blast radius (data touched, downtime window, affected users).
4. Write exact reversal steps and test them before the prod gate; a
   rollback that has never been rehearsed is a wish, not a plan.
5. Define smoke checks (HTTP/exit codes/screenshots, seed-data checks,
   env validation) required in `smoke-report.md` before prod.
6. Route through human gates: infra/spend/prod approvals are
   default-deny; two-person option for the enterprise pack.

## Constraints

- Core never auto-provisions infra, spends, or deploys to prod without a
  recorded human approval (deny-means-deny, audit-chained).
- No credentials in plans: env/keychain refs only (`env:NAME`).
- One target per plan file; multi-env rollouts sequence plans, never merge.

## Tools / MCP

- Provider MCPs under project scope with explicit auth (e.g.
  `cap.infra.plan`, `cap.cost.estimate`); quarantined outputs, TTL caches.
- Local validators: `terraform validate`, `docker build`, `kubeconform`
  where the gate scripts wrap them.

## Verification

- `shiploom validate --strict` passes; plan links arch + acceptance IDs.
- Cost/risk/rollback/smoke sections all non-empty; approver can explain
  the blast radius from the card alone (Nadia trial).

## Examples

- Staging deploy of the password-reset slice: relay reuse (no new vendor),
  zero-downtime window, revert commit as rollback, 60s-delivery seed check.
