---
name: greenfield-full-lite
version: 1.0.0
kind: sequential
resume: true
budgets:
  tokens: 800000
  spendUSD: 25
  wallClockH: 8
steps:
  - id: research
    uses: skills/market-research
    consumes: [idea.md]
    produces: [research/market.md, research/evidence.md]
    gate: none
  - id: scope-approval
    gate: human-approval
    onDeny: pause
  - id: product
    uses: skills/product-definition
    consumes: [idea.md, research/market.md]
    produces: [product/requirements.md]
    gate: none
  - id: arch
    uses: skills/architecture-design
    consumes: [product/requirements.md]
    produces: [architecture/decisions.md, architecture/architecture.md]
    gate: none
  - id: arch-approval
    gate: human-approval
    onDeny: pause
  - id: acceptance-lock
    uses: skills/spec-to-acceptance
    consumes: [product/requirements.md, architecture/architecture.md]
    produces: [acceptance/*.json]
    gate: verification
  - id: implement
    uses: roles/implementer.md
    consumes: [architecture/architecture.md, acceptance/*.json]
    produces: [implementation/attempt-log.md]
    retries: 4
    onFail: replan
  - id: verify
    uses: roles/verifier.md
    consumes: [implementation/attempt-log.md, acceptance/*.json]
    produces: [verification/verification-report.md, verification/verification-report.json]
    gate: verification
    retries: 2
    onFail: replan
  - id: merge-approval
    gate: human-approval
    onDeny: pause
---

# Greenfield Full Lite

> Right-sized greenfield path: research → approvals → specify → lock →
> build → verify → merge. Sequential; no DAG. The orchestrator (`shiploom
> run`) advances steps whose gate passes and whose `produces` files exist
> and validate; LLM work (`uses:`) is performed by the harness, and the
> orchestrator enforces order, inputs, and outputs around it.

## Sequence

research → scope-approval → product → arch → arch-approval →
acceptance-lock → implement → verify → merge-approval

## Step contracts

- **research:** consumes `idea.md`; produces evidence-graded `research/*`.
  Pauses until the files exist and pass `validate` (non-strict: links
  may dangle while authoring).
- **scope-approval:** human gate. `shiploom approve scope-approval`
  (or `--deny` with reason). Denied gates halt the workflow.
- **product / arch:** file-grounded Specifier work; same mechanics as research.
- **arch-approval:** human gate on architecture + ADRs.
- **acceptance-lock:** produces `acceptance/*.json`, then the
  `verification` gate runs `shiploom lock --check` semantics: the step
  completes only with a recorded, passing acceptance lock. Failures
  consume no retries here — fix scope or oracles, re-lock, re-run.
- **implement:** ≤4 verification-failure retries, then mandatory replan
  (downstream reset, checkpoint, audit). Produces the attempt log; the
  diff itself is reviewed at merge-approval.
- **verify:** the `verification` gate runs the deterministic gate set
  plus the report-twin check (schema-valid JSON, verdict pass, criteria
  locked); failures get ≤2 retries, then replan. The qualitative
  re-derivation stays with the Verifier role.
- **merge-approval:** human gate on diff + report + trace. Completes
  the workflow.

## Event routes (MVP)

- Verification failure on a step with `retries` → bounded retry, then
  `onFail: replan` (reset from failing step, checkpoint, audit entry).
- Denied human gate → halt with exit 2 until re-approved.
- Wall-clock budget exceeded → pause with exit 4.
