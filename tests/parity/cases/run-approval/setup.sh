#!/bin/sh
set -u
. "$REPO/tests/parity/cases/_seed.sh"
seed_manifest greenfield-full-lite
seed_artifact RM-001 research/market.md
seed_artifact RM-002 research/evidence.md
