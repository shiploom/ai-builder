---
name: post-deploy-validate
description: Check deployed behavior against locked acceptance and success metrics, and reopen requirements on mismatch. Use after every deploy that required a smoke report.
license: MIT
compatibility: base-spec
metadata:
  domain: deployment
  version: "1.0.0"
---

# Post-Deploy Validate

## Purpose

Close the loop: demonstrate the deployed system meets locked acceptance
and product success metrics — or reopen requirements through supersede,
never silent edit.

## Inputs

- `deployment/smoke-report.md` (must read pass).
- Locked `acceptance/*.json` + product success metrics.
- Telemetry access (logs, metrics, seeded probes) for the deployed target.

## Outputs

- `verification/post-deploy-report.md`: per-criterion observed vs expected,
  metric values vs targets, and either a Done confirmation or a
  supersede proposal with evidence.

## Prerequisites

- Smoke gate passed; deployment audit entry recorded (who/when/rollback).
- Metrics have numeric targets; unmeasurable "success" returns for revision.

## Methodology

1. Re-run oracle checks against the live target (seeded data, bounded
   timeouts); record observed values beside expected ones.
2. Compare success metrics to targets over the agreed window (not a
   single lucky sample); note sample size and window explicitly.
3. On mismatch: file a supersede proposal (new artifact version, changed
   requirement, evidence, owner) and route to human scope review. Never
   edit the approved requirement in place, never move the metric.
4. On match: confirm Done in the report with links
   Requirement→Decision→Implementation→Test→Verification Result.
5. Feed escapes back: every post-deploy mismatch becomes a seeded fault
   for the verifier catch-rate suite (AC3 loop).

## Constraints

- No prod writes during validation beyond seeded probes; read-only
  against real user data unless the approval explicitly allows more.
- A failing post-deploy check triggers the rollback plan, not a hotfix
  thread — hotfixes are new slices with new acceptance.

## Tools / MCP

- Read-only observability (`cap.logs.query`, `cap.metrics.read`) under
  project scope; seeded-probe scripts versioned with the deployment.

## Verification

- Report links every acceptance ID to an observed value; mismatches link
  supersede proposals, never edits.
- Rollback readiness re-confirmed (plan still rehearsed, still current).

## Examples

- Reset-email p95 delivery 41s vs 60s bound over 24h/312 deliveries →
  Done confirmed; token-expiry boundary probed at 15m00s → pass.
