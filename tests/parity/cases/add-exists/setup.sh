#!/bin/sh
set -u
. "$REPO/tests/parity/cases/_seed.sh"
seed_manifest greenfield-full-lite
mkdir -p seed/pack
cat > seed/pack/demo.md <<'MDEOF'
---
name: demo
version: 1.0.0
kind: sequential
steps:
  - id: build
    produces: [out.txt]
    gate: none
---

# demo
MDEOF
# Pre-install so the replayed add hits the exists guard.
(cd seed && python3 "$REPO/cli/shiploom.py" add workflow demo --from pack/demo.md >/dev/null 2>&1)
