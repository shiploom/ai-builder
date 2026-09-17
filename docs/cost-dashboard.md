# Cost / escape dashboard (contract)

Inputs are local files the orchestrator already writes; no vendor APIs.
`scripts/collect-dashboard.py [project]` emits this JSON (missing inputs
yield `null`, never guesses):

```json
{
  "workflow": "greenfield-full-lite",
  "steps": {"done": 9, "total": 9},
  "retries": {"implement": 0},
  "gates": {"scope-approval": "passed"},
  "budgets": {"tokens": {"limit": 800000, "used": 0}},
  "auditEvents": 42,
  "gateReport": {"verdict": "pass", "gates": {"test": "pass"}},
  "defectEscapes": null,
  "mergeStats": null,
  "verifierCatchRate": null
}
```

- `budgets.used` for tokens/spend is operator-reported until harness
  metering exists (wall-clock is enforced today).
- `defectEscapes`, `mergeStats`, `verifierCatchRate` unlock with pilot
  data: post-merge defect tracking and recorded verifier outcomes. The
  collector reports `null` with reasons rather than inventing numbers —
  enforceable cost/escape thresholds (and mutation floors per §33-D2) are
  set only from that data.
