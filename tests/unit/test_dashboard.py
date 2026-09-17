"""Hermetic unit tests for scripts/collect-dashboard.py (no network)."""

import json
import subprocess
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))

from cli.shiploom import main  # noqa: E402

REPO = Path(__file__).resolve().parents[2]
COLLECTOR = REPO / "scripts" / "collect-dashboard.py"


@pytest.fixture()
def proj(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    assert main(["init"]) == 0
    assert main(["approve", "scope-approval"]) == 0
    assert main(["verify", "--report"]) == 0
    return tmp_path


def test_collector_shape(proj, capsys):
    del capsys
    proc = subprocess.run([sys.executable, str(COLLECTOR), str(proj), "--json"],
                          capture_output=True, text=True, timeout=60)
    assert proc.returncode == 0
    dashboard = json.loads(proc.stdout)
    assert dashboard["workflow"] == "greenfield-full-lite"
    assert dashboard["steps"] == {"done": 0, "total": 0}
    assert dashboard["gates"] == {"scope-approval": "passed"}
    assert dashboard["budgets"]["tokens"]["limit"] == 800000
    assert dashboard["auditEvents"] >= 3
    assert dashboard["gateReport"]["verdict"] in ("pass", "fail")
    assert dashboard["defectEscapes"] is None
    assert dashboard["mergeStats"] is None
    assert dashboard["verifierCatchRate"] is None
    assert any("pilot" in w for w in dashboard["warnings"])


def test_collector_human_output(proj):
    proc = subprocess.run([sys.executable, str(COLLECTOR), str(proj)],
                          capture_output=True, text=True, timeout=60)
    assert proc.returncode == 0 and "dashboard:" in proc.stdout


def test_collector_missing_project(tmp_path):
    proc = subprocess.run([sys.executable, str(COLLECTOR), str(tmp_path / "nope"), "--json"],
                          capture_output=True, text=True, timeout=60)
    assert proc.returncode == 0  # warns, never fails
    dashboard = json.loads(proc.stdout)
    assert dashboard["workflow"] is None
    assert any("no manifest" in w for w in dashboard["warnings"])
