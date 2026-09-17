#!/bin/sh
set -u
. "$REPO/tests/parity/cases/_seed.sh"
seed_manifest greenfield-full-lite
python3 - <<'PYEOF'
import json
path = "seed/.shiploom/manifest.json"
data = json.load(open(path))
data["gates"] = {"scope-approval": {"state": "passed"},
                 "arch-approval": {"state": "passed"},
                 "merge-approval": {"state": "passed"}}
json.dump(data, open(path, "w"), indent=2, sort_keys=True)
PYEOF
