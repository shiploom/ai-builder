"""Shared stdlib-only assertion helper for unit tests.

Not collected by pytest (no `test_` prefix). Importable because pytest
prepends `tests/unit/` to `sys.path` (no `__init__.py` chain).

Background: `sys.stdlib_module_names` exists only on 3.10+, so on 3.9 the
tests fall back to FALLBACK. A hand-maintained per-test fallback drifted
out of sync and failed CI's 3.9 leg — hence this single canonical set.
FALLBACK MUST cover every stdlib module imported by `cli/*.py` and
`validators/*.py`; extend it together with any new stdlib import there.
"""

import ast
import sys
from pathlib import Path

FALLBACK = frozenset({
    "argparse", "ast", "datetime", "fnmatch", "hashlib", "json", "os", "pathlib",
    "re", "shlex", "shutil", "stat", "subprocess", "sys", "tempfile", "time",
})


def stdlib_names():
    """Full stdlib set on 3.10+; canonical FALLBACK on 3.9."""
    return set(getattr(sys, "stdlib_module_names", ())) or set(FALLBACK)


def assert_stdlib_only(source_path, extra=()):
    """Assert a source file imports stdlib (+ `extra`) modules only."""
    tree = ast.parse(Path(source_path).read_text(encoding="utf-8"))
    imports = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            imports.update(a.name.split(".")[0] for a in node.names)
        elif isinstance(node, ast.ImportFrom) and node.module:
            imports.add(node.module.split(".")[0])
    assert imports - stdlib_names() - set(extra) == set(), (source_path, imports)
