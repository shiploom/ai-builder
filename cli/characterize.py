#!/usr/bin/env python3
"""Behavior-snapshot capture/diff for brownfield work (stdlib-only).

Characterization snapshots pin what code does TODAY (including bugs)
before any edit, so post-change diffs distinguish intended behavior
change from accidental regression. Report-only by design: changed
snapshots warn (review at merge-approval), tool errors fail.

Snapshots live in `./.shiploom/characterization/<name>.json`:
{name, command, exit, outputSha, tail, capturedAt, actor}.
"""

import difflib
import hashlib
import json
import re
from pathlib import Path

from cli import auditlog
from cli import manifest as manifest_mod

SNAPSHOT_DIRNAME = "characterization"
TAIL_LIMIT = 20000
DIFF_CONTEXT = 3
DIFF_MAX_LINES = 60
DEFAULT_TIMEOUT_S = 600

_NAME_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$")


def snapshot_dir(project_dir):
    return Path(project_dir) / ".shiploom" / SNAPSHOT_DIRNAME


def _check_name(name):
    if not isinstance(name, str) or not _NAME_RE.match(name):
        raise ValueError("bad snapshot name %r (letters/digits/._-, max 64)" % (name,))


def _resolve_command(project_dir, command, timeout_s):
    """Explicit command wins, else the configured/auto test command."""
    if command:
        return command, timeout_s
    from cli import gates as gates_mod
    configured = gates_mod._configured(Path(project_dir))
    resolved, reason = gates_mod._gate_command("test", configured, project_dir)
    if resolved is None:
        raise ValueError("no command: pass --command or configure gates.test (%s)" % reason)
    entry = configured.get("test") or {}
    timeout = entry.get("timeoutS", DEFAULT_TIMEOUT_S) if isinstance(entry, dict) else DEFAULT_TIMEOUT_S
    return resolved, timeout_s if timeout_s else timeout


def _run(project_dir, command, timeout_s):
    from cli import gates as gates_mod
    return gates_mod._run_command(command, project_dir, timeout_s)


def _sha(text):
    return "sha256:" + hashlib.sha256(text.encode("utf-8")).hexdigest()


def capture(project_dir, name, command=None, timeout_s=0, actor="human"):
    """Run command and store the snapshot. Returns the entry. Raises ValueError."""
    _check_name(name)
    root = Path(project_dir)
    command, timeout = _resolve_command(root, command, timeout_s or 0)
    timeout = timeout or DEFAULT_TIMEOUT_S
    result = _run(root, command, timeout)
    out = result.get("tail", "")
    entry = {"name": name, "command": command, "exit": result.get("exit"),
             "outputSha": _sha(out), "tail": out[-TAIL_LIMIT:],
             "timeoutS": timeout,
             "capturedAt": manifest_mod.utcnow(), "actor": actor}
    dest = snapshot_dir(root)
    dest.mkdir(parents=True, exist_ok=True)
    (dest / ("%s.json" % name)).write_text(
        json.dumps(entry, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    auditlog.append(root, actor=actor, action="characterize.capture", target=name)
    return entry


def _load(project_dir, name):
    _check_name(name)
    path = snapshot_dir(project_dir) / ("%s.json" % name)
    try:
        doc = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError) as exc:
        raise ValueError("no snapshot %r (capture first): %s" % (name, exc))
    if not isinstance(doc, dict) or "outputSha" not in doc:
        raise ValueError("corrupt snapshot %r (recapture)" % name)
    return doc


def list_snapshots(project_dir):
    """Sorted snapshot names present in the project."""
    dest = snapshot_dir(project_dir)
    if not dest.is_dir():
        return []
    return sorted(p.stem for p in dest.glob("*.json") if p.is_file())


def diff(project_dir, name, timeout_s=0):
    """Re-run a snapshot's command and compare. Never raises for run outcomes.

    Returns {"name", "changed", "exitChanged", "outputChanged",
             "unifiedDiff": [...], "current": {exit, outputSha}} or
    {"name", "error": ...} when the comparison itself cannot run.
    Changed behavior is data (warn), not failure: fixes legitimately
    change behavior; humans review diffs at merge-approval.
    """
    try:
        entry = _load(project_dir, name)
    except ValueError as exc:
        return {"name": name, "error": str(exc)}
    root = Path(project_dir)
    try:
        result = _run(root, entry["command"],
                      timeout_s or entry.get("timeoutS") or DEFAULT_TIMEOUT_S)
    except Exception as exc:  # never crash a report on runner faults
        return {"name": name, "error": "runner fault: %s" % exc}
    out = result.get("tail", "")
    exit_changed = result.get("exit") != entry.get("exit")
    output_changed = _sha(out) != entry.get("outputSha")
    unified = []
    if output_changed:
        old = (entry.get("tail") or "").splitlines()
        new = out.splitlines()
        unified = list(difflib.unified_diff(old, new, "before", "after",
                                            n=DIFF_CONTEXT))[:DIFF_MAX_LINES]
    try:
        auditlog.append(root, actor="system", action="characterize.diff", target=name)
    except (OSError, ValueError):
        pass  # reporting must not fail for audit faults; run logs its own trail
    return {"name": name, "changed": bool(exit_changed or output_changed),
            "exitChanged": bool(exit_changed), "outputChanged": bool(output_changed),
            "unifiedDiff": unified,
            "current": {"exit": result.get("exit"), "outputSha": _sha(out)}}
