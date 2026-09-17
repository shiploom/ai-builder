#!/bin/sh
set -u
printf '# seed idea\n' > seed-idea.tmp
mkdir -p seed
mv seed-idea.tmp seed/idea.md
