#!/bin/sh
set -u
. "$REPO/tests/parity/cases/_seed.sh"
seed_manifest greenfield-full-lite
mkdir -p seed/.shiploom
cat > seed/.shiploom/config.json <<'EOF'
{"adapterTargets": ["auto"], "coreVersion": "1.1.0", "harness": "auto", "workflow": "greenfield-full-lite"}
EOF
printf "# seed idea\n" > seed/idea.md
