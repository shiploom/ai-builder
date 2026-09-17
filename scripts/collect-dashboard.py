#!/usr/bin/env python3
"""Collect cost/escape dashboard JSON from local project state (stdlib-only).

Reads ./.shiploom/manifest.json + audit.jsonl and verification/gate-report.json
(contract: docs/cost-dashboard.md). Pilot-gated fields (defect escapes, merge
stats, verifier catch rate) emit null with reasons — never invented numbers.

Usage: python3 scripts/collect-dashboard.py [project-dir] [--json]
Without --json prints a human summary. Exit 0 (missing inputs warn, not fail).
"""

import json
import sys
from pathlib import Path


def collect(project_dir):
    root = Path(project_dir)
    dot = root / ".shiploom"
    dashboard = {
        "workflow": None, "steps": {"done": 0, "total": 0}, "retries": {},
        "gates": {}, "budgets": {}, "auditEvents": 0,
        "gateReport": None, "defectEscapes": None, "mergeStats": None,
        "verifierCatchRate": None, "warnings": [],
    }
    try:
        manifest = json.loads((dot / "manifest.json").read_text(encoding="utf-8"))
    except (OSError, ValueError) as exc:
        dashboard["warnings"].append("no manifest: %s" % exc)
        return dashboard
    dashboard["workflow"] = manifest.get("workflow")
    steps = manifest.get("steps", {})
    dashboard["steps"] = {
        "done": sum(1 for s in steps.values() if s.get("state") == "done"),
        "total": len(steps),
    }
    dashboard["retries"] = manifest.get("retries", {})
    dashboard["gates"] = {k: v.get("state") for k, v in manifest.get("gates", {}).items()}
    dashboard["budgets"] = manifest.get("budgets", {})
    try:
        lines = (dot / "audit.jsonl").read_text(encoding="utf-8").splitlines()
        dashboard["auditEvents"] = sum(1 for line in lines if line.strip())
    except OSError as exc:
        dashboard["warnings"].append("no audit log: %s" % exc)
    report_path = root / "verification" / "gate-report.json"
    if report_path.is_file():
        try:
            report = json.loads(report_path.read_text(encoding="utf-8"))
            dashboard["gateReport"] = {
                "verdict": report.get("verdict"),
                "gates": {k: v.get("status") for k, v in report.get("gates", {}).items()},
            }
        except ValueError as exc:
            dashboard["warnings"].append("unreadable gate report: %s" % exc)
    else:
        dashboard["warnings"].append("no verification/gate-report.json (run shiploom verify --report)")
    dashboard["warnings"].append("defectEscapes: needs post-merge defect tracking (pilot)")
    dashboard["warnings"].append("mergeStats: needs maintainer-merge records (pilot)")
    dashboard["warnings"].append("verifierCatchRate: needs recorded verifier outcomes (pilot)")
    return dashboard


def main(argv=None):
    args = (argv if argv is not None else sys.argv)[1:]
    as_json = "--json" in args
    targets = [a for a in args if not a.startswith("-")]
    dashboard = collect(targets[0] if targets else ".")
    if as_json:
        json.dump(dashboard, sys.stdout, indent=2, sort_keys=True)
        sys.stdout.write("\n")
    else:
        sys.stdout.write("dashboard: workflow=%s steps=%d/%d audit events=%d\n" % (
            dashboard["workflow"], dashboard["steps"]["done"],
            dashboard["steps"]["total"], dashboard["auditEvents"]))
        ver = (dashboard["gateReport"] or {}).get("verdict", "no report")
        sys.stdout.write("gate verdict: %s\n" % ver)
        for warning in dashboard["warnings"]:
            sys.stdout.write("  note: %s\n" % warning)
    return 0


if __name__ == "__main__":
    sys.exit(main())
