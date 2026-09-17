#!/bin/sh
set -u
. "$REPO/tests/parity/cases/_seed.sh"
seed_manifest greenfield-full-lite
mkdir -p seed/.shiploom/.oracle
chmod 700 seed/.shiploom/.oracle
