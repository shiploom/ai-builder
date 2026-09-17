"""AC demo fixture tests (fast, hermetic, no network)."""

from app import TOKEN_TTL_S, is_expired, make_token


def test_token_has_128_bits():
    assert len(make_token()) == 32


def test_tokens_differ():
    assert make_token() != make_token()


def test_fresh_token_valid():
    assert is_expired(1000.0, now=1000.0 + TOKEN_TTL_S - 1) is False


def test_old_token_rejected():
    assert is_expired(1000.0, now=1000.0 + TOKEN_TTL_S + 1) is True
