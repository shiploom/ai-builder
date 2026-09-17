# normalize.sed — applied to BOTH binaries' outputs before comparison.
# Masks the intended differences (runtime identity + versions) so the
# harness asserts everything else byte-identical.
s/^shiploom [^ ]+ \(core [^,]+, (python|go) [^)]+\)$/shiploom PKG (core C, runtime R)/
s/python [0-9][0-9.]*/python V/g
s/go[0-9][0-9a-z.]*/go V/g
