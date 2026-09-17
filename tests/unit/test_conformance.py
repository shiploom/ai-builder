"""Hermetic unit tests for cli/conformance.py (no network, no LLM calls)."""

import ast
import json
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))

from cli import conformance as conformance_mod  # noqa: E402
from cli.shiploom import main  # noqa: E402

REPO = Path(__file__).resolve().parents[2]


def test_all_harnesses_pass():
    ok, results = conformance_mod.run_all()
    assert ok, results
    assert set(results) == {"base", "claude", "opencode"}
    for report in results.values():
        assert report["failures"] == 0


def test_unknown_harness_fails():
    ok, report = conformance_mod.check_harness("cursor")
    assert not ok and any("unknown harness" in e for e in report["errors"])


def test_record_writes_reports(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    ok, _ = conformance_mod.run_all(record=True)
    assert ok
    records = REPO / "tests" / "conformance" / "_records"
    assert {p.stem for p in records.glob("*.json")} == {"base", "claude", "opencode"}
    for path in records.glob("*.json"):
        assert json.loads(path.read_text(encoding="utf-8"))["ok"] is True
    for path in records.glob("*.json"):
        path.unlink()


def test_tampered_tree_fails():
    from cli import adapters as adapters_mod
    import tempfile
    with tempfile.TemporaryDirectory(prefix="shiploom-conf-neg-") as tmp:
        report, errors = adapters_mod.generate("claude", tmp)
        assert errors == []
        (Path(tmp) / ".claude" / "settings.json").unlink()
        checks = conformance_mod._check_tree("claude", Path(tmp))
        assert any(c["status"] == "fail" for c in checks)


def test_conformance_cli(proj_tmp, capsys):
    assert main(["conformance", "--harness", "base"]) == 0
    assert "base: PASS" in capsys.readouterr().out
    assert main(["conformance", "--harness", "bogus"]) == 2
    capsys.readouterr()
    assert main(["conformance", "--harness", "claude", "--json"]) == 0
    payload = json.loads(capsys.readouterr().out)
    assert payload["ok"] is True and payload["harness"] == "claude"


@pytest.fixture()
def proj_tmp(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    return tmp_path


def test_conformance_module_stdlib_only():
    tree = ast.parse((REPO / "cli" / "conformance.py").read_text(encoding="utf-8"))
    imports = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            imports.update(a.name.split(".")[0] for a in node.names)
        elif isinstance(node, ast.ImportFrom) and node.module:
            imports.add(node.module.split(".")[0])
    stdlib = set(getattr(sys, "stdlib_module_names", ())) or {
        "json", "subprocess", "sys", "tempfile", "pathlib", "cli", "validators"}
    assert imports - stdlib - {"cli", "validators"} == set(), imports
