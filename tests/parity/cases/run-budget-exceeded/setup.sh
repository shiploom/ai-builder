#!/bin/sh
set -u
. "$REPO/tests/parity/cases/_seed.sh"
seed_manifest greenfield-full-lite
python3 - <<'PYEOF'
import json
path = "seed/.shiploom/manifest.json"
data = json.load(open(path))
data["startedAt"] = "2020-01-01T00:00:00Z"
data["budgets"]["wallClockH"] = {"limit": 1, "used": 0}
json.dump(data, open(path, "w"), indent=2, sort_keys=True)
PYEOF
