# HRMS build playbook (greenfield, harness of your choice)

Build a complete HRMS — org chart, employees, leave, attendance, payroll
inputs, performance, recruitment, documents — with Shiploom orchestrating
and your coding-agent harness (Claude Code, OpenCode, or `auto`) doing
the LLM work. You drive; the orchestrator enforces order, inputs,
outputs, gates, budgets, and audit. Autonomy stops at human gates.

Prerequisites: `shiploom` on PATH (`npx -y @shiploom/cli`,
`brew install shiploom/tap/shiploom`, or `sh scripts/build-go.sh`),
`jq` installed, and a pytest-bearing `python3` first on PATH (operator
tooling for auto-detected test gates — not the implementation, which is
the Go binary). Node is optional (only the node leg needs it).

## 0. Scaffold the project

```bash
mkdir hrms && cd hrms
shiploom init --green --harness auto   # --harness claude|opencode to pin one
shiploom adapters --generate all       # harness files: skills, hooks, guard
```

`init` creates `idea.md`, `.shiploom/` (manifest, audit log, vault),
and starter config. `adapters --generate` installs the skill packs your
harness will execute (`market-research`, `product-definition`,
`architecture-design`, `spec-to-acceptance`, roles) plus the guard hook.

## 1. Write the idea (you, 30 min)

Replace your seeded `idea.md` with the block below (keep the
frontmatter shape — the stepper only needs the file to exist, but
valid frontmatter keeps every later validation green). It is written
to be testable end to end: every module has a measurable signal, every
hard rule is a hypothesis the acceptance step must lock.

```markdown
---
id: IDEA-201
kind: plan
title: "Complete HRMS for a 200-person services company, built from scratch"
status: proposed
provenance:
  - type: human
    ref: "idea.md:1"
    confidence: high
    date: 2026-09-19
links:
  requires: []
  decided_by: []
  implemented_by: []
  tested_by: []
  verified_by: []
owner: human
version: 1
---

# HRMS: complete system for people operations, built from scratch

## Problem
HR runs on spreadsheets + email. Leave balances are disputed monthly,
attendance is re-typed into payroll (errors every cycle), no one can
answer "who reports to whom" without asking around, offer letters are
assembled by hand, and auditors flagged missing hire/termination
trails last quarter. Nothing exists yet: greenfield, no legacy data
to migrate, no HR APIs to integrate.

## Users
- HR admins (2): own all data, run payroll end to end, answer disputes.
- Managers (~25): approve leave, view team attendance, run reviews.
- Employees (~175): request leave, view payslips, update profile,
  complete onboarding tasks.
- Recruiters (3): openings, pipeline, offers.
- Auditors (read-only): hire/termination/payroll trail per quarter.

## Scope — complete modules (one slice each)
1. Org chart: teams, reporting lines, manager of record per employee,
   effective-dated changes with full history.
2. Employees: profile, onboarding task lists, join/exit dates,
   employment state machine (candidate → active → on-notice →
   exited; no other transitions), rehire as a new record.
3. Leave: types (earned/sick/casual), monthly accrual, carry-forward
   caps, balances, request → approve/deny flow with audit events.
4. Attendance: daily present/absent/half-day, shifts + rosters,
   holiday lists, month locking (locked months read-only).
5. Payroll: salary structures, earnings/deductions, overtime, tax
   computation, payroll runs (draft → locked), payslips, bank advice
   file generation for disbursement.
6. Performance: review cycles, goals, ratings 1–5, calibration note
   per team, promotion recommendations.
7. Recruitment: openings, applicants, stage pipeline (applied →
   screen → interview → offer → hired/rejected), offer generation.
8. Documents: offer letters, experience letters, payslips, all
   generated from templates and versioned per employee per month.
9. Access control: roles (admin/manager/employee/recruiter/auditor),
   every data read/write checked against role + team scope.
10. Notifications: email queue for approvals, payslip releases,
    review deadlines (logged, retryable, no silent drops).
11. Reports: headcount, attrition, leave liability, payroll summary —
    every figure reproducible from locked source data.

## Hypotheses (must become locked acceptance)
- HYP-001: leave balances never go negative (request exceeding balance
  is rejected, not queued).
- HYP-002: payroll runs consume only locked attendance months;
  unlocked months are excluded with a warning, never silently.
- HYP-003: every hire, termination, approval, denial, payroll lock,
  and disbursement writes an audit event with actor + timestamp.
- HYP-004: an employee has exactly one manager of record at any date
  (overlaps rejected).
- HYP-005: exited employees are read-only everywhere except rehire,
  which creates a new employment record (history never rewritten).
- HYP-006: any destructive action (delete, bulk import, payroll lock,
  disbursement) requires a recorded human approval before execution.
- HYP-007: a locked payroll run is immutable; corrections require a
  supplementary run linked to the original (never an edit).
- HYP-008: tax is computed from versioned slabs effective-dated by
  the finance calendar; a run always cites the slab version used.
- HYP-009: no user sees data outside their role + team scope (deny by
  default; every denial logged).
- HYP-010: every report figure traces to locked source records; two
  runs over the same locks produce identical figures.

## Open questions
- Q-001: carry-forward cap for earned leave (10 days default?).
- Q-002: payroll bank advice format (which bank's layout first?).
- Q-003: tax slab source of truth — finance uploads per cycle? (Yes:
  versioned upload, effective-dated.)

## Constraints
- Runs offline-first on SQLite; external integrations are file
  import/export only in the first cut (no live third-party APIs).
- Every state-changing action emits an audit event (HYP-003).
- All dates ISO-8601; all money in integer minor units (no floats).

## Out of scope (non-goals, not later phases)
Biometric device drivers (attendance via file import only), government
e-filing live submission (generate the filings, submit externally),
third-party job-board integrations.
```


