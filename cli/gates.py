#!/usr/bin/env python3
"""Deterministic verification gates (stdlib-only, MASTER_SPEC 17 layers 2+4).

Gates are code, never LLM judgment. Each gate returns
{status: pass|fail|skip, command?, exit?, durationS, detail, tail?}.

Gate set (MVP):
  build/typecheck/lint/contract — project-configured commands only
    (unconfigured → skip). No auto-run of unknown commands.
  test — configured command, else auto-detected `pytest -q` when
    tests/ exists (npm/go runners: configured only, never auto).
  secrets — built-in stdlib secret scanner (always runs).
  depAudit — built-in dependency inventory + pin check, report-only
    (a real vuln DB lookup needs network; arrives post-MVP).
  compile — built-in `compileall` over project Python files (quality).

Quality table: compile result, secrets result, test result,
determinism (second test run when the first passes, bounded by the
same timeout), mutation (report-only note per 33-D2).
"""

import os
import re
import shlex
import subprocess
import sys
import time
from pathlib import Path

DEFAULT_TIMEOUT_S = 600
TAIL_LIMIT = 4000

SKIP_DIRS = {".git", ".venv", ".conda", "__pycache__", "node_modules",
             "dist", "build", ".oracle", ".validator-cache", "shiploom_core.egg-info"}

SECRET_RULES = [
    ("private-key", re.compile(r"-----BEGIN (?:RSA |OPENSSH |EC |DSA )?PRIVATE KEY-----")),
    ("aws-key", re.compile(r"AKIA[0-9A-Z]{16}")),
    ("token-prefix", re.compile(r"\b(?:ghp_|gho_|gsk_|sk-ant-|sk-proj-|xox[bpas]-)[A-Za-z0-9_-]{8,}")),
    ("secret-assign", re.compile(
        r"(?i)\b(?:secret|token|password|passwd|api[_-]?key|auth[_-]?token)\b"
        r"\s*[:=]\s*['\"][^'\"\s]{4,}['\"]")),
    ("conn-string", re.compile(
        r"(?i)\b(?:postgres|mysql|mongodb|redis)://[^/\s:]+:[^/\s@]+@[^\s]+")),
]


def _is_text(path):
    try:
        with open(path, "rb") as fh:
            return b"\x00" not in fh.read(8192)
    except OSError:
        return False


def scan_secrets(project_dir):
    """Scan text files for secret patterns. Findings never include values."""
    findings = []
    root = Path(project_dir)
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
        for fn in filenames:
            path = Path(dirpath) / fn
            if not _is_text(path):
                continue
            try:
                text = path.read_text(encoding="utf-8", errors="strict")
            except (OSError, ValueError, UnicodeError):
                continue
            for lineno, line in enumerate(text.splitlines(), 1):
                for rule, rx in SECRET_RULES:
                    if rx.search(line):
                        findings.append({"path": os.path.relpath(path, root),
                                         "line": lineno, "rule": rule})
                        break
    findings.sort(key=lambda f: (f["path"], f["line"]))
    return findings


def dep_inventory(project_dir):
    """Report-only dependency inventory: counts + unpinned warnings."""
    root = Path(project_dir)
    deps, unpinned = [], []
    req_files = sorted(root.glob("requirements*.txt")) + sorted(root.glob("requirements/*.txt"))
    for req in req_files:
        try:
            for line in req.read_text(encoding="utf-8").splitlines():
                line = line.strip()
                if not line or line.startswith("#") or line.startswith("-"):
                    continue
                name = re.split(r"[<>=!~\s\[]", line, maxsplit=1)[0].strip()
                if not name:
                    continue
                deps.append("pip:%s" % name)
                if "==" not in line:
                    unpinned.append("pip:%s" % name)
        except OSError:
            continue
    pkg = root / "package.json"
    if pkg.is_file():
        try:
            import json as _json
            doc = _json.loads(pkg.read_text(encoding="utf-8"))
            for section in ("dependencies", "devDependencies"):
                for name, spec in (doc.get(section) or {}).items():
                    deps.append("npm:%s" % name)
                    if not re.match(r"^[0-9]+\\.[0-9]+\\.[0-9]+$", str(spec).strip()):
                        unpinned.append("npm:%s" % name)
        except (OSError, ValueError):
            pass
    gomod = root / "go.mod"
    if gomod.is_file():
        try:
            for line in gomod.read_text(encoding="utf-8").splitlines():
                parts = line.strip().split()
                if len(parts) >= 2 and parts[0] not in ("module", "go", "require", ")", "//"):
                    deps.append("go:%s" % parts[0])
        except OSError:
            pass
    deps = sorted(set(deps))
    unpinned = sorted(set(unpinned))
    return {"deps": len(deps), "unpinned": unpinned}


