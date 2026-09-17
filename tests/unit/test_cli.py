"""Hermetic unit tests for the shiploom CLI (no network, stdlib only)."""

import ast
import json
import os
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))
from _stdlib import assert_stdlib_only

from cli import auditlog, manifest  # noqa: E402
from cli.shiploom import main  # noqa: E402
from validators.status import format_human  # noqa: E402

REPO = Path(__file__).resolve().parents[2]
CORE_VERSION = (REPO / "core" / "VERSION").read_text().strip()


@pytest.fixture()
def proj(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    return tmp_path


def test_install_local(proj):
    assert main(["install", "--local"]) == 0
    dest = proj / ".shiploom" / "core" / CORE_VERSION
    assert (dest / "receipt.json").exists()
    assert len(list((dest / "schemas").glob("*.schema.json"))) == 9
    assert (dest / "validators" / "validate.py").exists()
    assert (dest / "core" / "VERSION").read_text().strip() == CORE_VERSION
    assert "__pycache__" not in [p.name for p in dest.rglob("*")]


def test_install_global_uses_home(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    monkeypatch.setenv("HOME", str(tmp_path))
    assert main(["install", "--global"]) == 0
    assert (tmp_path / ".shiploom" / "core" / CORE_VERSION / "receipt.json").exists()


def test_install_version_mismatch(proj):
    assert main(["install", "--version", "9.9.9"]) == 2


def test_install_global_and_local_conflict(proj):
    assert main(["install", "--global", "--local"]) == 2


def test_init_green(proj):
    assert main(["init", "--green", "--harness", "auto", "--stack", "python"]) == 0
    dot = proj / ".shiploom"
    config = json.loads((dot / "config.json").read_text())
    assert config["coreVersion"] == CORE_VERSION
    assert config["harness"] == "auto"
    assert config["workflow"] == "greenfield-full-lite"
    assert config["adapterTargets"] == ["claude", "opencode"]
    assert (proj / "idea.md").exists()
    assert (dot / "mcp-registry.json").exists()
    assert (dot / ".gitignore").read_text() == ".oracle/\n"
    mode = (dot / ".oracle").stat().st_mode & 0o777
    if os.name == "posix":
        assert mode == 0o700
    assert manifest.load(proj)["workflow"] == "greenfield-full-lite"
    ok, errors = auditlog.verify(proj)
    assert ok, errors


def test_init_existing_seeds_repo_map(proj):
    assert main(["init", "--existing", "--harness", "claude"]) == 0
    assert (proj / "brownfield" / "repo-map.md").exists()
    config = json.loads((proj / ".shiploom" / "config.json").read_text())
    assert config["workflow"] == "brownfield-fix"
    assert config["adapterTargets"] == ["claude"]


def test_init_refuses_overwrite_without_force(proj):
    assert main(["init"]) == 0
    assert main(["init"]) == 2
    assert main(["init", "--force"]) == 0


def test_init_bad_harness_rejected(proj):
    with pytest.raises(SystemExit) as exc:
        main(["init", "--harness", "bogus"])
    assert exc.value.code == 2


def test_seeded_idea_strict_clean(proj):
    main(["init"])
    from validators.validate import validate_path
    errors, warnings, _ = validate_path(proj / "idea.md", strict=True)
    assert errors == [] and warnings == []


def test_manifest_genesis_and_roundtrip(proj):
    data = manifest.genesis("1.0.0-draft", "workflows/greenfield-full-lite")
    assert data["artifacts"] == {} and data["gates"] == {}
    assert data["budgets"]["tokens"]["limit"] == 800000
    assert data["workflowVersion"] == "1.0.0"
    manifest.save(proj, data)
    assert manifest.load(proj) == data
    data["gates"]["scope-approval"] = {"state": "passed"}
    manifest.save(proj, data)
    assert manifest.load(proj)["gates"]["scope-approval"]["state"] == "passed"


def test_manifest_corrupt_raises(proj):
    manifest.save(proj, manifest.genesis("x", "w"))
    manifest.manifest_path(proj).write_text("{nope", encoding="utf-8")
    with pytest.raises(ValueError):
        manifest.load(proj)
    with pytest.raises(FileNotFoundError):
        manifest.load(proj / "missing")


def test_sha256_file_format(proj):
    target = proj / "f.bin"
    target.write_bytes(b"abc")
    assert manifest.sha256_file(target).startswith("sha256:")
    assert len(manifest.sha256_file(target)) == len("sha256:") + 64


def test_audit_chain_and_tamper(proj):
    auditlog.init_log(proj)
    auditlog.append(proj, actor="human:priya", action="approve.prod", target="DEP-001")
    ok, errors = auditlog.verify(proj)
    assert ok, errors
    lines = (proj / ".shiploom" / "audit.jsonl").read_text().splitlines()
    assert len(lines) == 2
    tampered = json.loads(lines[1])
    tampered["action"] = "approve.everything"
    lines[1] = json.dumps(tampered, sort_keys=True)
    (proj / ".shiploom" / "audit.jsonl").write_text("\n".join(lines) + "\n")
    ok, errors = auditlog.verify(proj)
    assert not ok and errors


def test_audit_append_autoinits(proj):
    entry = auditlog.append(proj, actor="system", action="x", target=".")
    assert entry["prev"] != "GENESIS"  # chained to the auto-created genesis
    ok, _ = auditlog.verify(proj)
    assert ok


def test_audit_export_md(proj):
    auditlog.init_log(proj)
    md = auditlog.export_md(proj)
    assert md.startswith("# Audit log (1 entries)") and "log.genesis" in md


def test_validate_wrap_codes(proj, capsys):
    (proj / "ok.md").write_text("# plain\n")
    assert main(["validate", str(proj)]) == 0
    assert json.loads(capsys.readouterr().out)["ok"] is True
    (proj / "bad.md").write_text("---\nid: REQ-001\nkind: requirement\n---\nbody\n")
    assert main(["validate", str(proj)]) == 2
    assert json.loads(capsys.readouterr().out)["ok"] is False


def test_status_merges_manifest(proj, capsys):
    main(["init"])
    capsys.readouterr()
    assert main(["status", str(proj), "--json"]) == 0
    payload = json.loads(capsys.readouterr().out)
    assert payload["manifest"]["workflow"] == "greenfield-full-lite"
    assert payload["budgets"]["tokens"]["limit"] == 800000


def test_status_human_with_manifest_budgets(proj, capsys):
    main(["init"])
    assert main(["status", str(proj)]) == 0
    out = capsys.readouterr().out
    assert "budgets: tokens 0/800000" in out
    assert "workflow: greenfield-full-lite" in out


def test_format_human_manifest_budgets_shape():
    payload = {"target": ".", "coreVersion": "t",
               "artifacts": [], "counts": {"total": 0, "byStatus": {}},
               "invalid": [], "trace": {"nodes": 0, "edges": 0, "errors": []},
               "budgets": {"tokens": {"limit": 1, "used": 0},
                           "spendUSD": {"limit": 2.0, "used": 0.0}}}
    assert "budgets: tokens 0/1" in format_human(payload)


def test_doctor_json_ok_after_init(proj, capsys):
    main(["init"])
    capsys.readouterr()
    assert main(["doctor", "--json"]) == 0
    report = json.loads(capsys.readouterr().out)
    assert report["ok"] is True
    names = [c["name"] for c in report["checks"]]
    assert "project-manifest" in names and "project-audit" in names


def test_doctor_detects_broken_audit(proj, capsys):
    main(["init"])
    capsys.readouterr()
    (proj / ".shiploom" / "audit.jsonl").write_text("garbage\n")
    assert main(["doctor", "--json"]) == 5
    report = json.loads(capsys.readouterr().out)
    assert report["ok"] is False


def test_doctor_warns_not_fails_on_install_only(proj, capsys):
    main(["install", "--local"])
    capsys.readouterr()
    assert main(["doctor", "--json"]) == 0
    report = json.loads(capsys.readouterr().out)
    assert report["ok"] is True
    project = [c for c in report["checks"] if c["name"] == "project"]
    assert project and project[0]["status"] == "warn"


def test_audit_command_codes(proj, capsys):
    main(["init"])
    assert main(["audit"]) == 0
    assert "chain ok" in capsys.readouterr().out
    assert main(["audit", "--export", "json"]) == 0
    assert json.loads(capsys.readouterr().out)["ok"] is True
    assert main(["audit", "--export", "md"]) == 0
    assert "Audit log" in capsys.readouterr().out
    (proj / ".shiploom" / "audit.jsonl").write_text("garbage\n")
    assert main(["audit"]) == 2


def test_cli_modules_stdlib_only():
    for mod in ("shiploom", "manifest", "auditlog", "doctor"):
        assert_stdlib_only(REPO / "cli" / (mod + ".py"), extra={"cli", "validators"})
