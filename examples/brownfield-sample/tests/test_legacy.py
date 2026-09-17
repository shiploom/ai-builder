"""Characterization tests: pin CURRENT behavior before any fix.

These tests describe what the code does today (including DEBT-001),
not what it should do. The fix must update them deliberately.
"""

from legacy import count_users, list_users


def test_count():
    assert count_users() == 4


def test_page1_current_behavior():
    # DEBT-001: page 1 skips 'ada' today.
    assert list_users(1) == ["linus", "margaret"]


def test_page2_current_behavior():
    assert list_users(2) == []
