cat > bad.md <<'MDEOF'
---
id: REQ-001
kind: nonsense
title: "T"
status: proposed
provenance:
  - type: human
    ref: "x:1"
    confidence: high
    date: 2026-09-17
links:
  requires: []
  decided_by: []
  implemented_by: []
  tested_by: []
  verified_by: []
owner: specifier
version: 1
---

Body.
MDEOF
