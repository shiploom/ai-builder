"""Hermetic unit tests for validators/status.py (no network, stdlib only)."""

import ast
import json
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))

from validators.status import format_human, main, status_of  # noqa: E402

REPO = Path(__file__).resolve().parents[2]

ART_MD = """\
---
id: %s
kind: requirement
title: "T"
status: %s
provenance:
  - type: human
    ref: "x:1"
    confidence: high
    date: 2026-09-17
links:
  requires: []
  decided_by: []
  implemented_by: []
  tested_by: []
  verified_by: []
owner: specifier
version: 1
---

Body.
"""


def write(path, content):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")
    return path


def test_status_counts_and_version(tmp_path):
    write(tmp_path / "a.md", ART_MD % ("REQ-001", "approved"))
    write(tmp_path / "b.md", ART_MD % ("REQ-002", "proposed"))
    payload, error = status_of(tmp_path)
    assert error is None
    assert payload["coreVersion"] == (REPO / "core" / "VERSION").read_text().strip()
    assert payload["counts"]["total"] == 2
    assert payload["counts"]["byStatus"] == {"approved": 1, "proposed": 1}
    assert payload["trace"]["nodes"] == 2 and payload["trace"]["edges"] == 0
    assert payload["budgets"]["status"] == "unavailable"
    assert [a["id"] for a in payload["artifacts"]] == ["REQ-001", "REQ-002"]


def test_status_reports_invalid_without_crashing(tmp_path):
    write(tmp_path / "bad.md", "---\nid: [unclosed\n---\nbody\n")
    payload, error = status_of(tmp_path)
    assert error is None
    assert payload["counts"]["total"] == 0 and len(payload["invalid"]) == 1


def test_status_missing_path(tmp_path):
    payload, error = status_of(tmp_path / "nope")
    assert payload is None and "no such" in error


def test_human_output_lists_artifacts(tmp_path):
    write(tmp_path / "a.md", ART_MD % ("REQ-001", "approved"))
    payload, _ = status_of(tmp_path)
    text = format_human(payload)
    assert "artifacts: 1" in text and "REQ-001" in text and "approved" in text


def test_cli_json_and_human(tmp_path, capsys):
    write(tmp_path / "a.md", ART_MD % ("REQ-001", "approved"))
    assert main([str(tmp_path), "--json"]) == 0
    payload = json.loads(capsys.readouterr().out)
    assert payload["ok"] is True and payload["counts"]["total"] == 1
    assert main([str(tmp_path)]) == 0
    assert "REQ-001" in capsys.readouterr().out
    assert main([str(tmp_path / "nope")]) == 2


def test_cli_subprocess(tmp_path):
    write(tmp_path / "a.md", ART_MD % ("REQ-001", "approved"))
    proc = subprocess.run([sys.executable, str(REPO / "validators" / "status.py"),
                           str(tmp_path), "--json"], capture_output=True, text=True)
    assert proc.returncode == 0 and json.loads(proc.stdout)["counts"]["total"] == 1


def test_status_stdlib_only():
    tree = ast.parse((REPO / "validators" / "status.py").read_text(encoding="utf-8"))
    imports = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            imports.update(a.name.split(".")[0] for a in node.names)
        elif isinstance(node, ast.ImportFrom) and node.module:
            imports.add(node.module.split(".")[0])
    stdlib = set(getattr(sys, "stdlib_module_names", ())) or {
        "argparse", "json", "sys", "pathlib", "validators"}
    assert imports - stdlib - {"validators"} == set()
