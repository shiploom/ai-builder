"""Hermetic unit tests for cli/oracle.py + `shiploom lock` (no network)."""

import ast
import json
import os
import shutil
import subprocess
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))
from _stdlib import assert_stdlib_only

from cli import auditlog, manifest  # noqa: E402
from cli import oracle as oracle_mod  # noqa: E402
from cli.shiploom import main  # noqa: E402

REPO = Path(__file__).resolve().parents[2]

ACC = {
    "id": "ACC-001",
    "statement": "Reset email arrives within 60 seconds for valid accounts",
    "howToVerify": {"type": "script", "command": "pytest t.py -q",
                    "expect": "exit 0 under 60s"},
    "oracleRef": "oracle/ACC-001.sh",
}


def write(path, content):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")
    return path


@pytest.fixture()
def proj(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    assert main(["init"]) == 0
    write(tmp_path / "acceptance" / "auth.json", json.dumps([ACC]))
    vault = tmp_path / ".shiploom" / ".oracle" / "oracle"
    vault.mkdir(parents=True, exist_ok=True)
    write(vault / "ACC-001.sh", "#!/bin/sh\nexit 0\n")
    return tmp_path


def test_lock_happy_path(proj, capsys):
    capsys.readouterr()
    assert main(["lock", "--actor", "human:priya"]) == 0
    out = capsys.readouterr().out
    assert "locked 1 criteria from 1 files" in out
    data = manifest.load(proj)
    assert data["acceptance"]["lockedBy"] == "human:priya"
    assert data["acceptance"]["criteria"]["ACC-001"]["oracleHash"].startswith("sha256:")
    actions = [e["action"] for e in auditlog.read_all(proj)]
    assert "acceptance.lock" in actions
    assert main(["lock", "--check"]) == 0
    assert "acceptance lock: ok" in capsys.readouterr().out


def test_lock_json_shape(proj, capsys):
    capsys.readouterr()
    assert main(["lock", "--json"]) == 0
    payload = json.loads(capsys.readouterr().out)
    assert payload["ok"] is True and payload["summary"] == {"criteria": 1, "files": 1}


def test_lock_missing_oracle(proj, capsys):
    (proj / ".shiploom" / ".oracle" / "oracle" / "ACC-001.sh").unlink()
    capsys.readouterr()
    assert main(["lock"]) == 2
    assert "missing oracle implementation" in capsys.readouterr().out


def test_lock_no_acceptance(tmp_path, monkeypatch, capsys):
    monkeypatch.chdir(tmp_path)
    main(["init"])
    capsys.readouterr()
    assert main(["lock"]) == 2
    assert "no acceptance" in capsys.readouterr().out


def test_lock_rejects_absolute_oracle_ref(proj, capsys):
    bad = dict(ACC, oracleRef="/etc/oracle.sh")
    write(proj / "acceptance" / "auth.json", json.dumps([bad]))
    capsys.readouterr()
    assert main(["lock"]) == 2
    assert "oracleRef" in capsys.readouterr().out


def test_lock_rejects_dotdot_oracle_ref(proj, capsys):
    bad = dict(ACC, oracleRef="../escape.sh")
    write(proj / "acceptance" / "auth.json", json.dumps([bad]))
    capsys.readouterr()
    assert main(["lock"]) == 2


def test_lock_duplicate_ids(proj, capsys):
    write(proj / "acceptance" / "more.json", json.dumps([ACC]))
    capsys.readouterr()
    assert main(["lock"]) == 2
    assert "duplicate acceptance id" in capsys.readouterr().out


def test_check_without_lock_fails(tmp_path, monkeypatch, capsys):
    monkeypatch.chdir(tmp_path)
    main(["init"])
    capsys.readouterr()
    assert main(["lock", "--check"]) == 2
    assert "no acceptance lock" in capsys.readouterr().out


def test_check_detects_redefinition(proj, capsys):
    main(["lock"])
    capsys.readouterr()
    doc = json.loads((proj / "acceptance" / "auth.json").read_text())
    doc[0]["statement"] = "Reset email arrives within 61 seconds for valid accounts"
    write(proj / "acceptance" / "auth.json", json.dumps(doc))
    assert main(["lock", "--check"]) == 2
    assert "redefined after lock" in capsys.readouterr().out


def test_check_detects_oracle_change(proj, capsys):
    main(["lock"])
    capsys.readouterr()
    write(proj / ".shiploom" / ".oracle" / "oracle" / "ACC-001.sh", "#!/bin/sh\nexit 1\n")
    assert main(["lock", "--check"]) == 2
    assert "oracle changed after lock" in capsys.readouterr().out


def test_check_detects_uncovered_file(proj, capsys):
    main(["lock"])
    capsys.readouterr()
    extra = dict(ACC, id="ACC-002", oracleRef="oracle/ACC-002.sh")
    write(proj / "acceptance" / "extra.json", json.dumps([extra]))
    assert main(["lock", "--check"]) == 2
    assert "not covered by lock" in capsys.readouterr().out


def test_relock_after_scope_change(proj, capsys):
    main(["lock"])
    capsys.readouterr()
    extra = dict(ACC, id="ACC-002", oracleRef="oracle/ACC-002.sh")
    write(proj / "acceptance" / "extra.json", json.dumps([extra]))
    write(proj / ".shiploom" / ".oracle" / "oracle" / "ACC-002.sh", "#!/bin/sh\nexit 0\n")
    assert main(["lock"]) == 0
    assert "locked 2 criteria from 2 files" in capsys.readouterr().out
    assert main(["lock", "--check"]) == 0


def test_lock_bad_vault_mode(proj, capsys):
    if os.name != "posix":
        pytest.skip("POSIX-only mode check")
    (proj / ".shiploom" / ".oracle").chmod(0o755)
    capsys.readouterr()
    assert main(["lock"]) == 2
    assert "not 0700" in capsys.readouterr().out


def test_oracle_ref_is_vault_relative(proj):
    vault = proj / ".shiploom" / ".oracle"
    assert oracle_mod._resolve_inside(vault, "oracle/ACC-001.sh") == vault / "oracle" / "ACC-001.sh"
    assert oracle_mod._resolve_inside(vault, "../escape.sh") is None
    assert oracle_mod._resolve_inside(vault, "/abs.sh") is None


def test_lock_detects_git_tracked_oracle(proj, capsys):
    if shutil.which("git") is None:
        pytest.skip("git not available")
    subprocess.run(["git", "init", "-q"], cwd=proj, check=True)
    # -f: .shiploom/.gitignore normally blocks this; force-add simulates
    # a leak (user override or negated global ignore).
    subprocess.run(["git", "add", "-f", ".shiploom/.oracle/oracle/ACC-001.sh",
                    "acceptance/auth.json"], cwd=proj, check=True)
    capsys.readouterr()
    assert main(["lock"]) == 2
    assert "tracked by git" in capsys.readouterr().out


def test_oracle_module_stdlib_only():
    assert_stdlib_only(REPO / "cli" / "oracle.py", extra={"cli", "validators"})
