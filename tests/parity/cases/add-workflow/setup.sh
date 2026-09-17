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
