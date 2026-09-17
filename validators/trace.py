#!/usr/bin/env python3
"""Shiploom trace index builder (stdlib-only).

Builds the Requirement -> Decision -> Implementation -> Test ->
Verification Result index from artifact frontmatter links plus acceptance
and verification-report JSON ids (MASTER_SPEC 16, BUILD_PLAN PR3).

Usage:
    python3 validators/trace.py [--strict] [path] [--out trace.json]

Output (stdout, always JSON):
    {"ok": bool, "trace": {...}, "errors": [...], "warnings": [...]}

--out writes the trace map to FILE (parents created). Exit 0 pass, 2 fail.
No network. No other writes.
"""

import argparse
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from validators.validate import (  # noqa: E402
    _is_excluded,
    collect_files,
    load_schema,
    parse_frontmatter,
    validate_against_schema,
)

RELATIONS = ("requires", "decided_by", "implemented_by", "tested_by", "verified_by")


def empty_links():
    return {rel: [] for rel in RELATIONS}


def build_trace(target, strict=False):
    """Build and schema-check the trace index.

    Returns (trace, errors, warnings). `trace` maps artifact id ->
    {requires, decided_by, implemented_by, tested_by, verified_by}.
    """
    errors, warnings, trace = [], [], {}
    root = Path(target)
    base = root if root.is_dir() else root.parent
    files = collect_files(root)

    for path in files:
        if _is_excluded(path, base):
            continue
        if path.suffix == ".md":
            try:
                text = path.read_text(encoding="utf-8")
            except OSError as exc:
                errors.append({"path": str(path), "message": "unreadable: %s" % exc})
                continue
            fm, _, perr = parse_frontmatter(text)
            if fm is None:
                if perr:
                    errors.append({"path": str(path), "message": perr, "rule": "frontmatter"})
                continue
            if "id" not in fm or "kind" not in fm:
                continue
            aid = fm.get("id")
            if not isinstance(aid, str):
                continue
            if aid in trace:
                errors.append({"path": str(path),
                               "message": "duplicate artifact id %r" % aid,
                               "rule": "trace.duplicate"})
                continue
            links = empty_links()
            raw_links = fm.get("links") or {}
            if isinstance(raw_links, dict):
                for rel in RELATIONS:
                    vals = raw_links.get(rel, [])
                    links[rel] = list(vals) if isinstance(vals, list) else []
            trace[aid] = links
        elif path.suffix == ".json" and path.name != "trace.json":
            try:
                doc = json.loads(path.read_text(encoding="utf-8"))
            except (OSError, ValueError):
                continue  # JSON syntax is validate.py's job
            if isinstance(doc, dict) and "$schema" in doc:
                continue
            objs = doc if isinstance(doc, list) else [doc]
            for obj in objs:
                if not isinstance(obj, dict):
                    continue
                if "results" in obj and "verdict" in obj and isinstance(obj.get("id"), str):
                    rid = obj["id"]
                    if rid not in trace:
                        tested = [r.get("acceptanceId") for r in obj.get("results", [])
                                  if isinstance(r, dict) and isinstance(r.get("acceptanceId"), str)]
                        trace[rid] = dict(empty_links(), tested_by=tested)
                elif "howToVerify" in obj and isinstance(obj.get("id"), str):
                    if obj["id"] not in trace:
                        trace[obj["id"]] = empty_links()

    for msg in validate_against_schema(trace, load_schema("trace"), "$"):
        errors.append({"path": str(root), "message": msg, "rule": "trace.schema"})

    known = set(trace)
    for aid in sorted(trace):
        for rel in RELATIONS:
            for tgt in trace[aid][rel]:
                if tgt not in known:
                    msg = "%s: dangling link %s -> %r" % (aid, rel, tgt)
                    entry = {"path": str(root), "message": msg, "rule": "links.dangling"}
                    (errors if strict else warnings).append(entry)

    return trace, errors, warnings


def main(argv=None):
    parser = argparse.ArgumentParser(description="Shiploom trace index builder (stdlib-only).")
    parser.add_argument("path", nargs="?", default=".", help="file or directory (default: .)")
    parser.add_argument("--strict", action="store_true",
                        help="dangling links fail instead of warn")
    parser.add_argument("--out", default=None, help="write trace map to FILE")
    args = parser.parse_args(argv)

    if not Path(args.path).exists():
        result = {"ok": False, "trace": {},
                  "errors": [{"path": args.path, "message": "no such file or directory"}],
                  "warnings": []}
        json.dump(result, sys.stdout, indent=2, sort_keys=True)
        sys.stdout.write("\n")
        return 2

    trace, errors, warnings = build_trace(args.path, strict=args.strict)
    errors.sort(key=lambda e: (e["path"], e["message"]))
    warnings.sort(key=lambda e: (e.get("path", ""), e.get("message", "")))
    result = {"ok": not errors, "trace": trace, "errors": errors, "warnings": warnings}
    if args.out:
        out = Path(args.out)
        out.parent.mkdir(parents=True, exist_ok=True)
        out.write_text(json.dumps(trace, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    json.dump(result, sys.stdout, indent=2, sort_keys=True)
    sys.stdout.write("\n")
    return 0 if not errors else 2


if __name__ == "__main__":
    sys.exit(main())