def _run_command(command, project_dir, timeout_s):
    """Run a shell-less command with timeout. Returns result dict."""
    started = time.monotonic()
    try:
        proc = subprocess.run(shlex.split(command), cwd=str(project_dir),
                              capture_output=True, text=True, timeout=timeout_s)
        out = (proc.stdout or "") + (proc.stderr or "")
        return {"status": "pass" if proc.returncode == 0 else "fail",
                "command": command, "exit": proc.returncode,
                "durationS": round(time.monotonic() - started, 2),
                "detail": "exit %d" % proc.returncode,
                "tail": out[-TAIL_LIMIT:]}
    except FileNotFoundError:
        return {"status": "fail", "command": command, "exit": 127,
                "durationS": round(time.monotonic() - started, 2),
                "detail": "command not found", "tail": ""}
    except subprocess.TimeoutExpired as exc:
        out = ((exc.stdout or "") + (exc.stderr or "")) if isinstance(exc.stdout, str) else ""
        return {"status": "fail", "command": command, "exit": 124,
                "durationS": round(time.monotonic() - started, 2),
                "detail": "timeout after %ds" % timeout_s, "tail": out[-TAIL_LIMIT:]}


def _configured(project_dir):
    try:
        import json as _json
        config = _json.loads((Path(project_dir) / ".shiploom" / "config.json")
                             .read_text(encoding="utf-8"))
        gates = config.get("gates")
        return gates if isinstance(gates, dict) else {}
    except (OSError, ValueError):
        return {}


def _gate_command(gate_id, configured, project_dir):
    """Resolve the command for a configurable gate. Returns (command|None, reason)."""
    entry = configured.get(gate_id)
    if isinstance(entry, dict) and entry.get("command"):
        return entry["command"], "configured"
    if isinstance(entry, str):
        return entry, "configured"
    if gate_id == "test":
        if (Path(project_dir) / "tests").is_dir():
            return "%s -m pytest -q" % sys.executable, "auto-detected pytest"
        return None, "no tests/ directory"
    return None, "not configured"


def run_gates(project_dir, selected=None):
    """Run gates in fixed order. Returns gate report dict (verdict included)."""
    root = Path(project_dir)
    configured = _configured(root)
    order = ["build", "typecheck", "lint", "test", "contract", "secrets", "depAudit", "compile"]
    if selected:
        unknown = [g for g in selected if g not in order]
        if unknown:
            return {"ok": False, "gates": {}, "quality": {},
                    "warnings": [], "verdict": "fail",
                    "errors": ["unknown gates: %s" % ", ".join(unknown)]}
        order = [g for g in order if g in selected]
    gates, warnings = {}, {}
    test_cmd, test_timeout = None, DEFAULT_TIMEOUT_S

    for gate_id in order:
        if gate_id in ("build", "typecheck", "lint", "test", "contract"):
            command, reason = _gate_command(gate_id, configured, root)
            entry = configured.get(gate_id) or {}
            timeout_s = entry.get("timeoutS", DEFAULT_TIMEOUT_S) if isinstance(entry, dict) else DEFAULT_TIMEOUT_S
            if command is None:
                gates[gate_id] = {"status": "skip", "durationS": 0.0, "detail": reason}
                continue
            result = _run_command(command, root, timeout_s)
            gates[gate_id] = result
            if gate_id == "test":
                test_cmd, test_timeout = command, timeout_s
        elif gate_id == "secrets":
            started = time.monotonic()
            findings = scan_secrets(root)
            gates[gate_id] = {
                "status": "fail" if findings else "pass",
                "durationS": round(time.monotonic() - started, 2),
                "detail": "%d finding(s)" % len(findings) if findings else "clean",
                "findings": findings,
            }
        elif gate_id == "depAudit":
            started = time.monotonic()
            inv = dep_inventory(root)
            if inv["unpinned"]:
                warnings["depAudit"] = "unpinned: %s" % ", ".join(inv["unpinned"][:10])
            gates[gate_id] = {
                "status": "pass", "durationS": round(time.monotonic() - started, 2),
                "detail": "%d deps, %d unpinned (report-only; vuln DB lookup post-MVP)"
                          % (inv["deps"], len(inv["unpinned"])),
                "inventory": inv,
            }
        elif gate_id == "compile":
            result = _run_command(
                "%s -m compileall -q ." % sys.executable, root, DEFAULT_TIMEOUT_S)
            result["detail"] = ("all Python files compile"
                                if result["status"] == "pass" else result["detail"])
            result.pop("tail", None)
            gates[gate_id] = result

    quality = {
        "compile": gates.get("compile", {}).get("status", "skip"),
        "secrets": gates.get("secrets", {}).get("status", "skip"),
        "tests": gates.get("test", {}).get("status", "skip"),
        "mutation": "not-run (report-only sampling, MVP per 33-D2)",
    }
    if gates.get("test", {}).get("status") == "pass" and test_cmd:
        second = _run_command(test_cmd, root, test_timeout)
        quality["determinism"] = ("pass (2 identical runs)"
                                  if second["status"] == "pass"
                                  else "fail (flaky: second run %s)" % second["detail"])
        if second["status"] != "pass":
            gates["test"] = {"status": "fail", "command": test_cmd,
                             "durationS": second["durationS"],
                             "detail": "flaky: " + second["detail"],
                             "tail": second.get("tail", "")}
    else:
        quality["determinism"] = "not-run (no passing test gate)"

    verdict = "fail" if any(g.get("status") == "fail" for g in gates.values()) else "pass"
    return {"ok": verdict == "pass", "gates": gates, "quality": quality,
            "warnings": warnings, "verdict": verdict, "errors": []}


