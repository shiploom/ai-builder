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
python3 - <<'PYEOF'
import json
path = "seed/.shiploom/manifest.json"
data = json.load(open(path))
data["gates"] = {"ship": {"state": "denied"}}
data["retries"] = {"ship": 2}
json.dump(data, open(path, "w"), indent=2, sort_keys=True)
PYEOF
