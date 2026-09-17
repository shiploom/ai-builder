"""Hermetic unit tests for cli/characterize.py + command (no network)."""

import ast
import json
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))

from _stdlib import assert_stdlib_only
from cli import characterize as characterize_mod  # noqa: E402
from cli.shiploom import main  # noqa: E402

REPO = Path(__file__).resolve().parents[2]
PY = sys.executable

ACC = {"id": "ACC-001",
       "statement": "Reset email arrives within 60 seconds for valid accounts",
       "howToVerify": {"type": "script", "command": "pytest t.py -q",
                       "expect": "exit 0 under 60s"},
       "oracleRef": "oracle/ACC-001.sh"}

REPORT = {"id": "VR-001", "attemptRef": "diff:x", "verifier": "agent:verifier:fresh",
          "results": [{"acceptanceId": "ACC-001", "result": "pass",
                       "evidence": "oracle log", "oracleUsed": True}],
          "gatesSummary": {"test": "pass", "secrets": "pass"},
          "verdict": "pass", "timestamp": "2026-09-17T10:00:00Z"}


def write(path, content):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")
    return path


@pytest.fixture()
def proj(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    assert main(["init"]) == 0
    return tmp_path


def test_capture_and_unchanged_diff(proj):
    write(proj / "marker.txt", "hello\n")
    entry = characterize_mod.capture(proj, "base", command="cat marker.txt")
    assert entry["exit"] == 0 and entry["command"] == "cat marker.txt"
    assert entry["timeoutS"] == 600 and entry["actor"] == "human"
    stored = json.loads((proj / ".shiploom" / "characterization" / "base.json")
                        .read_text(encoding="utf-8"))
    assert stored["outputSha"] == entry["outputSha"]
    assert characterize_mod.list_snapshots(proj) == ["base"]
    result = characterize_mod.diff(proj, "base")
    assert result["changed"] is False and result["exitChanged"] is False
    assert result["outputChanged"] is False and result["unifiedDiff"] == []


def test_diff_detects_change(proj):
    write(proj / "marker.txt", "hello\n")
    characterize_mod.capture(proj, "base", command="cat marker.txt")
    write(proj / "marker.txt", "world\n")
    result = characterize_mod.diff(proj, "base")
    assert result["changed"] is True and result["outputChanged"] is True
    assert result["exitChanged"] is False
    assert any(line.startswith("-hello") for line in result["unifiedDiff"])
    assert any(line.startswith("+world") for line in result["unifiedDiff"])


def test_diff_detects_exit_change(proj):
    characterize_mod.capture(proj, "rc", command="%s -c \"import sys; sys.exit(0)\"" % PY)
    (proj / ".shiploom" / "characterization" / "rc.json").write_text(
        json.dumps(dict(json.loads((proj / ".shiploom" / "characterization" / "rc.json")
                                   .read_text()), exit=3)))
    write(proj / "fail.py", "import sys; sys.exit(1)\n")
    entry = json.loads((proj / ".shiploom" / "characterization" / "rc.json").read_text())
    entry["command"] = "%s fail.py" % PY
    (proj / ".shiploom" / "characterization" / "rc.json").write_text(json.dumps(entry))
    result = characterize_mod.diff(proj, "rc")
    assert result["changed"] is True and result["exitChanged"] is True


def test_names_rejected_and_missing_snapshot(proj):
    with pytest.raises(ValueError):
        characterize_mod.capture(proj, "../evil", command="echo hi")
    with pytest.raises(ValueError):
        characterize_mod.capture(proj, "", command="echo hi")
    assert characterize_mod.diff(proj, "ghost")["error"].startswith("no snapshot")
    assert characterize_mod.list_snapshots(proj) == []


def test_capture_needs_a_command(proj):
    with pytest.raises(ValueError) as exc:
        characterize_mod.capture(proj, "x")
    assert "no command" in str(exc.value)


def test_capture_uses_configured_test_command(proj):
    write(proj / "tests" / "test_ok.py", "def test_ok():\n    assert True\n")
    entry = characterize_mod.capture(proj, "suite")
    assert entry["exit"] == 0 and "pytest" in entry["command"]


def test_cli_codes_and_shapes(proj, capsys):
    capsys.readouterr()
    with pytest.raises(SystemExit):
        main(["characterize"])  # mode required (argparse)
    with pytest.raises(SystemExit):
        main(["characterize", "--capture", "a", "--diff", "b"])  # exclusive
    assert main(["characterize", "--timeout", "-1", "--capture", "a",
                 "--command", "echo hi"]) == 2
    write(proj / "marker.txt", "hello\n")
    assert main(["characterize", "--capture", "base",
                 "--command", "cat marker.txt", "--json"]) == 0
    payload = json.loads(capsys.readouterr().out)
    assert payload["ok"] is True and payload["snapshot"]["name"] == "base"
    assert main(["characterize", "--diff", "base"]) == 0
    assert "unchanged: base" in capsys.readouterr().out
    assert main(["characterize", "--list"]) == 0
    assert "base" in capsys.readouterr().out
    assert main(["characterize", "--diff", "ghost"]) == 2


def _locked_project_with_twin(proj):
    write(proj / "acceptance" / "a.json", json.dumps([ACC]))
    vault = proj / ".shiploom" / ".oracle" / "oracle"
    vault.mkdir(parents=True, exist_ok=True)
    write(vault / "ACC-001.sh", "#!/bin/sh\nexit 0\n")
    assert main(["lock"]) == 0
    write(proj / "verification" / "verification-report.json", json.dumps(REPORT))


def test_verify_warns_on_changed_snapshot(proj, capsys):
    from cli import gates as gates_mod
    capsys.readouterr()
    _locked_project_with_twin(proj)
    write(proj / "marker.txt", "hello\n")
    characterize_mod.capture(proj, "base", command="cat marker.txt")
    ok, _, warnings = gates_mod.verify_step_check(proj)
    assert ok is True and warnings == []
    write(proj / "marker.txt", "world\n")
    ok, _, warnings = gates_mod.verify_step_check(proj)
    assert ok is True  # report-only: changed behavior warns, never fails
    assert any("behavior changed" in w["message"] for w in warnings)


def test_run_surfaces_checker_warnings(proj, capsys):
    capsys.readouterr()
    _locked_project_with_twin(proj)
    write(proj / "marker.txt", "hello\n")
    characterize_mod.capture(proj, "base", command="cat marker.txt")
    write(proj / ".shiploom" / "workflows" / "vrt.md",
          "---\nname: vrt\nversion: 1.0.0\nkind: sequential\nresume: true\n"
          "budgets:\n  tokens: 100\n  spendUSD: 1\n  wallClockH: 1\nsteps:\n"
          "  - id: verify\n    produces: [verification/verification-report.json]\n"
          "    gate: verification\n    retries: 2\n    onFail: replan\n---\nBody\n")
    assert main(["run", "vrt"]) == 0
    assert "advanced: verify" in capsys.readouterr().out
    write(proj / "marker.txt", "world\n")
    assert main(["run", "vrt", "--from", "verify", "--json"]) == 0
    payload = json.loads(capsys.readouterr().out)
    assert any("behavior changed" in w["message"] for w in payload["warnings"])


def test_characterize_module_stdlib_only():
    assert_stdlib_only(REPO / "cli" / "characterize.py", extra={"cli"})
