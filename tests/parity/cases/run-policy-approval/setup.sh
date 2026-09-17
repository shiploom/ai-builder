#!/bin/sh
set -u
. "$REPO/tests/parity/cases/_seed.sh"
seed_manifest deployflow
mkdir -p seed/.shiploom/workflows
cat > seed/.shiploom/workflows/deployflow.md <<'MDEOF'
---
name: deployflow
version: 1.0.0
kind: sequential
steps:
  - id: ship
    gate: policy
    action: deploy.prod
    resource: prod
---

# deploy flow
MDEOF
