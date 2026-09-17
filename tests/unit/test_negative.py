"""Adversarial tests: bypass attempts must block and audit (no network).

Covers MASTER_SPEC AC6 negatives: oracle reads by the builder, test
redefinition, hand-marked Done, stale locks, unapproved destructive
actions, secret leaks, and non-offline construction.
"""

import ast
import json
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))

from cli import gates as gates_mod  # noqa: E402
from cli import manifest as manifest_mod  # noqa: E402
from cli import oracle as oracle_mod  # noqa: E402
from cli.shiploom import main  # noqa: E402
from validators.validate import collect_files  # noqa: E402

REPO = Path(__file__).resolve().parents[2]

ACC = {"id": "ACC-001",
       "statement": "Reset email arrives within 60 seconds for valid accounts",
       "howToVerify": {"type": "script", "command": "pytest t.py -q",
                       "expect": "exit 0 under 60s"},
       "oracleRef": "oracle/ACC-001.sh"}

WF = """\
---
name: neg
version: 1.0.0
kind: sequential
resume: true
budgets:
  tokens: 100
  spendUSD: 1
  wallClockH: 1
steps:
  - id: draft
    consumes: [idea.md]
    produces: [out/note.md]
    gate: none
---

# Negative-test workflow
"""

NOTE = """\
---
id: RPT-090
kind: report
title: "Note"
status: proposed
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
owner: human
version: 1
---

Body.
"""


def write(path, content):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")
    return path


