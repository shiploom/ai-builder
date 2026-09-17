# Policy cookbook

The default pack (`core/policies/default.json`) denies destructive actions
(`infra.destroy`, `db.destroy`, `db.migrate-prod`, `secret.exfiltrate`),
requires approval for deploys, protected merges, spend, and provisioning,
and allows read-local work. Default effect is deny.

## Writing a pack

Packs follow `schemas/policy.schema.json`: `{policyId, version,
defaultEffect, rules[]}` where each rule has `{id, effect, actions[],
resources[], condition?, message?}`. Actions/resources are `*` globs;
`condition` maps context keys (`actor`, `workflow`, `step`) to required
values. Precedence: deny beats require-approval beats allow; conditional
rules win ties at the same effect.

## Wiring a policy gate

```yaml
- id: ship-prod
  gate: policy
  action: deploy.prod
  resource: prod
  produces: [idea.md]
```

`shiploom run` evaluates the step: `deny` exits 3 with rule attribution
(audit `run.policy.deny`); `require-approval` pauses until
`shiploom approve ship-prod`; `allow` proceeds. Omit `policy:` to use the
default pack, or point it at a project pack (e.g. `policies/team.json`).

## Approving

```bash
shiploom approvals              # what is pending, with attempts + reasons
shiploom approve ship-prod --reason "..."
shiploom approve ship-prod --deny --reason "..."   # reason mandatory
```

Denied gates halt `run` with exit 2 until re-approved. Every decision lands
in the hash-chained audit log.

## Starting from the team template

`examples/team-policy-pack/` is a stricter overlay showing the three moves
teams actually make: name the denial (`db.migrate` in all envs, with an
auditable message instead of silent default-deny), condition on actor
(agent deploys need approval, human deploys pass), and own your allow-list
(every `gate: policy` action your workflows declare must match, or the
pack blocks it and names the rule to review).

```bash
shiploom add policy team --from examples/team-policy-pack
```

then point steps at it with `policy: .shiploom/policies/team.json`.
Graduating to enterprise (SSO/RBAC, audit export, attestation-required MCP)
still waits on a pilot — the template is the policy half, ready to extend.