```

## 2. Run the stepper, answer its pauses

The loop for the whole build is one command, re-run after each input:

```bash
shiploom run greenfield-full-lite
```

It advances as far as mechanics allow, then pauses with a reason. Your
job at each pause is small and stated. The full journey:

| # | Pause message | You / harness do | Then |
|---|---|---|---|
| 1 | `step 'research' incomplete, missing outputs` | Harness drafts `research/market.md`, `research/evidence.md` (market-research skill; grade evidence Fact/Inferred/Opinion) | re-run → `advanced: research` |
| 2 | `awaiting approval: scope-approval` | Read the research, then `shiploom approve scope-approval` (or `--deny --reason "..."`) | re-run |
| 3 | `step 'product' incomplete` | Harness drafts `product/requirements.md` (product-definition; one `REQ-xxx` per HRMS module, each measurable) | re-run → advances `product`, `arch` needs `architecture/` |
| 4 | `step 'arch' incomplete` | Harness drafts `architecture/decisions.md` + `architecture/architecture.md` (one ADR per cross-module call) | re-run |
| 5 | `awaiting approval: arch-approval` | `shiploom approve arch-approval` | re-run |
| 6 | `step 'acceptance-lock' incomplete` | Harness drafts `acceptance/*.json` (one `ACC-xxx` per REQ, with `howToVerify` + `oracle` refs) | re-run → `acceptance not locked yet` |
| 7 | `acceptance not locked yet` | `shiploom lock` (hash-locks criteria + oracle implementations into `.shiploom/.oracle/`, mode `0700`) | re-run → advances lock |
| 8 | `verifier report twin missing` | Harness implements (`roles/implementer.md` → `implementation/attempt-log.md`), then verifies (`roles/verifier.md` → `verification/verification-report.{md,json}` twin) | re-run → `awaiting approval: merge-approval` |
| 9 | `awaiting approval: merge-approval` | `shiploom approve merge-approval` | re-run → `workflow greenfield-full-lite complete` |

Suggested HRMS module spine for `REQ-xxx` (one slice each, keep slices
independent): org-chart, employees, leave, attendance, payroll-inputs,
performance, recruitment, documents. Each needs `ACC-xxx` with a
machine-checkable `howToVerify` (script/http gate) or a named human
oracle — vague criteria fail `lock`.

## 3. Harness prompts that work

Give your harness one job per run-step, with file paths, not vibes:

- Research: "Using skills/market-research, compare 3 open-source HRMS
  (modules, auth model, leave engines). Write research/market.md +
  research/evidence.md; mark every claim Fact/Inferred/Opinion."
- Product: "Using skills/product-definition, turn idea.md + research/
  into product/requirements.md: REQ-001… per module above; each
  requirement states its acceptance signal."
- Architecture: "Using skills/architecture-design, write
  architecture/decisions.md (ADRs) + architecture/architecture.md:
  modules, data ownership (who owns leave balances?), call graph."
- Acceptance: "Using skills/spec-to-acceptance, write acceptance/*.json:
  ACC-xxx per REQ-xxx with howToVerify {type: script|http|human} and an
  oracle/ implementation for each."
- Implement: "Using roles/implementer.md, implement one module slice;
  log every attempt in implementation/attempt-log.md."
- Verify: "Using roles/verifier.md, independently verify against the
  locked acceptance; write the verification twin (report .md + .json)."

Never let the harness edit the manifest, audit log, or vault by hand —
done-state is re-verified every run, so hand edits get reopened, not
rewarded.

## 4. Operate it

```bash
shiploom status                    # artifact states + manifest position
shiploom approvals                 # pending human gates (add --watch to poll)
shiploom budget                    # token/spend/wall-clock limits + usage
shiploom run greenfield-full-lite --from <step>   # reset from a step onward
shiploom run greenfield-full-lite --only <step>   # advance one step
shiploom audit --export md        # reviewable audit trail
```

Kill-safe: `kill -9` mid-verify loses nothing (atomic manifest writes);
re-running never redoes verified steps. `verify` failures consume
retries, then force replan (`onFail: abort` ends the run with exit 2).

## 5. Troubleshooting

- `unknown uses ref` — skill/role path missing; re-run `adapters --generate all`.
- `missing inputs` — the listed `consumes` file doesn't exist yet; draft it.
- `outputs fail validation` — first error message names the file + rule; fix and re-run (done steps re-verify automatically).
- `step 'x': policy deny (...)` (exit 3) — destructive action (e.g. prod
  data writes); narrow the step's action/resource or record approval.
- `wall-clock budget exceeded` (exit 4) — raise with
  `run --budget wallClockH=<hours>` or `budget --set`.
- `replan required after N attempts` — verification kept failing; inspect
  the twin, fix the module, re-run from the failed step.
- `gate denied (re-approve to unblock)` — a human denied it; address the
  reason, then `approve <gate>` again.

## 6. Ship it

Copy a starter from `examples/deploy/` (docker/k8s/aws/preview),
write the per-target plan with the `deployment-plan` skill, keep a
human prod gate, and run `post-deploy-validate`. Export the audit log
for review before merge, every merge.
