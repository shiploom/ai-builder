#!/bin/sh
set -u
. "$REPO/tests/parity/cases/_seed.sh"
seed_manifest greenfield-full-lite
seed_artifact RM-001 research/market.md
seed_artifact RM-002 research/evidence.md
python3 - <<'PYEOF'
import json
path = "seed/.shiploom/manifest.json"
data = json.load(open(path))
data["steps"] = {"research": {"state": "done", "at": "2026-01-01T00:00:00Z"}}
json.dump(data, open(path, "w"), indent=2, sort_keys=True)
PYEOF
