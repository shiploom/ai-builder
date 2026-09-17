"""Hermetic unit tests for --version, pin, upgrade, version parity (no network)."""

import json
import re
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))

from cli.shiploom import main  # noqa: E402

REPO = Path(__file__).resolve().parents[2]
CORE_VERSION = (REPO / "core" / "VERSION").read_text().strip()


@pytest.fixture()
def proj(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    assert main(["init"]) == 0
    return tmp_path


def _manifest(proj):
    return json.loads((proj / ".shiploom" / "manifest.json").read_text())


def _write_manifest(proj, data):
    (proj / ".shiploom" / "manifest.json").write_text(
        json.dumps(data, indent=2, sort_keys=True) + "\n")


def _write_config(proj, data):
    (proj / ".shiploom" / "config.json").write_text(
        json.dumps(data, indent=2, sort_keys=True) + "\n")


def test_version_flag(capsys):
    assert main(["--version"]) == 0
    out = capsys.readouterr().out
    assert CORE_VERSION in out and "python" in out


def test_bare_invocation_still_errors(capsys):
    with pytest.raises(SystemExit) as exc:
        main([])
    assert exc.value.code == 2


def test_pin_write_check_and_drift(proj, capsys):
    capsys.readouterr()
    assert main(["pin"]) == 0
    lock = json.loads((proj / ".shiploom" / "lock.json").read_text())
    assert lock["coreVersion"] == CORE_VERSION
    assert lock["manifestCore"] == CORE_VERSION
    assert lock["models"].startswith("BYO")
    assert main(["pin", "--check"]) == 0
    assert "pin clean" in capsys.readouterr().out
    lock["coreVersion"] = "0.0.0"
    (proj / ".shiploom" / "lock.json").write_text(json.dumps(lock))
    assert main(["pin", "--check"]) == 2
    assert "drift" in capsys.readouterr().out
    assert main(["pin", "--check", "--json"]) == 2
    payload = json.loads(capsys.readouterr().out)
    assert payload["ok"] is False and any("coreVersion" in d for d in payload["drifts"])


def test_pin_needs_manifest(tmp_path, monkeypatch, capsys):
    monkeypatch.chdir(tmp_path)
    capsys.readouterr()
    assert main(["pin"]) == 2
    assert main(["pin", "--check"]) == 2


def _age(proj, version="0.9.0"):
    data = _manifest(proj)
    data["coreVersion"] = version
    _write_manifest(proj, data)
    config = json.loads((proj / ".shiploom" / "config.json").read_text())
    config["coreVersion"] = version
    _write_config(proj, config)


def test_upgrade_dry_run_upgrade_rollback(proj, capsys):
    _age(proj)
    capsys.readouterr()
    assert main(["upgrade", "--dry-run"]) == 0
    out = capsys.readouterr().out
    assert "0.9.0 -> %s" % CORE_VERSION in out and "would change" in out
    assert main(["upgrade"]) == 0
    assert "upgraded 0.9.0 -> %s" % CORE_VERSION in capsys.readouterr().out
    assert _manifest(proj)["coreVersion"] == CORE_VERSION
    assert (_manifest(proj)["coreVersion"] ==
            json.loads((proj / ".shiploom" / "config.json").read_text())["coreVersion"])
    assert main(["upgrade"]) == 0  # idempotent when current
    assert "already at core" in capsys.readouterr().out
    assert main(["upgrade", "--rollback"]) == 0
    assert "rolled back to core 0.9.0" in capsys.readouterr().out
    assert _manifest(proj)["coreVersion"] == "0.9.0"
    assert main(["upgrade", "--rollback"]) == 2  # backup consumed


def test_upgrade_rejects_incompatible_manifest(proj, capsys):
    data = _manifest(proj)
    del data["budgets"]
    _write_manifest(proj, data)
    capsys.readouterr()
    assert main(["upgrade", "--dry-run", "--json"]) == 2  # incompatible: fail-closed
    payload = json.loads(capsys.readouterr().out)
    assert payload["ok"] is False and payload["missingKeys"] == ["budgets"]
    assert main(["upgrade"]) == 2


def test_upgrade_auto_rollback_on_failed_validation(proj, capsys):
    _age(proj)
    (proj / "bad.md").write_text("---\nid: [unclosed\n---\nbody\n")
    capsys.readouterr()
    assert main(["upgrade"]) == 2
    out = capsys.readouterr().out
    assert "rolled back" in out
    assert _manifest(proj)["coreVersion"] == "0.9.0"  # restored


def test_dual_ship_versions_agree():
    """Static half of scripts/version-check.sh: every version file tracks core."""
    pyproject = (REPO / "pyproject.toml").read_text()
    pkg = re.search(r'^version = "([^"]+)"', pyproject, re.M).group(1)
    npx = json.loads((REPO / "wrappers" / "npx" / "package.json").read_text())["version"]
    assert pkg == CORE_VERSION == npx


def test_version_reports_core(capsys):
    capsys.readouterr()
    assert main(["--version"]) == 0
    assert "(core %s," % CORE_VERSION in capsys.readouterr().out
