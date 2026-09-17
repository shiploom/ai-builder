---
name: debug-triage
description: Reproduce first, then bisect via checkpoints and attempt logs within bounded retries. Use when a gate fails, a verifier rejects, or behavior diverges from acceptance.
license: MIT
compatibility: base-spec
metadata:
  domain: debugging
  version: "1.0.0"
---

# Debug Triage

## Purpose

Convert failures into minimal reproductions and bounded fixes — no
shotgun edits, no retry storms, no proving impossible infrastructure
possible for a day.

## Inputs

- The failure: gate output, verifier report section, or divergent
  behavior with steps to observe it.
- Attempt log, checkpoints, manifest state, and the locked acceptance.

## Outputs

- A repro (command or script, deterministic or flakiness-noted).
- A diagnosis (root cause + evidence chain) and a scoped fix slice, or
  an escalation with stop rationale.

## Prerequisites

- Read the attempt log first: the same fix must never be tried twice
  without new evidence (each attempt is logged with gate outputs).

## Methodology

1. Reproduce before theorizing: smallest command that shows the failure;
   note environment (OS/shell/venv, pinned versions) — env blindness is
   a top failure mode (§4.16).
2. Bisect with the tools at hand: checkpoints and attempt history for
   workflow faults; characterization snapshots pre/post for behavior
   drift; contract diffs for interface faults.
3. Fix inside the slice's blast radius; expanding scope requires replan,
   not absorption.
4. Count attempts against the ≤4 implement→verify budget; at the cap,
   mandatory replan with new evidence — never a fifth retry.
5. Escalate fast on: impossible infra (Railway rule — stop, don't burn
   budget), missing oracle, absent tool auth, budget at 80%, or
   requirements contradicting each other.

## Constraints

- No edits to verifier output, manifest, audit log, or locked acceptance
  during triage; findings are advisory until a new attempt re-verifies.
- Flaky green is red: determinism gate retries 2×, quarantine on doubt.

## Tools / MCP

- Least-privilege shell/test runner via harness; fresh-context re-runs
  to confirm the repro isn't poisoned exploration (§4.3).

## Verification

- Repro script deterministic (or flakiness characterized with rate);
  diagnosis links evidence lines; fix slice carries acceptance.

## Examples

- Intermittent token-expiry failure: repro pins clock at the 15m
  boundary (deterministic), root cause is wall-clock read in two places,
  fix injects the clock; verifier re-derives from the oracle.
