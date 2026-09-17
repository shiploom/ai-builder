#!/bin/sh
set -u
. "$REPO/tests/parity/cases/_seed.sh"
seed_manifest greenfield-full-lite
mkdir -p seed/.shiploom
cat > seed/.shiploom/config.json <<'EOF'
{"adapterTargets": ["auto"], "coreVersion": "1.0.0", "harness": "auto", "workflow": "greenfield-full-lite"}
EOF
python3 - <<'PYEOF'
import json
path = "seed/.shiploom/manifest.json"
data = json.load(open(path))
data["coreVersion"] = "1.0.0"
json.dump(data, open(path, "w"), indent=2, sort_keys=True)
PYEOF
printf "# seed idea\n" > seed/idea.md
(cd seed && python3 "$REPO/cli/shiploom.py" upgrade >/dev/null 2>&1)
