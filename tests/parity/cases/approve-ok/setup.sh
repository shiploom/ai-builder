#!/bin/sh
set -u
. "$REPO/tests/parity/cases/_seed.sh"
seed_manifest gateflow
mkdir -p seed/.shiploom/workflows
cat > seed/.shiploom/workflows/gateflow.md <<'MDEOF'
---
name: gateflow
version: 1.0.0
kind: sequential
steps:
  - id: ship
    gate: policy
    action: deploy.prod
    resource: prod
  - id: boss
    gate: human-approval
---

# gate flow
MDEOF
