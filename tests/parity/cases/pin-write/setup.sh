#!/bin/sh
set -u
. "$REPO/tests/parity/cases/_seed.sh"
seed_manifest greenfield-full-lite
printf '{"capabilities": {}}\n' > seed/.shiploom/mcp-registry.json
