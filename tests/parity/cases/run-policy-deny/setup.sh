#!/bin/sh
set -u
. "$REPO/tests/parity/cases/_seed.sh"
seed_manifest denyflow
mkdir -p seed/.shiploom/workflows
cat > seed/.shiploom/workflows/denyflow.md <<'MDEOF'
---
name: denyflow
version: 1.0.0
kind: sequential
steps:
  - id: boom
    gate: policy
    action: db.destroy
    resource: prod
---

# deny flow
MDEOF
