#!/bin/sh
set -u
. "$REPO/tests/parity/cases/_seed.sh"
seed_manifest greenfield-full-lite
mkdir -p seed/.shiploom/.oracle
CORE_VERSION="$(cat "$REPO/core/VERSION")"
cat > seed/.shiploom/config.json <<EOF
{"adapterTargets": ["auto"], "budgets": {"spendUSD": 25.0, "tokens": 800000, "wallClockH": 8.0}, "coreVersion": "$CORE_VERSION", "harness": "auto", "mcpRegistry": "./.shiploom/mcp-registry.json", "policyPack": "default", "stack": null, "workflow": "greenfield-full-lite"}
EOF
printf '{"capabilities": {}}\n' > seed/.shiploom/mcp-registry.json
chmod 700 seed/.shiploom/.oracle
# Generate an audit log so project-audit passes deterministically.
(cd seed && python3 "$REPO/cli/shiploom.py" budget --set tokens=100 >/dev/null 2>&1)
