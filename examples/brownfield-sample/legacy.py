"""Legacy user listing (AC2 demo fixture).

DEBT-001: pagination is 1-based for callers but 0-based internally;
page 1 currently skips the first row (off-by-one). Characterization
tests in tests/test_legacy.py pin current behavior before any fix.
"""

USERS = ["ada", "grace", "linus", "margaret"]


def list_users(page, per_page=2):
    """Return rows for 1-based page. DEBT-001: drops row 0 on page 1."""
    start = page * per_page  # BUG: should be (page - 1) * per_page
    return USERS[start:start + per_page]


def count_users():
    return len(USERS)
