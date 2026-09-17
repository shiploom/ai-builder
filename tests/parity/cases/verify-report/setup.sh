#!/bin/sh
set -u
. "$REPO/tests/parity/cases/_seed.sh"
seed_manifest greenfield-full-lite
printf '# seed idea\n' > seed/idea.md