def verify_step_check(project_dir):
    """Deterministic half of a verify/regression-verify workflow step.

    Runs the full default gate set, then requires a schema-valid
    `verification/verification-report.json` twin with verdict pass whose
    criteria are all locked (when a lock exists). The qualitative
    re-derivation stays with the Verifier role; this function only
    enforces what code can enforce.

    Returns (ok, errors, warnings) with validator-style entries.
    """
    import json as _json
    from cli import manifest as _manifest
    from cli import oracle as _oracle
    from validators.validate import load_schema as _load_schema
    from validators.validate import validate_against_schema as _validate

    root = Path(project_dir)
    report = run_gates(root)
    failed = sorted(g for g, r in report["gates"].items() if r.get("status") == "fail")
    if failed:
        return False, [{"path": str(root),
                        "message": "gate failed: %s" % ", ".join(failed)}], []

    twin = root / "verification" / "verification-report.json"
    if not twin.is_file():
        return False, [{"path": "verification/verification-report.json",
                        "message": "verifier report twin missing"}], []
    # A lock broken after acceptance-lock completed must fail verification:
    # re-run the lock comparison (hash compares only, no side effects).
    lock_ok, lock_errors, _ = _oracle.check_lock(root)
    if not lock_ok and not any("no acceptance lock" in e.get("message", "")
                               for e in lock_errors):
        return False, [{"path": e.get("path", "."),
                        "message": "stale lock: %s" % e.get("message", "")}
                       for e in lock_errors[:3]], []
    try:
        doc = _json.loads(twin.read_text(encoding="utf-8"))
    except (OSError, ValueError) as exc:
        return False, [{"path": str(twin), "message": "invalid JSON: %s" % exc}], []
    schema_errors = _validate(doc, _load_schema("verification-report"), "$")
    if schema_errors:
        return False, [{"path": str(twin), "message": schema_errors[0]}], []
    if doc.get("verdict") != "pass":
        return False, [{"path": str(twin),
                        "message": "verdict is %r, not pass" % doc.get("verdict")}], []
    try:
        locked = ((_manifest.load(root).get("acceptance") or {}).get("criteria") or {})
    except (FileNotFoundError, ValueError):
        locked = {}
    if locked:
        unlocked = sorted({r.get("acceptanceId") for r in doc.get("results", [])
                           if r.get("acceptanceId") not in locked})
        if unlocked:
            return False, [{"path": str(twin),
                            "message": "unlocked criteria in report: %s" % ", ".join(unlocked)}], []
    else:
        oracle_files = _oracle.discover_acceptance(root)
        if oracle_files:
            return False, [{"path": str(root),
                            "message": "acceptance present but not locked"}], []
    return True, [], []
