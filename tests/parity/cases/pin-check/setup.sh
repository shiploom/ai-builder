#!/bin/sh
set -u
. "$REPO/tests/parity/cases/_seed.sh"
seed_manifest greenfield-full-lite
printf '{"capabilities": {}}\n' > seed/.shiploom/mcp-registry.json
# Pre-pin the seed so `pin --check` replays against the same file.
(cd seed && python3 "$REPO/cli/shiploom.py" pin >/dev/null 2>&1)
