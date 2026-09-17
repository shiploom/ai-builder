#!/usr/bin/env python3
"""Deterministic harness-conformance runner (stdlib-only).

Checks each harness adapter against its `tests/conformance/<harness>/`
profile: generate into a scratch dir, then assert must-produce files
exist, generated skills validate, frontmatter matches core, and
harness-specific contracts hold (settings.json shape + live guard
behavior for Claude; opencode.json shape for OpenCode).

Recorded-transcript mode is the default (no LLM calls, fully
deterministic). `--record` saves the JSON report under
`tests/conformance/_records/` (gitignored, for local drift diffing).
The nightly live-LLM lane needs model credentials and is out of scope.
"""

import json
import subprocess
import sys
import tempfile
from pathlib import Path

TOOL_ROOT = Path(__file__).resolve().parent.parent
PROFILES_DIR = TOOL_ROOT / "tests" / "conformance"
RECORDS_DIR = PROFILES_DIR / "_records"

ALL_HARNESSES = ("base", "claude", "opencode")


def _check(name, ok, detail):
    return {"name": name, "status": "pass" if ok else "fail", "detail": detail}


def _skill_names():
    skills = TOOL_ROOT / "core" / "skills"
    return sorted(p.name for p in skills.iterdir() if p.is_dir())


def _check_tree(harness, root):
    """Run profile checks against an already-generated tree. Returns checks[]."""
    from cli import adapters as adapters_mod
    from validators.validate import parse_frontmatter, validate_path

    checks = []
    profile_path = PROFILES_DIR / harness / "capabilities.json"
    try:
        profile = json.loads(profile_path.read_text(encoding="utf-8"))
        checks.append(_check("profile-loads", True, str(profile_path)))
    except (OSError, ValueError) as exc:
        return [_check("profile-loads", False, str(exc))]

    mapping = next((a for a in adapters_mod.list_adapters() if a["adapter"] == harness), None)
    if mapping is None:
        return checks + [_check("adapter-registered", False, harness)]
    checks.append(_check("adapter-registered", True, "v%s" % mapping["version"]))
    if mapping["version"] != profile.get("version"):
        checks.append(_check("adapter-profile-version",
                             False, "adapter v%s != profile v%s"
                             % (mapping["version"], profile.get("version"))))
    else:
        checks.append(_check("adapter-profile-version", True,
                             "v%s" % mapping["version"]))

    skills_dirs = {"claude": ".claude/skills", "opencode": ".opencode/skills"}
    if harness in skills_dirs:
        generated = sorted((root / skills_dirs[harness]).glob("*/SKILL.md")) \
            if (root / skills_dirs[harness]).is_dir() else []
        names = [p.parent.name for p in generated]
        checks.append(_check("skills-mirror-core", names == _skill_names(),
                             "%d skills" % len(names)))
        bad = 0
        for path in generated:
            errors, warnings, _ = validate_path(path, strict=True)
            gen_fm, _, gen_err = parse_frontmatter(path.read_text(encoding="utf-8"))
            src = TOOL_ROOT / "core" / "skills" / path.parent.name / "SKILL.md"
            src_fm, _, src_err = parse_frontmatter(src.read_text(encoding="utf-8"))
            if errors or warnings or gen_err or src_err or gen_fm != src_fm:
                bad += 1
        checks.append(_check("skills-validate-clean", bad == 0,
                             "%d bad" % bad if bad else "all strict-clean"))

    if harness == "base":
        agents = root / "AGENTS.md"
        if not agents.is_file():
            checks.append(_check("agents-md", False, "missing AGENTS.md"))
        else:
            fm, _, _ = parse_frontmatter(agents.read_text(encoding="utf-8"))
            checks.append(_check("agents-md", fm is None and "DO NOT EDIT" in
                                 agents.read_text(encoding="utf-8"),
                                 "plain Markdown with header"))

    if harness == "claude":
        settings_path = root / ".claude" / "settings.json"
        try:
            settings = json.loads(settings_path.read_text(encoding="utf-8"))
            groups = settings.get("hooks", {}).get("PreToolUse", [])
            has_guard = any("shiploom-guard.py" in str(h.get("command", ""))
                            for g in groups for h in g.get("hooks", []))
            checks.append(_check("settings-shape", isinstance(groups, list) and has_guard,
                                 "PreToolUse guard wired"))
        except (OSError, ValueError) as exc:
            checks.append(_check("settings-shape", False, str(exc)))
        guard = root / ".claude" / "hooks" / "shiploom-guard.py"
        if not guard.is_file():
            checks.append(_check("guard-behavior", False, "missing guard script"))
        else:
            checks.append(_check("guard-behavior", _guard_denies(guard, True)
                                 and _guard_denies(guard, False),
                                 "denies oracle writes, silent otherwise"))

    if harness == "opencode":
        config_path = root / "opencode.json"
        try:
            config = json.loads(config_path.read_text(encoding="utf-8"))
            ok = (config.get("$schema") == "https://opencode.ai/config.json"
                  and "AGENTS.md" in (config.get("instructions") or [])
                  and (config.get("permission") or {}).get("edit") == "ask")
            checks.append(_check("config-shape", ok, "instructions + permission defaults"))
        except (OSError, ValueError) as exc:
            checks.append(_check("config-shape", False, str(exc)))

    return checks


def _guard_denies(guard, oracle_case):
    """Run the generated guard with a fixture event. oracle_case=True must
    yield a deny decision; False must stay silent with exit 0."""
    event = {"tool_name": "Edit", "tool_input": {
        "file_path": ".shiploom/.oracle/x" if oracle_case else "src/app.py"}}
    try:
        proc = subprocess.run([sys.executable, str(guard)], input=json.dumps(event),
                              capture_output=True, text=True, timeout=30)
    except (OSError, subprocess.SubprocessError):
        return False
    if oracle_case:
        try:
            decision = json.loads(proc.stdout or "{}")
            return (proc.returncode == 0 and decision.get("hookSpecificOutput", {})
                    .get("permissionDecision") == "deny")
        except ValueError:
            return False
    return proc.returncode == 0 and not (proc.stdout or "").strip()


def check_harness(harness):
    """Generate into scratch and check. Returns (ok, report)."""
    from cli import adapters as adapters_mod

    if harness not in ALL_HARNESSES:
        return False, {"harness": harness, "ok": False,
                       "errors": ["unknown harness %r" % harness]}
    with tempfile.TemporaryDirectory(prefix="shiploom-conf-") as tmp:
        report, errors = adapters_mod.generate(harness, tmp)
        if errors:
            return False, {"harness": harness, "ok": False, "errors": errors}
        checks = _check_tree(harness, Path(tmp))
    failures = sum(1 for c in checks if c["status"] == "fail")
    return failures == 0, {"harness": harness, "ok": failures == 0,
                           "checks": checks, "failures": failures}


def run_all(record=False):
    """Check every harness. Returns (ok, results)."""
    results = {}
    for harness in ALL_HARNESSES:
        ok, report = check_harness(harness)
        results[harness] = report
        if record:
            RECORDS_DIR.mkdir(parents=True, exist_ok=True)
            (RECORDS_DIR / ("%s.json" % harness)).write_text(
                json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    return all(r["ok"] for r in results.values()), results
