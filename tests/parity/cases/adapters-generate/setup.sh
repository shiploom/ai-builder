#!/bin/sh
set -u
. "$REPO/tests/parity/cases/_seed.sh"
seed_manifest greenfield-full-lite
mkdir -p seed/.shiploom
cat > seed/.shiploom/manifest.json <<'EOF'
{"artifacts": {}, "budgets": {"spendUSD": {"limit": 25.0, "used": 0.0}, "tokens": {"limit": 800000, "used": 0}}, "checkpoints": [], "coreVersion": "1.1.0", "gates": {}, "initializedAt": "2026-01-01T00:00:00Z", "retries": {}, "steps": {}, "workflow": "greenfield-full-lite", "workflowVersion": "1.0.0"}
EOF