@pytest.fixture()
def proj(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    assert main(["init"]) == 0
    return tmp_path


def test_oracle_invisible_to_validators_and_discovery(proj):
    """Builder-side tooling never sees vault contents, even when malformed."""
    vault = proj / ".shiploom" / ".oracle"
    write(vault / "ACC-001.sh", "#!/bin/sh\nexit 0\n")
    write(vault / "notes.md", "---\nnot: [valid\n---\nbody\n")
    write(vault / "fake.json", "{nope")
    from validators.validate import validate_path
    errors, warnings, _ = validate_path(proj, strict=True)
    assert errors == [] and warnings == []
    assert oracle_mod.discover_acceptance(proj) == []
    assert all(".oracle" not in str(p) for p in collect_files(proj))


def test_hand_marked_done_is_reopened(proj, capsys):
    """A manifest hand-edited to done (without outputs) does not skip gates."""
    write(proj / ".shiploom" / "workflows" / "neg.md", WF)
    data = manifest_mod.load(proj)
    data["workflow"] = "neg"
    data["steps"] = {"draft": {"state": "done", "at": "2026-09-17T00:00:00Z"}}
    manifest_mod.save(proj, data)
    capsys.readouterr()
    assert main(["run", "neg"]) == 0
    assert "missing outputs" in capsys.readouterr().out
    data = manifest_mod.load(proj)
    assert data["steps"]["draft"]["state"] != "done"


def test_revoked_approval_reopens_step(proj, capsys):
    """Denying after approval re-blocks the workflow on the next run."""
    write(proj / ".shiploom" / "workflows" / "neg.md", WF.replace(
        "    gate: none", "    gate: human-approval", 1))
    write(proj / "out" / "note.md", NOTE)
    assert main(["run", "neg"]) == 0  # adopts neg, pauses at approval
    assert main(["approve", "draft"]) == 0
    capsys.readouterr()
    assert main(["run", "neg"]) == 0  # completes behind the approval
    assert main(["approve", "draft", "--deny", "--reason", "regression"]) == 0
    data = manifest_mod.load(proj)
    data["steps"] = {}  # fresh traversal, revoked gate
    manifest_mod.save(proj, data)
    assert main(["run", "neg"]) == 2


def test_stale_lock_breaks_later_verification(proj, capsys):
    """Tampering with the oracle after acceptance-lock poisons verify."""
    capsys.readouterr()
    write(proj / "acceptance" / "a.json", json.dumps([ACC]))
    vault = proj / ".shiploom" / ".oracle" / "oracle"
    vault.mkdir(parents=True, exist_ok=True)
    write(vault / "ACC-001.sh", "#!/bin/sh\nexit 0\n")
    assert main(["lock"]) == 0
    # A complete verify package (twin present) so the checker reaches the
    # lock-freshness comparison instead of stopping at twin-missing.
    write(proj / "verification" / "verification-report.json", json.dumps(
        {"id": "VR-001", "attemptRef": "diff:x", "verifier": "agent:verifier:fresh",
         "results": [{"acceptanceId": "ACC-001", "result": "pass",
                      "evidence": "oracle log", "oracleUsed": True}],
         "gatesSummary": {"test": "pass", "secrets": "pass"},
         "verdict": "pass", "timestamp": "2026-09-17T10:00:00Z"}))
    write(vault / "ACC-001.sh", "#!/bin/sh\nexit 1\n")
    ok, errors, _ = gates_mod.verify_step_check(proj)
    assert not ok and any("stale lock" in e["message"] for e in errors)


def test_destructive_action_denied_end_to_end(proj, capsys):
    """A policy-gated destructive action exits 3 with rule attribution."""
    write(proj / ".shiploom" / "workflows" / "neg.md", WF.replace(
        "  - id: draft\n    consumes: [idea.md]\n    produces: [out/note.md]\n    gate: none",
        "  - id: nuke\n    consumes: [idea.md]\n    produces: [idea.md]\n"
        "    gate: policy\n    action: db.destroy\n    resource: prod"))
    capsys.readouterr()
    assert main(["run", "neg"]) == 3
    assert "policy deny" in capsys.readouterr().out


def test_no_bypass_permissions_in_generated_files(proj, capsys):
    """Adapters must never emit permission-bypass escapes."""
    capsys.readouterr()
    assert main(["adapters", "--generate", "all"]) == 0
    for path in collect_files(proj):
        if ".shiploom" in path.parts:
            continue
        text = path.read_text(encoding="utf-8", errors="replace")
        assert "bypassPermissions" not in text, path


def test_secrets_gate_blocks_leak(proj, capsys):
    # NOTE: split token keeps this file scan-clean; runtime string identical.
    write(proj / "config_helper.py", 'api' + '_key = "abcdef1234567890"\n')
    capsys.readouterr()
    assert main(["verify", "--gates", "secrets"]) == 2


def test_audit_and_config_carry_no_secrets(proj, capsys):
    """Worked state (approvals, locks, audit) must be secret-free."""
    capsys.readouterr()
    write(proj / "acceptance" / "a.json", json.dumps([ACC]))
    vault = proj / ".shiploom" / ".oracle" / "oracle"
    vault.mkdir(parents=True, exist_ok=True)
    write(vault / "ACC-001.sh", "#!/bin/sh\nexit 0\n")
    assert main(["lock", "--actor", "human:priya"]) == 0
    findings = gates_mod.scan_secrets(proj / ".shiploom")
    assert findings == []
    findings = gates_mod.scan_secrets(proj)
    assert findings == []


def test_no_network_imports_in_core():
    """Offline construction: core code cannot import network clients."""
    denied = {"socket", "urllib", "http", "http.client", "ssl", "requests",
              "httpx", "urllib3", "websocket", "websockets", "aiohttp"}
    offenders = []
    for package in ("validators", "cli"):
        for path in (REPO / package).glob("*.py"):
            tree = ast.parse(path.read_text(encoding="utf-8"))
            for node in ast.walk(tree):
                names = []
                if isinstance(node, ast.Import):
                    names = [a.name.split(".")[0] for a in node.names]
                elif isinstance(node, ast.ImportFrom) and node.module:
                    names = [node.module.split(".")[0]]
                for name in names:
                    if name in denied:
                        offenders.append("%s imports %s" % (path, name))
    assert offenders == []
