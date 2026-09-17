"""Hermetic unit tests for trace/budget/resume/approvals/add (no network)."""

import ast
import json
import subprocess
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))

from cli import approvals as approvals_mod  # noqa: E402
from cli.shiploom import main  # noqa: E402

REPO = Path(__file__).resolve().parents[2]

ART_MD = """\
---
id: REQ-001
kind: requirement
title: "T"
status: approved
provenance:
  - type: human
    ref: "x:1"
    confidence: high
    date: 2026-09-17
links:
  requires: [IDEA-001]
  decided_by: [ADR-001]
  implemented_by: []
  tested_by: []
  verified_by: []
owner: specifier
version: 1
---

Body.
"""

SKILL_SRC = """\
---
name: demo-skill
description: A demo skill pack for add-command tests.
license: MIT
compatibility: base-spec
metadata:
  domain: test
  version: "1.0.0"
---

# Demo Skill

## Purpose

Test the add command.

## Inputs

None.

## Outputs

None.

## Prerequisites

None.

## Methodology

1. Do nothing.

## Constraints

None.

## Tools / MCP

None.

## Verification

Validate this file.

## Examples

None.
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


def test_trace_known_and_unknown(proj, capsys):
    write(proj / "req.md", ART_MD)
    write(proj / "idea.md", ART_MD.replace("id: REQ-001", "id: IDEA-001")
          .replace("kind: requirement", "kind: plan")
          .replace("  requires: [IDEA-001]\n  decided_by: [ADR-001]\n",
                    "  requires: []\n  decided_by: []\n")
          .replace("owner: specifier", "owner: human"))
    capsys.readouterr()
    assert main(["trace", "REQ-001", str(proj)]) == 0
    out = capsys.readouterr().out
    assert "requires: IDEA-001" in out and "referenced by:" in out
    assert main(["trace", "NOPE-001", str(proj)]) == 2
    assert main(["trace", "REQ-001", str(proj), "--json"]) == 0
    payload = json.loads(capsys.readouterr().out)
    assert payload["links"]["requires"] == ["IDEA-001"]


def test_budget_show_set_and_bad_key(proj, capsys):
    capsys.readouterr()
    assert main(["budget", str(proj)]) == 0
    assert "tokens: 0/800000" in capsys.readouterr().out
    assert main(["budget", str(proj), "--set", "tokens=1000"]) == 0
    capsys.readouterr()
    assert main(["budget", str(proj), "--json"]) == 0
    assert json.loads(capsys.readouterr().out)["budgets"]["tokens"]["limit"] == 1000
    assert main(["budget", str(proj), "--set", "widgets=1"]) == 2
    assert main(["budget", str(proj), "--set", "bogus"]) == 2


def test_budget_no_manifest(tmp_path, monkeypatch, capsys):
    monkeypatch.chdir(tmp_path)
    capsys.readouterr()
    assert main(["budget", str(tmp_path)]) == 2


def test_resume_reports_and_advances(proj, capsys):
    capsys.readouterr()
    assert main(["resume"]) == 0
    out = capsys.readouterr().out
    assert "resume greenfield-full-lite: 0/9 done, next: research" in out
    assert "pending gates: scope-approval" in out
    assert main(["resume", "--json"]) == 0
    payload = json.loads(capsys.readouterr().out)
    assert payload["position"]["total"] == 9 and payload["position"]["next"] == "research"


def test_resume_no_manifest(tmp_path, monkeypatch, capsys):
    monkeypatch.chdir(tmp_path)
    capsys.readouterr()
    assert main(["resume"]) == 2


def test_approvals_lists_and_clears(proj, capsys):
    capsys.readouterr()
    assert main(["approvals"]) == 0
    out = capsys.readouterr().out
    assert "scope-approval" in out and "human-approval" in out and "awaiting" in out
    assert "rationale" in out  # limitation note present
    assert main(["approvals", "--json"]) == 0
    pending = json.loads(capsys.readouterr().out)["pending"]
    assert {e["gate"] for e in pending} == {"scope-approval", "arch-approval", "merge-approval"}
    assert main(["approve", "scope-approval"]) == 0
    assert main(["approve", "arch-approval"]) == 0
    assert main(["approve", "merge-approval"]) == 0
    capsys.readouterr()
    assert main(["approvals", "--watch"]) == 0
    assert "no pending approvals" in capsys.readouterr().out


def test_approvals_bad_interval(proj, capsys):
    capsys.readouterr()
    assert main(["approvals", "--interval", "0"]) == 2


def test_approvals_policy_gate_pending(proj, capsys):
    write(proj / ".shiploom" / "workflows" / "dep.md",
          "---\nname: dep\nversion: 1.0.0\nkind: sequential\nresume: true\n"
          "budgets:\n  tokens: 1\n  spendUSD: 1\n  wallClockH: 1\nsteps:\n"
          "  - id: ship\n    consumes: [idea.md]\n    produces: [idea.md]\n"
          "    gate: policy\n    action: deploy.prod\n    resource: prod\n---\nBody\n")
    assert main(["run", "dep"]) == 0  # adopts dep, pauses at policy approval
    capsys.readouterr()
    assert main(["approvals", "--json"]) == 0
    pending = json.loads(capsys.readouterr().out)["pending"]
    assert any(e["gate"] == "ship" and e["kind"] == "policy" for e in pending)


def test_add_skill_local(proj, capsys):
    src = proj / "src-skill"
    write(src / "SKILL.md", SKILL_SRC)
    capsys.readouterr()
    assert main(["add", "skill", "demo-skill", "--from", str(src)]) == 0
    out = capsys.readouterr().out
    assert "installed skill demo-skill" in out and "unsigned" in out
    dest = proj / ".shiploom" / "skills" / "demo-skill" / "SKILL.md"
    assert dest.is_file()
    prov = json.loads((proj / ".shiploom" / "skills" / "_provenance.json").read_text())
    assert prov["demo-skill"]["signed"] is False
    assert main(["add", "skill", "demo-skill", "--from", str(src)]) == 2  # exists
    assert main(["add", "skill", "demo-skill", "--from", str(src), "--force"]) == 0


def test_add_rejects_name_mismatch(proj, capsys):
    src = proj / "src-skill"
    write(src / "SKILL.md", SKILL_SRC)
    capsys.readouterr()
    assert main(["add", "skill", "other-name", "--from", str(src)]) == 2
    assert "must equal directory name" in capsys.readouterr().err
    assert not (proj / ".shiploom" / "skills" / "other-name").exists()  # rolled back


def test_add_rejects_invalid_and_missing(proj, capsys):
    write(proj / "bad.md", "---\nname: bad\nversion: 1.0.0\nsteps:\n"
                            "  - id: a\n    bogus-key: 1\n---\nbody\n")
    capsys.readouterr()
    assert main(["add", "workflow", "bad", "--from", str(proj / "bad.md")]) == 2
    assert main(["add", "skill", "x", "--from", str(proj / "missing")]) == 2
    with pytest.raises(SystemExit) as exc:
        main(["add", "bogus", "x", "--from", "."])
    assert exc.value.code == 2


def test_add_workflow_and_policy(proj, capsys):
    wf = ("---\nname: mini2\nversion: 1.0.0\nkind: sequential\nresume: true\n"
          "steps:\n  - id: a\n    produces: [idea.md]\n    gate: none\n---\nBody\n")
    write(proj / "mini2.md", wf)
    capsys.readouterr()
    assert main(["add", "workflow", "mini2", "--from", str(proj / "mini2.md")]) == 0
    assert (proj / ".shiploom" / "workflows" / "mini2.md").is_file()
    pol = {"policyId": "t", "version": "1.0.0",
           "rules": [{"id": "r", "effect": "allow", "actions": ["a"], "resources": ["*"]}], }
    write(proj / "t.json", json.dumps(pol))
    assert main(["add", "policy", "t", "--from", str(proj / "t.json")]) == 0


def test_add_from_git_url_with_tag(proj, capsys, tmp_path):
    if not __import__("shutil").which("git"):
        pytest.skip("git not available")
    repo = tmp_path / "upstream"
    repo.mkdir()
    src = repo / "cool"
    write(src / "SKILL.md", SKILL_SRC.replace("name: demo-skill", "name: cool"))
    subprocess.run(["git", "init", "-q"], cwd=repo, check=True)
    subprocess.run(["git", "add", "."], cwd=repo, check=True)
    subprocess.run(["git", "-c", "user.email=t@t", "-c", "user.name=t",
                    "commit", "-qm", "x"], cwd=repo, check=True)
    subprocess.run(["git", "tag", "1.0.0"], cwd=repo, check=True)
    capsys.readouterr()
    assert main(["add", "skill", "cool", "--from", str(repo),
                 "--tag", "1.0.0"]) == 0
    assert (proj / ".shiploom" / "skills" / "cool" / "SKILL.md").is_file()
    assert main(["add", "skill", "cool2", "--from", str(repo)]) == 2  # tag required


def test_pending_approvals_library(proj):
    entries = approvals_mod.pending_approvals(proj)
    assert [e["gate"] for e in entries] == ["scope-approval", "arch-approval", "merge-approval"]
    assert all(e["attempts"] == 0 for e in entries)


def test_new_modules_stdlib_only():
    import ast as _ast
    for mod in ("approvals", "add"):
        tree = _ast.parse((REPO / "cli" / (mod + ".py")).read_text(encoding="utf-8"))
        imports = set()
        for node in _ast.walk(tree):
            if isinstance(node, _ast.Import):
                imports.update(a.name.split(".")[0] for a in node.names)
            elif isinstance(node, _ast.ImportFrom) and node.module:
                imports.add(node.module.split(".")[0])
        stdlib = set(getattr(sys, "stdlib_module_names", ())) or {
            "fnmatch", "json", "os", "shutil", "stat", "subprocess",
            "sys", "tempfile", "pathlib", "cli", "validators"}
        assert imports - stdlib - {"cli", "validators"} == set(), (mod, imports)
