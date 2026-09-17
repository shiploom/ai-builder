#!/bin/sh
set -u
. "$REPO/tests/parity/cases/_seed.sh"
seed_manifest greenfield-full-lite
# Generate an audit log via a writer command (timestamps/hashes land in
# seed once, then pristine-copied per runtime).
(cd seed && python3 "$REPO/cli/shiploom.py" budget --set tokens=100 >/dev/null 2>&1)
