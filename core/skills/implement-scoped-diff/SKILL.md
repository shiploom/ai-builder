---
name: implement-scoped-diff
description: Implement one approved task slice as a minimal diff with tests, docs, and an attempt-log entry. Use when architecture and locked acceptance exist and a build slice is ready.
license: MIT
compatibility: base-spec
metadata:
  domain: implementation
  version: "1.0.0"
---

# Implement Scoped Diff

## Purpose

Turn one approved slice (arch + acceptance + scoped context) into the
smallest diff that satisfies the acceptance criteria, with tests and an
attempt-log entry — nothing more.

## Inputs

- Approved `architecture/*` and locked `acceptance/*.json` for the slice.
- Scoped repo context: module + direct callers + types (muted-repo
  masks apply to `archive/` and generated code).

## Outputs

- Minimal diff + tests + docs.
- One `implementation/attempt-log.md` entry per attempt (diff summary,
  gate outputs, what failed and why).

## Prerequisites

- Acceptance locked (`shiploom lock --check` passes). Building on
  unlocked acceptance is rework by definition — stop and lock first.
- The slice fits its blast radius; oversized slices replan before code.

## Methodology

1. Read the slice acceptance statements. You see statements only: the
   vault (`./.shiploom/.oracle/`) is unreadable to you by design, and
   attempts to read it are logged and blocked.
2. Capture characterization tests first on brownfield touches
   (record current behavior before editing).
3. Write the minimal diff: no drive-by refactors, no silent debt
   paydown (record debt in `debt-register.md` instead), no speculative
   abstraction (Simplicity Gate).
4. Add tests derived from the acceptance statements, then run the
   deterministic gates for the slice.
5. Append the attempt-log entry. Output is `attempt`, never `result`.
6. On verification failure: fix within the slice, ≤4 attempts total,
   then mandatory replan — never a fifth retry.

## Constraints

- Least-privilege tools; write allowlisted slice paths only.
- No prod, protected-branch, infra, or destructive-migration changes
  without a recorded human approval.
- Never redefine tests to match your code; never edit verifier output,
  the manifest, or the audit log.

## Tools / MCP

- Least-privilege shell + test runner via harness (scoped, logged).
  New capabilities require orchestrator approval first.

## Verification

- Deterministic gates pass for the slice; `verify` step mechanics
  (gate set + report twin) enforced by the orchestrator.
- Attempt log present with gate outputs; diff touches only the slice.

## Examples

- Slice "auth reset email" with ACC-101: characterization test for the
  existing mailer, minimal diff adding the timed-delivery path, new
  test asserting the 60s bound, attempt-log entry with gate outputs.
