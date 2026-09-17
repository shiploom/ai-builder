#!/usr/bin/env python3
"""Shiploom read-only status reporter (stdlib-only, BUILD_PLAN PR3).

Reports artifact states + trace summary from files alone. Budgets and
gates are manifest-owned and arrive with the orchestrator in PR4.

Usage:
    python3 validators/status.py [path]        # human-readable
    python3 validators/status.py [path] --json # machine-readable

Exit 0 on success, 2 if the target does not exist.
"""

import argparse
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from validators.trace import build_trace  # noqa: E402
from validators.validate import _is_excluded, collect_files, parse_frontmatter  # noqa: E402

TOOL_ROOT = Path(__file__).resolve().parent.parent


def core_version():
    try:
        return (TOOL_ROOT / "core" / "VERSION").read_text(encoding="utf-8").strip()
    except OSError:
        return "unknown"


def status_of(target):
    """Collect read-only status. Returns (payload, error|None)."""
    root = Path(target)
    if not root.exists():
        return None, "no such file or directory: %s" % target
    base = root if root.is_dir() else root.parent
    artifacts, invalid = [], []
    for path in collect_files(root):
        if _is_excluded(path, base) or path.suffix != ".md":
            continue
        try:
            text = path.read_text(encoding="utf-8")
        except OSError as exc:
            invalid.append({"path": str(path), "message": "unreadable: %s" % exc})
            continue
        fm, _, perr = parse_frontmatter(text)
        if fm is None:
            if perr:
                invalid.append({"path": str(path), "message": perr})
            continue
        if "id" not in fm or "kind" not in fm:
            continue
        try:
            rel = str(path.relative_to(base))
        except ValueError:
            rel = str(path)
        artifacts.append({
            "path": rel,
            "id": fm.get("id"),
            "kind": fm.get("kind"),
            "title": fm.get("title"),
            "status": fm.get("status"),
            "owner": fm.get("owner"),
            "version": fm.get("version"),
        })

    artifacts.sort(key=lambda a: (str(a["id"]), a["path"]))
    by_status = {}
    for art in artifacts:
        by_status[str(art["status"])] = by_status.get(str(art["status"]), 0) + 1

    trace, trace_errors, _ = build_trace(target)
    edges = sum(len(trace[aid][rel]) for aid in trace for rel in trace[aid])

    return {
        "coreVersion": core_version(),
        "target": str(root),
        "artifacts": artifacts,
        "counts": {"total": len(artifacts), "byStatus": by_status,
                   "invalid": len(invalid)},
        "invalid": invalid,
        "trace": {"nodes": len(trace), "edges": edges,
                  "errors": [e["message"] for e in trace_errors]},
        "budgets": {"status": "unavailable",
                    "reason": "manifest-owned (merged by `shiploom status` when"
                              " `./.shiploom/manifest.json` exists)"},
    }, None


def format_human(payload):
    lines = ["shiploom status: %s (core %s)" % (payload["target"], payload["coreVersion"])]
    lines.append("artifacts: %d  trace nodes/edges: %d/%d" % (
        payload["counts"]["total"], payload["trace"]["nodes"], payload["trace"]["edges"]))
    by_status = payload["counts"]["byStatus"]
    lines.append("by status: " + (", ".join("%s=%d" % (k, by_status[k]) for k in sorted(by_status))
                                  or "none"))
    for art in payload["artifacts"]:
        lines.append("  %-10s %-12s %-10s %s" % (
            art["id"], str(art["kind"]), str(art["status"]), art["path"]))
    for inv in payload["invalid"]:
        lines.append("  INVALID %s: %s" % (inv["path"], inv["message"]))
    for msg in payload["trace"]["errors"]:
        lines.append("  TRACE ERROR: %s" % msg)
    budgets = payload.get("budgets") or {}
    if budgets.get("status") == "unavailable":
        lines.append(budgets.get("reason", "budgets unavailable"))
    else:
        parts = []
        for key in ("tokens", "spendUSD"):
            slot = budgets.get(key)
            if isinstance(slot, dict):
                parts.append("%s %s/%s" % (key, slot.get("used"), slot.get("limit")))
        lines.append("budgets: " + (", ".join(parts) or "none tracked"))
    return "\n".join(lines) + "\n"


def main(argv=None):
    parser = argparse.ArgumentParser(description="Shiploom status (read-only, offline).")
    parser.add_argument("path", nargs="?", default=".")
    parser.add_argument("--json", action="store_true")
    args = parser.parse_args(argv)

    payload, error = status_of(args.path)
    if error:
        json.dump({"ok": False, "error": error}, sys.stdout, indent=2, sort_keys=True)
        sys.stdout.write("\n")
        return 2
    if args.json:
        json.dump({"ok": True, **payload}, sys.stdout, indent=2, sort_keys=True)
        sys.stdout.write("\n")
    else:
        sys.stdout.write(format_human(payload))
    return 0


if __name__ == "__main__":
    sys.exit(main())
