"""Hermetic unit tests for cli/run.py + run/approve commands (no network)."""

import ast
import json
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))
from _stdlib import assert_stdlib_only

from cli import manifest  # noqa: E402
from cli import run as run_mod  # noqa: E402
from cli.shiploom import main  # noqa: E402

REPO = Path(__file__).resolve().parents[2]

MINI_WF = """\
---
name: mini
version: 1.0.0
kind: sequential
resume: true
budgets:
  tokens: 1000
  spendUSD: 1
  wallClockH: 1
steps:
  - id: draft
    consumes: [idea.md]
    produces: [out/note.md]
    gate: none
  - id: review
    gate: human-approval
    onDeny: pause
  - id: ship
    consumes: [out/note.md]
    produces: [out/final.md]
    gate: none
---

# Mini workflow (test fixture)
"""

NOTE_MD = """\
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
    write(tmp_path / ".shiploom" / "workflows" / "mini.md", MINI_WF)
    return tmp_path


def run(*argv):
    return main(["run"] + list(argv))


def test_workflow_definitions_strict_clean():
    wf_dir = REPO / "core" / "workflows"
    files = sorted(wf_dir.glob("*.md"))
    assert len(files) >= 2
    from validators.validate import validate_path
    for path in files:
        errors, warnings, _ = validate_path(path, strict=True)
        assert errors == [], (path, errors)
        assert warnings == [], (path, warnings)


def test_overlay_shadows_core(proj):
    assert run_mod.find_workflow("mini", proj).parent == proj / ".shiploom" / "workflows"
    assert run_mod.find_workflow("greenfield-full-lite", proj).parent == \
        REPO / "core" / "workflows"
    assert run_mod.find_workflow("nope", proj) is None


def test_pause_on_missing_outputs_then_advance(proj, capsys):
    capsys.readouterr()
    assert run("mini") == 0
    assert "missing outputs: out/note.md" in capsys.readouterr().out
    write(proj / "out" / "note.md", NOTE_MD)
    assert run("mini") == 0
    assert "awaiting approval: review" in capsys.readouterr().out
    data = manifest.load(proj)
    assert data["steps"]["draft"]["state"] == "done"
    assert data["checkpoints"][0]["id"] == "cp-draft"


def test_run_idempotent_skips_done(proj, capsys):
    write(proj / "out" / "note.md", NOTE_MD)
    run("mini")
    capsys.readouterr()
    assert run("mini") == 0
    assert "advanced" not in capsys.readouterr().out  # nothing new to do


def test_approve_advances_and_deny_blocks(proj, capsys):
    write(proj / "out" / "note.md", NOTE_MD)
    run("mini")
    capsys.readouterr()
    assert main(["approve", "review", "--deny"]) == 2  # reason required
    assert main(["approve", "review", "--deny", "--reason", "thin"]) == 0
    assert "denied review" in capsys.readouterr().out
    assert run("mini") == 2  # denied gate halts
    assert main(["approve", "review"]) == 0  # re-approve unblocks
    write(proj / "out" / "final.md", NOTE_MD.replace("RPT-090", "RPT-091"))
    assert run("mini") == 0
    assert "workflow mini complete" in capsys.readouterr().out


def test_approve_unknown_and_nonhuman_gate(proj, capsys):
    capsys.readouterr()
    assert main(["approve", "nope"]) == 2
    assert "unknown gate" in capsys.readouterr().err
    assert main(["approve", "draft"]) == 2  # not a step of the bound workflow
    write(proj / "out" / "note.md", NOTE_MD)
    run("mini")  # adopt mini
    capsys.readouterr()
    assert main(["approve", "draft"]) == 2  # gate: none, not human-approval
    assert "not an approvable gate" in capsys.readouterr().err


def test_only_requires_predecessors(proj, capsys):
    capsys.readouterr()
    assert run("mini", "--only", "ship") == 2
    assert "predecessor" in capsys.readouterr().out


def test_only_and_from(proj, capsys):
    write(proj / "out" / "note.md", NOTE_MD)
    capsys.readouterr()
    assert run("mini", "--only", "draft") == 0
    assert "advanced: draft" in capsys.readouterr().out
    assert main(["approve", "review"]) == 0
    capsys.readouterr()
    assert run("mini", "--from", "draft") == 0  # resets draft onward
    data = manifest.load(proj)
    assert data["steps"]["draft"]["state"] == "done"  # re-advanced (file exists)
    assert run("mini", "--only", "bogus") == 2


def test_unknown_workflow_and_missing_manifest(tmp_path, monkeypatch, capsys):
    monkeypatch.chdir(tmp_path)
    assert main(["init"]) == 0
    capsys.readouterr()
    assert run("nope") == 2
    (tmp_path / ".shiploom" / "manifest.json").unlink()
    assert run("mini") == 2


def test_unknown_uses_and_missing_consumes(proj, capsys):
    bad = MINI_WF.replace("consumes: [idea.md]",
                          "uses: skills/does-not-exist\n    consumes: [idea.md]")
    write(proj / ".shiploom" / "workflows" / "mini.md", bad)
    capsys.readouterr()
    assert run("mini") == 2
    assert "unknown uses ref" in capsys.readouterr().out
    bad = MINI_WF.replace("consumes: [idea.md]", "consumes: [missing.md]")
    write(proj / ".shiploom" / "workflows" / "mini.md", bad)
    assert run("mini") == 2
    assert "missing inputs" in capsys.readouterr().out


def test_acceptance_lock_gate_pauses_until_locked(proj, capsys):
    wf = MINI_WF.replace(
        "  - id: ship\n    consumes: [out/note.md]\n    produces: [out/final.md]\n    gate: none",
        "  - id: acceptance-lock\n    produces: [acceptance/*.json]\n    gate: verification")
    write(proj / ".shiploom" / "workflows" / "mini.md", wf)
    write(proj / "out" / "note.md", NOTE_MD)
    run("mini")  # adopts mini, completes draft, pauses at review
    assert main(["approve", "review"]) == 0
    capsys.readouterr()
    assert run("mini") == 0
    out = capsys.readouterr().out
    assert "acceptance not locked yet" in out
    data = manifest.load(proj)
    assert data.get("retries", {}).get("acceptance-lock", 0) == 0  # no retry consumed


def test_verification_retries_then_replan(proj, capsys):
    acc = {"id": "ACC-001", "statement": "X arrives within 60 seconds for valid accounts",
           "howToVerify": {"type": "script", "command": "pytest t.py -q",
                           "expect": "exit 0 under 60s"},
           "oracleRef": "oracle/ACC-001.sh"}
    write(proj / "acceptance" / "a.json", json.dumps([acc]))
    vault = proj / ".shiploom" / ".oracle" / "oracle"
    vault.mkdir(parents=True, exist_ok=True)
    write(vault / "ACC-001.sh", "#!/bin/sh\nexit 0\n")
    main(["lock"])
    wf = ("---\nname: vrt\nversion: 1.0.0\nkind: sequential\nresume: true\n"
          "budgets:\n    tokens: 100\n    spendUSD: 1\n    wallClockH: 1\nsteps:\n"
          "  - id: acceptance-lock\n    produces: [acceptance/*.json]\n"
          "    gate: verification\n    retries: 2\n    onFail: replan\n---\nBody\n")
    write(proj / ".shiploom" / "workflows" / "vrt.md", wf)
    write(vault / "ACC-001.sh", "#!/bin/sh\nexit 1\n")  # break the lock
    capsys.readouterr()
    assert run("vrt") == 0 and "attempt 1/2" in capsys.readouterr().out
    assert run("vrt") == 0 and "attempt 2/2" in capsys.readouterr().out
    assert run("vrt") == 0 and "replan required" in capsys.readouterr().out
    data = manifest.load(proj)
    assert data["workflow"] == "vrt"  # adopted (no prior progress)
    assert any(c["id"] == "cp-replan-acceptance-lock" for c in data["checkpoints"])


def test_verification_abort(proj, capsys):
    acc = {"id": "ACC-001", "statement": "X arrives within 60 seconds for valid accounts",
           "howToVerify": {"type": "script", "command": "pytest t.py -q",
                           "expect": "exit 0 under 60s"},
           "oracleRef": "oracle/ACC-001.sh"}
    write(proj / "acceptance" / "a.json", json.dumps([acc]))
    vault = proj / ".shiploom" / ".oracle" / "oracle"
    vault.mkdir(parents=True, exist_ok=True)
    write(vault / "ACC-001.sh", "#!/bin/sh\nexit 0\n")
    main(["lock"])
    write(vault / "ACC-001.sh", "#!/bin/sh\nexit 1\n")  # break the lock
    wf = ("---\nname: vrt\nversion: 1.0.0\nkind: sequential\nresume: true\n"
          "budgets:\n    tokens: 100\n    spendUSD: 1\n    wallClockH: 1\nsteps:\n"
          "  - id: acceptance-lock\n    produces: [acceptance/*.json]\n"
          "    gate: verification\n    retries: 0\n    onFail: abort\n---\nBody\n")
    write(proj / ".shiploom" / "workflows" / "vrt.md", wf)
    capsys.readouterr()
    assert run("vrt") == 2  # broken lock -> check fails -> abort (no attempts allowed)


def test_budget_wallclock_exceeded(proj, capsys):
    write(proj / "out" / "note.md", NOTE_MD)
    capsys.readouterr()
    assert run("mini") == 0  # first run records startedAt
    assert run("mini", "--budget", "wallClockH=0") == 4
    assert "budget exceeded" in capsys.readouterr().out


def test_budget_bad_flags(proj, capsys):
    capsys.readouterr()
    assert run("mini", "--budget", "bogus") == 2
    assert run("mini", "--budget", "tokens=many") == 2
    assert run("mini", "--budget", "widgets=1") == 2


def test_workflow_switch_guarded(proj, capsys):
    write(proj / "out" / "note.md", NOTE_MD)
    run("mini")
    capsys.readouterr()
    assert run("greenfield-full-lite") == 2  # mini in progress
    assert "in progress" in capsys.readouterr().out


def test_status_shows_steps(proj, capsys):
    write(proj / "out" / "note.md", NOTE_MD)
    run("mini")
    capsys.readouterr()
    assert main(["status", str(proj), "--json"]) == 0
    payload = json.loads(capsys.readouterr().out)
    assert payload["manifest"]["steps"]["draft"] == "done"


def test_run_module_stdlib_only():
    assert_stdlib_only(REPO / "cli" / "run.py", extra={"cli", "validators"})
