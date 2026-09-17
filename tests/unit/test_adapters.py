"""Hermetic unit tests for cli/adapters.py + adapters command (no network)."""

import ast
import json
import os
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))
from _stdlib import assert_stdlib_only

from cli import adapters as adapters_mod  # noqa: E402
from cli.shiploom import main  # noqa: E402
from validators.validate import parse_frontmatter, validate_path  # noqa: E402

REPO = Path(__file__).resolve().parents[2]
CORE_VERSION = (REPO / "core" / "VERSION").read_text().strip()
CORE_SKILLS = sorted(p.name for p in (REPO / "core" / "skills").iterdir() if p.is_dir())


@pytest.fixture()
def proj(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    return tmp_path


def test_list_adapters():
    adapters = adapters_mod.list_adapters()
    assert {a["adapter"] for a in adapters} == {"base", "claude", "opencode"}
    versions = {a["adapter"]: a["version"] for a in adapters}
    assert versions == {"base": "1.0.0", "claude": "1.1.0", "opencode": "1.1.0"}
    for adapter in adapters:
        assert adapter["description"]


def test_generate_base(proj):
    report, errors = adapters_mod.generate("base", proj)
    assert errors == [] and report["created"] == ["AGENTS.md"]
    text = (proj / "AGENTS.md").read_text(encoding="utf-8")
    assert "DO NOT EDIT" in text and CORE_VERSION in text
    assert proj.name in text and "{{" not in text
    report, errors = adapters_mod.generate("base", proj)
    assert errors == [] and report["unchanged"] == ["AGENTS.md"]
    (proj / "AGENTS.md").write_text("edited\n", encoding="utf-8")
    report, errors = adapters_mod.generate("base", proj)
    assert errors == [] and report["updated"] == ["AGENTS.md"]


def test_generate_claude_skills_mirror_core(proj):
    report, errors = adapters_mod.generate("claude", proj)
    assert errors == []
    generated = sorted((proj / ".claude" / "skills").iterdir())
    assert [p.name for p in generated] == CORE_SKILLS
    for skill in generated:
        gen_text = (skill / "SKILL.md").read_text(encoding="utf-8")
        src_text = (REPO / "core" / "skills" / skill.name / "SKILL.md").read_text()
        gen_fm, _, gen_err = parse_frontmatter(gen_text)
        src_fm, _, src_err = parse_frontmatter(src_text)
        assert gen_err is None and src_err is None
        assert gen_fm == src_fm  # frontmatter byte-identical in effect
        assert "DO NOT EDIT" in gen_text
        errors, warnings, _ = validate_path(skill / "SKILL.md", strict=True)
        assert errors == [] and warnings == []
    assert (proj / "CLAUDE.md").exists()


def test_generate_claude_settings_and_guard(proj):
    import json as _json
    import subprocess as _subprocess
    report, errors = adapters_mod.generate("claude", proj)
    assert errors == []
    settings = _json.loads((proj / ".claude" / "settings.json").read_text())
    groups = settings["hooks"]["PreToolUse"]
    assert any("shiploom-guard.py" in str(h.get("command", ""))
               for g in groups for h in g["hooks"])
    guard = proj / ".claude" / "hooks" / "shiploom-guard.py"
    assert guard.is_file()
    if os.name == "posix":
        assert guard.stat().st_mode & 0o111
    oracle_event = _json.dumps({"tool_name": "Edit",
                                "tool_input": {"file_path": ".shiploom/.oracle/x"}})
    proc = _subprocess.run([sys.executable, str(guard)], input=oracle_event,
                           capture_output=True, text=True, timeout=30)
    decision = _json.loads(proc.stdout)["hookSpecificOutput"]
    assert proc.returncode == 0 and decision["permissionDecision"] == "deny"
    benign_event = _json.dumps({"tool_name": "Edit",
                                "tool_input": {"file_path": "src/app.py"}})
    proc = _subprocess.run([sys.executable, str(guard)], input=benign_event,
                           capture_output=True, text=True, timeout=30)
    assert proc.returncode == 0 and not proc.stdout.strip()
    proc = _subprocess.run([sys.executable, str(guard)], input="not json",
                           capture_output=True, text=True, timeout=30)
    assert proc.returncode == 0 and not proc.stdout.strip()


def test_generate_opencode(proj):
    import json as _json
    report, errors = adapters_mod.generate("opencode", proj)
    assert errors == []
    assert sorted(p.name for p in (proj / ".opencode" / "skills").iterdir()) == CORE_SKILLS
    config = _json.loads((proj / "opencode.json").read_text(encoding="utf-8"))
    assert config["$schema"] == "https://opencode.ai/config.json"
    assert "AGENTS.md" in config["instructions"]
    assert config["permission"] == {"edit": "ask", "bash": "ask"}


def test_generate_unknown_adapter(proj):
    report, errors = adapters_mod.generate("cursor", proj)
    assert report is None and any("unknown adapter" in e for e in errors)


def test_generate_all_and_idempotent_bytes(proj):
    report, errors = adapters_mod.generate("all", proj)
    assert errors == [] and report["adapter"] == "all"
    before = {p: p.read_bytes() for p in sorted(proj.rglob("*")) if p.is_file()}
    report, errors = adapters_mod.generate("all", proj)
    assert errors == []
    assert report["created"] == []
    after = {p: p.read_bytes() for p in sorted(proj.rglob("*")) if p.is_file()}
    assert before == after


def test_workflow_var_from_config(proj, monkeypatch):
    monkeypatch.chdir(proj)
    assert main(["init", "--existing"]) == 0
    adapters_mod.generate("base", proj)
    assert "brownfield-fix" in (proj / "AGENTS.md").read_text(encoding="utf-8")


def test_adapters_cli(proj, capsys, monkeypatch):
    monkeypatch.chdir(proj)
    assert main(["adapters", "--list"]) == 0
    assert "claude" in capsys.readouterr().out
    assert main(["adapters", "--generate", "bogus"]) == 2
    capsys.readouterr()
    assert main(["adapters", "--generate", "base", "--json"]) == 0
    payload = json.loads(capsys.readouterr().out)
    assert payload["ok"] is True and payload["created"] == ["AGENTS.md"]


def test_adapters_module_stdlib_only():
    assert_stdlib_only(REPO / "cli" / "adapters.py")


def test_conformance_profiles_valid_json():
    for harness in ("base", "claude", "opencode"):
        doc = json.loads((REPO / "tests" / "conformance" / harness / "capabilities.json")
                         .read_text(encoding="utf-8"))
        assert doc["harness"] == harness and doc["mustProduce"]
