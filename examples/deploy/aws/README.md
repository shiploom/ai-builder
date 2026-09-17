# deploy/aws — cloud target stub (starter)

- `main.tf`: provider pin, S3 backend placeholder (configured per env),
  sensitive variables only. Capability blocks bind here, never in core.
- Workflow: `terraform validate` → `plan` (both read-only) → cost
  estimate artifact → human spend/prod approval → `apply`.
- Least-privilege IAM assumed; state locking on.
