#!/bin/sh
set -u
. "$REPO/tests/parity/cases/_seed.sh"
seed_manifest greenfield-full-lite
(cd seed && python3 "$REPO/cli/shiploom.py" characterize --capture snap --command true >/dev/null 2>&1)
