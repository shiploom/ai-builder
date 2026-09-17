---
name: brownfield-fix
version: 1.0.0
kind: sequential
resume: true
budgets:
  tokens: 400000
  spendUSD: 12
  wallClockH: 4
steps:
  - id: map
    uses: skills/brownfield-map
    consumes: []
    produces: [brownfield/repo-map.md]
    gate: none
  - id: map-approval
    gate: human-approval
    onDeny: pause
  - id: impact
    uses: skills/impact-analysis
    consumes: [brownfield/repo-map.md]
    produces: [brownfield/impact-plan.md]
    gate: none
  - id: plan-approval
    gate: human-approval
    onDeny: pause
  - id: implement
    uses: roles/implementer.md
    consumes: [brownfield/impact-plan.md]
    produces: [implementation/attempt-log.md]
    retries: 4
    onFail: replan
  - id: regression-verify
    uses: roles/verifier.md
    consumes: [implementation/attempt-log.md]
    produces: [verification/verification-report.md, verification/verification-report.json]
    gate: verification
    retries: 4
    onFail: replan
  - id: merge-approval
    gate: human-approval
    onDeny: pause
---

# Brownfield Fix (skeleton)

> Scoped-change path for existing repos: map → approve → impact → approve
> → implement → regression-verify → merge. Characterization tests are
> mandatory before edits where regression coverage is thin (see the
> impact-plan template). Worked with the `brownfield-map` and
> `impact-analysis` skill packs (shipped). "Skeleton" marks hardening
> depth, not function: differential sampling and the hardened flow
> arrive post-MVP; order, approvals, and artifacts are enforced now.
