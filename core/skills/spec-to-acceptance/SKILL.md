---
name: spec-to-acceptance
description: Convert approved requirements into locked, measurable acceptance criteria plus hidden oracle implementations. Use when requirements and architecture are approved and build must be gated on demonstrable Done.
license: MIT
compatibility: base-spec
metadata:
  domain: verification
  version: "1.0.0"
---

# Spec to Acceptance

## Purpose

Turn each FR/NFR into ACC-xxx criteria with `howToVerify` plus a hidden
oracle implementation, then lock both so builders cannot redefine Done.

## Inputs

- Approved `product/requirements.md` and `architecture/*`.
- Human scope/arch approvals recorded (build never starts without them).

## Outputs

- `acceptance/*.json` per `schemas/acceptance.schema.json`.
- Oracle implementations under `./.shiploom/.oracle/` (one file per
  criterion, referenced by `oracleRef`).
- `shiploom lock` recorded in the manifest (hashes of criteria + oracles).

## Prerequisites

- Requirements and architecture approved. Drafting acceptance on
  unapproved scope bakes drift into the lock.

## Methodology

1. Write one ACC-xxx per testable statement. Prefer `script` / `http` /
   `browser` verifiers over `human`; human verifiers need an `expect`
   string ≥10 chars describing exactly what the reviewer checks.
2. Purge vagueness: "fast", "secure", "scalable" become numbers with
   bounds and conditions, or move back to open questions.
   `validate --strict` rejects the rest.
3. Write the oracle implementation per criterion: the independent check
   the Verifier will run (script with exit codes, HTTP expectations, or
   seeded-data assertions). The oracle must pass on the intended
   behavior and fail on at least one plausible wrong behavior.
4. Present plain-language cards to the human (Nadia-approved narrative):
   what is promised, how it is demonstrated, cost/risk notes.
5. On human approval, run `shiploom lock`: hashes criteria + oracles
   into the manifest and appends the audit entry. From here the
   Implementer sees statements only; oracle paths never appear in
   builder context (adapters exclude `.oracle/`).

## Constraints

- Oracle implementations live ONLY under `./.shiploom/.oracle/`
  (mode 0700, gitignored). Absolute `oracleRef` paths and `..` are rejected.
- Never reuse the builder's tests as oracles (shared blind spots).
- Re-lock after any scope change; silent acceptance edits after lock
  fail `shiploom lock --check` by hash mismatch.

## Tools / MCP

- Read-only repo access to pin commands and versions in `howToVerify`.
  No network at lock time except pinned doc confirmation.

## Verification

- `shiploom validate --strict` passes on `acceptance/*.json`.
- `shiploom lock` succeeds: every `oracleRef` resolves inside the vault,
  vault is 0700, oracles are git-ignored, hashes recorded.
- Guided-wizard exit check: the human can explain scope, cost, and risk
  from the cards alone.

## Examples

- ACC-101: "reset email arrives within 60 seconds" → `howToVerify`
  script `pytest tests/e2e_reset.py -q`, oracle
  `oracle/ACC-101.sh` asserting delivery-timestamp deltas on seeded data.
