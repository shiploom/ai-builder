"""Password-reset helpers (AC demo fixture)."""

import secrets
import time

TOKEN_TTL_S = 15 * 60


def make_token():
    """Return a 128-bit hex token."""
    return secrets.token_hex(16)


def is_expired(issued_at, now=None):
    """True when the token is older than TOKEN_TTL_S."""
    now = time.time() if now is None else now
    return (now - issued_at) > TOKEN_TTL_S
