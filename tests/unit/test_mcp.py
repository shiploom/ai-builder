"""Hermetic unit tests for cli/mcp.py (no network, stdlib only)."""

import ast
import json
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))
from _stdlib import assert_stdlib_only

from cli import mcp as mcp_mod  # noqa: E402
from cli.shiploom import main  # noqa: E402

REPO = Path(__file__).resolve().parents[2]

REGISTRY = {
    "capabilities": {
        "cap.web.search": {"providers": ["tavily"], "trust": "community",
                           "ttlS": 3600, "fallback": "fetch"},
        "cap.repo.symbols": {"providers": ["local-lsp"], "trust": "first-party", "ttlS": 30},
    },
    "servers": {
        "tavily": {"transport": "http", "version": "1.2.0", "attested": False,
                   "scopes": ["search:read"], "auth": "env:TAVILY_KEY"},
        "fetch": {"transport": "stdio", "version": "1.0.0", "attested": True,
                  "scopes": ["fetch:read"], "auth": "env:ALLOWLIST_ONLY"},
        "local-lsp": {"transport": "stdio", "version": "1.0.0", "attested": True,
                      "scopes": ["symbols:read"], "auth": "env:EMPTY"},
    },
}


def test_resolve_orders_chain_and_flags_quarantine():
    chain = mcp_mod.resolve(REGISTRY, "cap.web.search")
    assert chain["providers"] == ["tavily"] and chain["fallback"] == "fetch"
    assert chain["ttlS"] == 3600
    assert chain["quarantine"] is True  # tavily unattested
    assert chain["servers"]["tavily"]["auth"] == "env:TAVILY_KEY"


def test_resolve_attested_no_quarantine():
    chain = mcp_mod.resolve(REGISTRY, "cap.repo.symbols")
    assert chain["quarantine"] is False and chain["fallback"] is None


def test_resolve_unknown_returns_none():
    assert mcp_mod.resolve(REGISTRY, "cap.nope") is None
    assert mcp_mod.resolve({}, "cap.web.search") is None


def test_load_registry_roundtrip(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    assert main(["init"]) == 0
    doc = mcp_mod.load_registry(tmp_path)
    assert doc == {"capabilities": {}}
    with pytest.raises(ValueError):
        mcp_mod.load_registry(tmp_path / "missing")


def test_fixture_registry_validates_and_resolves():
    path = REPO / "examples" / "greenfield-starter" / ".shiploom" / "mcp-registry.json"
    doc = json.loads(path.read_text(encoding="utf-8"))
    from validators.validate import load_schema, validate_against_schema
    assert validate_against_schema(doc, load_schema("mcp-registry"), "$") == []
    chain = mcp_mod.resolve(doc, "cap.web.search")
    assert chain["providers"] == ["tavily"] and chain["quarantine"] is True


def test_doctor_reports_capability_count(tmp_path, monkeypatch, capsys):
    monkeypatch.chdir(tmp_path)
    assert main(["init"]) == 0
    capsys.readouterr()
    assert main(["doctor", "--json"]) == 0
    report = json.loads(capsys.readouterr().out)
    mcp = next(c for c in report["checks"] if c["name"] == "project-mcp-registry")
    assert mcp["status"] == "pass" and "0 capabilities" in mcp["detail"]


def test_doctor_warns_on_unprovenanced_servers(tmp_path, monkeypatch, capsys):
    monkeypatch.chdir(tmp_path)
    assert main(["init"]) == 0
    (tmp_path / ".shiploom" / "mcp-registry.json").write_text(
        json.dumps(REGISTRY), encoding="utf-8")
    capsys.readouterr()
    assert main(["doctor", "--json"]) == 0  # warn, never fail
    report = json.loads(capsys.readouterr().out)
    assert report["ok"] is True
    mcp = next(c for c in report["checks"] if c["name"] == "project-mcp-registry")
    assert mcp["status"] == "warn"
    assert "tavily" in mcp["detail"] and "local-lsp" in mcp["detail"]


def _attested_registry():
    doc = json.loads(json.dumps(REGISTRY))
    doc["servers"]["fetch"]["attestation"] = {
        "method": "pinned-digest", "digest": "sha256:abc",
        "verifiedAt": "2026-09-17T00:00:00Z"}
    return doc


def test_attestation_block_validates_and_rejects():
    from validators.validate import load_schema, validate_against_schema
    schema = load_schema("mcp-registry")
    assert validate_against_schema(_attested_registry(), schema, "$") == []
    bad = _attested_registry()
    bad["servers"]["fetch"]["attestation"] = {"method": "handshake"}
    assert validate_against_schema(bad, schema, "$") != []
    legacy = {"capabilities": {"cap.x": {"providers": ["s"], "ttlS": 1}},
              "servers": {"s": {"transport": "stdio", "version": "1",
                                "attested": False, "scopes": [], "auth": "env:X"}}}
    assert validate_against_schema(legacy, schema, "$") == []  # backward compat


def test_resolve_reports_per_provider_attestation():
    chain = mcp_mod.resolve(_attested_registry(), "cap.web.search")
    assert chain["attestation"] == {"tavily": "unattested"}
    assert mcp_mod.resolve(REGISTRY, "cap.repo.symbols")["attestation"] == {
        "local-lsp": "unverified"}  # claim without evidence


def test_attestation_status_summary():
    summary = mcp_mod.attestation_status(_attested_registry())
    assert summary["servers"] == {"tavily": "unattested", "fetch": "attested",
                                  "local-lsp": "unverified"}
    assert summary["unverifiedAttested"] == ["local-lsp"]
    assert summary["counts"] == {"unattested": 1, "attested": 1, "unverified": 1}
    assert mcp_mod.attestation_status({})["counts"] == {}
    assert mcp_mod.attestation_status(None)["counts"] == {}
    assert mcp_mod.attestation_status({"servers": "nope"})["counts"] == {}


def test_mcp_module_stdlib_only():
    assert_stdlib_only(REPO / "cli" / "mcp.py")
