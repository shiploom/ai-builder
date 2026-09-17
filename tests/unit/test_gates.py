"""Hermetic unit tests for cli/gates.py + `shiploom verify` (no network)."""

import ast
import json
import os
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))
from _stdlib import assert_stdlib_only

from cli import gates as gates_mod  # noqa: E402
from cli.shiploom import main  # noqa: E402

REPO = Path(__file__).resolve().parents[2]
PY = sys.executable


def write(path, content):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")
    return path


def set_gate(proj, gate_id, command, timeout_s=60):
    config_path = proj / ".shiploom" / "config.json"
    config = json.loads(config_path.read_text(encoding="utf-8"))
    config.setdefault("gates", {})[gate_id] = {"command": command, "timeoutS": timeout_s}
    config_path.write_text(json.dumps(config, indent=2, sort_keys=True) + "\n", encoding="utf-8")


@pytest.fixture()
def proj(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    assert main(["init"]) == 0
    return tmp_path


def test_secrets_finds_and_redacts(proj):
    # NOTE: trigger tokens are split so this file itself stays scan-clean;
    # the runtime strings written to tmp are identical.
    write(proj / "app.py", 'pass' + 'word = "hunter2hunter"\n')
    write(proj / "key.pem", "-----BEGIN " + "RSA PRIVATE KEY-----\nabc\n")
    write(proj / ".venv" / "app.py", 'pass' + 'word = "hunter2hunter"\n')
    blob = proj / "blob.bin"
    blob.write_bytes(b'\x00pass' + b'word = "hunter2hunter"\n')
    findings = gates_mod.scan_secrets(proj)
    rules = {(f["path"], f["rule"]) for f in findings}
    assert ("app.py", "secret-assign") in rules
    assert ("key.pem", "private-key") in rules
    assert not any(f["path"].startswith(".venv") for f in findings)
    assert not any("hunter2" in json.dumps(f) for f in findings)


def test_secrets_clean_tree(proj):
    assert gates_mod.scan_secrets(proj) == []


def test_dep_inventory_pins(proj):
    write(proj / "requirements.txt", "pinned==1.0\nloose>=2.0\n# comment\n")
    write(proj / "package.json", json.dumps({"dependencies": {"left": "1.2.3", "right": "^9.9.9"}}))
    inv = gates_mod.dep_inventory(proj)
    assert inv["deps"] == 4
    assert "pip:loose" in inv["unpinned"] and "npm:right" in inv["unpinned"]
    assert "pip:pinned" not in inv["unpinned"]


def test_configured_gate_pass_fail_missing(proj):
    set_gate(proj, "test", "%s -c \"import sys; sys.exit(0)\"" % PY)
    report = gates_mod.run_gates(proj, selected=["test"])
    assert report["gates"]["test"]["status"] == "pass"
    set_gate(proj, "test", "%s -c \"import sys; sys.exit(3)\"" % PY)
    report = gates_mod.run_gates(proj, selected=["test"])
    assert report["gates"]["test"]["status"] == "fail"
    assert report["gates"]["test"]["exit"] == 3
    assert report["verdict"] == "fail" and not report["ok"]
    set_gate(proj, "test", "definitely-not-a-real-binary-xyz")
    report = gates_mod.run_gates(proj, selected=["test"])
    assert report["gates"]["test"]["detail"] == "command not found"


def test_gate_timeout(proj):
    set_gate(proj, "test", "%s -c \"import time; time.sleep(30)\"" % PY, timeout_s=1)
    report = gates_mod.run_gates(proj, selected=["test"])
    assert report["gates"]["test"]["status"] == "fail"
    assert "timeout" in report["gates"]["test"]["detail"]


def test_unconfigured_gates_skip(proj):
    report = gates_mod.run_gates(proj, selected=["build", "lint", "contract"])
    assert all(r["status"] == "skip" for r in report["gates"].values())
    assert report["ok"] is True


def test_unknown_gate_selection(proj):
    report = gates_mod.run_gates(proj, selected=["nope"])
    assert report["ok"] is False and report["errors"]


def test_determinism_second_run_flaky(proj):
    script = ("import pathlib, sys;"
              " p = pathlib.Path('marker');"
              " sys.exit(1) if p.exists() else p.write_text('x')")
    set_gate(proj, "test", "%s -c \"%s\"" % (PY, script))
    report = gates_mod.run_gates(proj, selected=["test"])
    assert report["gates"]["test"]["status"] == "fail"
    assert "flaky" in report["gates"]["test"]["detail"]


def test_compile_gate_catches_syntax_error(proj):
    write(proj / "broken.py", "def f(:\n")
    report = gates_mod.run_gates(proj, selected=["compile"])
    assert report["gates"]["compile"]["status"] == "fail"


def test_verify_command_and_report(proj, capsys):
    capsys.readouterr()
    assert main(["verify"]) == 0
    assert "verify: pass" in capsys.readouterr().out
    assert main(["verify", "--report"]) == 0
    capsys.readouterr()
    gate_report = json.loads((proj / "verification" / "gate-report.json").read_text())
    assert gate_report["verdict"] == "pass"
    assert main(["verify", "--json", "--gates", "secrets,compile"]) == 0
    payload = json.loads(capsys.readouterr().out)
    assert set(payload["gates"]) == {"secrets", "compile"}


def test_verify_fails_on_secret(proj, capsys):
    write(proj / "leak.py", 'tok' + 'en = "abcdef12345"\n')
    capsys.readouterr()
    assert main(["verify", "--gates", "secrets"]) == 2


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


def _locked_project(proj):
    write(proj / "acceptance" / "a.json", json.dumps([ACC]))
    vault = proj / ".shiploom" / ".oracle" / "oracle"
    vault.mkdir(parents=True, exist_ok=True)
    write(vault / "ACC-001.sh", "#!/bin/sh\nexit 0\n")
    assert main(["lock"]) == 0
    write(proj / "verification" / "verification-report.json", json.dumps(REPORT))
    set_gate(proj, "test", "%s -c \"import sys; sys.exit(0)\"" % PY)


def test_verify_step_check_positive(proj, capsys):
    capsys.readouterr()
    _locked_project(proj)
    ok, errors, _ = gates_mod.verify_step_check(proj)
    assert ok, errors


def test_verify_step_check_gate_failure(proj, capsys):
    capsys.readouterr()
    _locked_project(proj)
    set_gate(proj, "test", "%s -c \"import sys; sys.exit(1)\"" % PY)
    ok, errors, _ = gates_mod.verify_step_check(proj)
    assert not ok and any("gate failed" in e["message"] for e in errors)


def test_verify_step_check_bad_verdict(proj, capsys):
    capsys.readouterr()
    _locked_project(proj)
    bad = dict(REPORT, verdict="fail")
    write(proj / "verification" / "verification-report.json", json.dumps(bad))
    ok, errors, _ = gates_mod.verify_step_check(proj)
    assert not ok and any("verdict" in e["message"] for e in errors)


def test_verify_step_check_unlocked_criterion(proj, capsys):
    capsys.readouterr()
    _locked_project(proj)
    bad = dict(REPORT, results=[{"acceptanceId": "ACC-999", "result": "pass",
                                 "evidence": "x", "oracleUsed": True}])
    write(proj / "verification" / "verification-report.json", json.dumps(bad))
    ok, errors, _ = gates_mod.verify_step_check(proj)
    assert not ok and any("unlocked" in e["message"] for e in errors)


def test_verify_step_check_missing_twin(proj, capsys):
    capsys.readouterr()
    _locked_project(proj)
    (proj / "verification" / "verification-report.json").unlink()
    ok, errors, _ = gates_mod.verify_step_check(proj)
    assert not ok and any("missing" in e["message"] for e in errors)


def test_gates_module_stdlib_only():
    assert_stdlib_only(REPO / "cli" / "gates.py", extra={"cli", "validators", "json"})


def test_sast_configured_and_skipped(proj):
    set_gate(proj, "sast", "%s -c \"import sys; sys.exit(0)\"" % PY)
    assert gates_mod.run_gates(proj, selected=["sast"])["gates"]["sast"]["status"] == "pass"
    set_gate(proj, "sast", "%s -c \"import sys; sys.exit(1)\"" % PY)
    assert gates_mod.run_gates(proj, selected=["sast"])["gates"]["sast"]["status"] == "fail"
    report = gates_mod.run_gates(proj, selected=["license", "mutation"])
    assert "sast" not in report["gates"]


def test_dast_configured_and_skipped(proj):
    set_gate(proj, "dast", "%s -c \"import sys; sys.exit(0)\"" % PY)
    assert gates_mod.run_gates(proj, selected=["dast"])["gates"]["dast"]["status"] == "pass"
    set_gate(proj, "dast", "%s -c \"import sys; sys.exit(1)\"" % PY)
    assert gates_mod.run_gates(proj, selected=["dast"])["gates"]["dast"]["status"] == "fail"
    report = gates_mod.run_gates(proj, selected=["license"])
    assert "dast" not in report["gates"]


def test_license_inventory_declared_and_unknown(proj):
    write(proj / "package-lock.json", json.dumps({
        "name": "x",
        "packages": {
            "": {"name": "x"},
            "node_modules/left-pad": {"version": "1.3.0", "license": "MIT"},
            "node_modules/evil": {"version": "9.9.9", "license": "GPL-3.0-only"},
            "node_modules/mystery": {"version": "1.0.0"},
        }}))
    write(proj / "package.json", json.dumps({"dependencies": {"left-pad": "1.3.0"}}))
    write(proj / "requirements.txt", "foo\n")
    inv = gates_mod.license_inventory(proj)
    assert inv["declared"] == {"npm:left-pad": "MIT", "npm:evil": "GPL-3.0-only"}
    assert inv["unknown"] == ["pip:foo"]
    assert inv["nonAllowlisted"] == ["npm:evil"]
    report = gates_mod.run_gates(proj, selected=["license"])
    assert report["gates"]["license"]["status"] == "pass"  # report-only
    assert "non-allowlisted" in report["warnings"]["license"]


def test_license_alias_normalization(proj):
    assert gates_mod._normalize_license("MIT License") == "MIT"
    assert gates_mod._normalize_license("  ") is None
    assert gates_mod._normalize_license(None) is None


OSV_CLEAN = {"results": []}
OSV_VULN = {"results": [{"packages": [{"package": {"name": "left-pad", "version": "1.3.0"},
                                        "vulnerabilities": [{"id": "GHSA-x"}]}]}]}


def _fake_osv(bindir, payload):
    script = bindir / "osv-scanner"
    script.write_text("#!/usr/bin/env python3\nimport json,sys\n"
                      "json.dump(%r, sys.stdout)\n" % (payload,))
    script.chmod(0o755)
    return bindir


def test_dep_audit_osv_clean_and_vuln(proj, tmp_path, monkeypatch):
    bindir = tmp_path / "bin"
    bindir.mkdir()
    write(proj / "package-lock.json", json.dumps({"name": "x", "packages": {}}))
    monkeypatch.setenv("PATH", str(bindir) + os.pathsep + os.environ.get("PATH", ""))
    _fake_osv(bindir, OSV_CLEAN)
    report = gates_mod.run_gates(proj, selected=["depAudit"])
    assert report["gates"]["depAudit"]["status"] == "pass"
    _fake_osv(bindir, OSV_VULN)
    report = gates_mod.run_gates(proj, selected=["depAudit"])
    gate = report["gates"]["depAudit"]
    assert gate["status"] == "fail" and "1 vuln(s)" in gate["detail"]
    assert gate["vulnerabilities"] == ["left-pad@1.3.0 GHSA-x"]


def test_dep_audit_osv_absent_falls_back(proj, tmp_path, monkeypatch):
    empty = tmp_path / "emptybin"
    empty.mkdir()
    monkeypatch.setenv("PATH", str(empty))
    report = gates_mod.run_gates(proj, selected=["depAudit"])
    assert report["gates"]["depAudit"]["status"] == "pass"
    assert "osv-scanner not on PATH" in report["gates"]["depAudit"]["detail"]


MUT_SRC = """\
def f(a, b):
    if a == b and b > 0:
        return True
    return a + b
    x = not a
    return x
"""


def test_mutation_sampler_counts_and_limits(proj):
    write(proj / "src" / "mod.py", MUT_SRC)
    write(proj / "src" / "broken.py", "def f(:\n")
    write(proj / "tests" / "test_mod.py", MUT_SRC)  # tests/ excluded
    sample = gates_mod.mutation_sample(proj)
    assert sample["candidates"] == 6
    assert [c["kind"] for c in sample["sample"]] == [
        "boolean-op", "comparison", "comparison",
        "boolean-return", "arithmetic", "negation"]
    limited = gates_mod.mutation_sample(proj, limit=2)
    assert len(limited["sample"]) == 2 and limited["candidates"] == 6
    again = gates_mod.mutation_sample(proj)
    assert again == sample  # deterministic
    report = gates_mod.run_gates(proj, selected=["mutation"])
    assert report["gates"]["mutation"]["status"] == "pass"


def test_quality_includes_new_gates(proj):
    report = gates_mod.run_gates(proj)
    assert report["quality"]["license"] == "pass"
    assert "candidates" in report["quality"]["mutation"]
