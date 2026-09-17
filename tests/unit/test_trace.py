"""Hermetic unit tests for validators/trace.py (no network, stdlib only)."""

import ast
import json
import subprocess
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))

from validators.trace import build_trace, main  # noqa: E402
from validators.validate import load_schema, validate_against_schema  # noqa: E402

REPO = Path(__file__).resolve().parents[2]

REQ_MD = """\
---
id: REQ-001
kind: requirement
title: "Reset"
status: approved
provenance:
  - type: human
    ref: "idea.md:1"
    confidence: high
    date: 2026-09-17
links:
  requires: [IDEA-001]
  decided_by: [ADR-001]
  implemented_by: []
  tested_by: [ACC-001]
  verified_by: []
owner: specifier
version: 1
---

Body.
"""

IDEA_MD = REQ_MD.replace("id: REQ-001", "id: IDEA-001").replace(
    "kind: requirement", "kind: plan").replace(
    "  requires: [IDEA-001]\n  decided_by: [ADR-001]\n", "  requires: []\n  decided_by: []\n").replace(
    "  tested_by: [ACC-001]\n", "  tested_by: []\n").replace("owner: specifier", "owner: human")

ADR_MD = REQ_MD.replace("id: REQ-001", "id: ADR-001").replace(
    "kind: requirement", "kind: decision").replace(
    "  requires: [IDEA-001]\n  decided_by: [ADR-001]\n", "  requires: [REQ-001]\n  decided_by: []\n").replace(
    "  tested_by: [ACC-001]\n", "  tested_by: []\n")

ACC_JSON = json.dumps({
    "id": "ACC-001",
    "statement": "Reset email arrives within 60 seconds for valid accounts",
    "howToVerify": {"type": "script", "command": "pytest t.py -q",
                    "expect": "exit 0 under 60s"},
    "oracleRef": "oracle/ACC-001.sh",
})


def write(path, content):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")
    return path


@pytest.fixture()
def chain(tmp_path):
    write(tmp_path / "idea.md", IDEA_MD)
    write(tmp_path / "req.md", REQ_MD)
    write(tmp_path / "adr.md", ADR_MD)
    write(tmp_path / "acceptance" / "a.json", ACC_JSON)
    return tmp_path


def test_build_trace_chain(chain):
    trace, errors, warnings = build_trace(chain)
    assert errors == [] and warnings == []
    assert trace["REQ-001"]["requires"] == ["IDEA-001"]
    assert trace["REQ-001"]["decided_by"] == ["ADR-001"]
    assert trace["REQ-001"]["tested_by"] == ["ACC-001"]
    assert trace["ADR-001"]["requires"] == ["REQ-001"]
    assert set(trace) == {"IDEA-001", "REQ-001", "ADR-001", "ACC-001"}


def test_generated_trace_conforms_to_schema(chain):
    trace, errors, _ = build_trace(chain)
    assert errors == []
    assert validate_against_schema(trace, load_schema("trace"), "$") == []


def test_duplicate_id_errors(tmp_path):
    write(tmp_path / "a.md", REQ_MD)
    write(tmp_path / "b.md", REQ_MD)
    _, errors, _ = build_trace(tmp_path)
    assert any("duplicate artifact id" in e["message"] for e in errors)


def test_dangling_warns_then_strict_errors(tmp_path):
    write(tmp_path / "req.md", REQ_MD)  # IDEA-001/ADR-001/ACC-001 absent
    _, errors, warnings = build_trace(tmp_path)
    assert errors == [] and any("dangling" in w["message"] for w in warnings)
    _, errors, _ = build_trace(tmp_path, strict=True)
    assert any("dangling" in e["message"] for e in errors)


def test_empty_dir_ok(tmp_path):
    trace, errors, warnings = build_trace(tmp_path)
    assert (trace, errors, warnings) == ({}, [], [])


def test_out_flag_writes_conforming_file(chain, tmp_path):
    out = tmp_path / "out" / "trace.json"
    assert main([str(chain), "--out", str(out)]) == 0
    doc = json.loads(out.read_text(encoding="utf-8"))
    assert validate_against_schema(doc, load_schema("trace"), "$") == []
    assert doc["REQ-001"]["requires"] == ["IDEA-001"]


def test_cli_json_shape_and_missing_path(chain, tmp_path, capsys):
    assert main([str(chain)]) == 0
    payload = json.loads(capsys.readouterr().out)
    assert payload["ok"] is True and payload["trace"]["REQ-001"]["requires"] == ["IDEA-001"]
    assert main([str(tmp_path / "nope")]) == 2
    payload = json.loads(capsys.readouterr().out)
    assert payload["ok"] is False


def test_cli_subprocess_exit_codes(chain, tmp_path):
    proc = subprocess.run([sys.executable, str(REPO / "validators" / "trace.py"), str(chain)],
                          capture_output=True, text=True)
    assert proc.returncode == 0 and json.loads(proc.stdout)["ok"] is True
    lone = tmp_path / "lone"
    lone.mkdir()
    write(lone / "req.md", REQ_MD)
    proc = subprocess.run(
        [sys.executable, str(REPO / "validators" / "trace.py"), "--strict", str(lone)],
        capture_output=True, text=True)
    assert proc.returncode == 2


def test_trace_stdlib_only():
    tree = ast.parse((REPO / "validators" / "trace.py").read_text(encoding="utf-8"))
    imports = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            imports.update(a.name.split(".")[0] for a in node.names)
        elif isinstance(node, ast.ImportFrom) and node.module:
            mods = node.module.split(".")
            imports.add(mods[0])
    stdlib = set(getattr(sys, "stdlib_module_names", ())) or {
        "argparse", "json", "sys", "pathlib", "validators"}
    assert imports - stdlib - {"validators"} == set()
