#!/bin/sh
set -u
. "$REPO/tests/parity/cases/_seed.sh"
seed_manifest greenfield-full-lite
mkdir -p seed/acceptance seed/.shiploom/.oracle/oracle
cat > seed/acceptance/auth.json <<'EOF'
[{"id": "ACC-001", "statement": "Reset email arrives within 60 seconds for valid accounts", "howToVerify": {"type": "script", "command": "pytest t.py -q", "expect": "exit 0 under 60s"}, "oracleRef": "oracle/ACC-001.sh"}]
EOF
printf '#!/bin/sh\nexit 0\n' > seed/.shiploom/.oracle/oracle/ACC-001.sh
chmod 700 seed/.shiploom/.oracle
