# team-policy-pack — example team overlay (template, not certification)

A stricter starting point than `core/policies/default.json`, showing the
three moves teams actually make:

1. **Name the denial** — `db.migrate` denied in *all* envs by an explicit,
   messaged rule. The default pack reaches the same verdict for non-prod
   only via silent default-deny; the team version states intent where the
   audit trail can quote it.
2. **Condition on actor** — agent-driven deploys need approval; human deploys
   pass. Precedence is deny > require-approval > allow, with conditional
   rules winning ties, so both rules coexist safely.
3. **Own your allow-list** — every `gate: policy` action your workflows
   declare must match an allow rule or a scoped approval, or default-deny
   blocks it. When `run` pauses on an unexpected deny, the rule id in the
   message tells you exactly which line to review.

## Install

```bash
shiploom add policy team --from examples/team-policy-pack
# wires to ./.shiploom/policies/team.json (unsigned provenance, validated)
```

## Wire a step to it

```yaml
- id: ship-staging
  gate: policy
  policy: .shiploom/policies/team.json
  action: deploy.staging
  resource: staging
```

## Graduate to enterprise (demand-gated)

SSO/RBAC principals, audit export to SIEM, attestation-required MCP, and
threshold signatures arrive with an enterprise pilot — this template is the
policy half, ready to extend.
