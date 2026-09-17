#!/usr/bin/env python3
"""Offline compatibility checks: harness + MCP + toolchain (stdlib-only).

`shiploom doctor` never touches the network. Statuses: pass / warn / fail.
Exit 0 when nothing fails, 5 (harness mismatch) otherwise.
"""

import json
import shutil
import sys
from pathlib import Path

TOOL_ROOT = Path(__file__).resolve().parent.parent
SCHEMA_FILES = [
    "artifact-frontmatter.schema.json",
    "acceptance.schema.json",
    "skill.schema.json",
    "workflow.schema.json",
    "hook.schema.json",
    "policy.schema.json",
    "mcp-registry.schema.json",
    "verification-report.schema.json",
    "trace-link.schema.json",
]


def _check(name, status, detail):
    return {"name": name, "status": status, "detail": detail}


def run_checks(project_dir="."):
    root = Path(project_dir)
    shiploom_dir = root / ".shiploom"
    checks = []

    # --- toolchain / runtime (fail = cannot run core at all) ---
    if sys.version_info >= (3, 9):
        checks.append(_check("python", "pass",
                             "Python %s" % ".".join(map(str, sys.version_info[:3]))))
    else:
        checks.append(_check("python", "fail",
                             "requires >=3.9, found %s" % ".".join(map(str, sys.version_info[:3]))))

    version_file = TOOL_ROOT / "core" / "VERSION"
    try:
        core_version = version_file.read_text(encoding="utf-8").strip()
        checks.append(_check("core-version", "pass", core_version))
    except OSError:
        core_version = None
        checks.append(_check("core-version", "fail", "unreadable %s" % version_file))

    missing, bad = [], []
    for fname in SCHEMA_FILES:
        path = TOOL_ROOT / "schemas" / fname
        try:
            doc = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, ValueError):
            bad.append(fname)
            continue
        if (doc.get("$schema") != "https://json-schema.org/draft/2020-12/schema"
                or not str(doc.get("$id", "")).startswith("https://shiploom.dev/schemas/")):
            bad.append(fname)
    if missing or bad:
        checks.append(_check("schemas", "fail", "bad: %s" % ", ".join(missing + bad)))
    else:
        checks.append(_check("schemas", "pass", "%d normative schemas" % len(SCHEMA_FILES)))

    try:
        import validators.validate  # noqa: F401
        import validators.trace  # noqa: F401
        import validators.status  # noqa: F401
        checks.append(_check("validators", "pass", "importable (stdlib-only)"))
    except Exception as exc:  # never crash doctor on import problems
        checks.append(_check("validators", "fail", "import failed: %s" % exc))

    # --- harnesses (warn-only: files generate without the CLI installed) ---
    for harness, binary in (("claude", "claude"), ("opencode", "opencode")):
        found = shutil.which(binary)
        checks.append(_check("harness-%s" % harness,
                             "pass" if found else "warn",
                             "CLI on PATH" if found else "CLI not on PATH"))

    # --- project (only when initialized: ./.shiploom/config.json exists) ---
    if (shiploom_dir / "config.json").is_file():
        config_path = shiploom_dir / "config.json"
        try:
            config = json.loads(config_path.read_text(encoding="utf-8"))
            if "coreVersion" in config and "harness" in config:
                checks.append(_check("project-config", "pass",
                                     "harness=%s" % config["harness"]))
            else:
                checks.append(_check("project-config", "fail",
                                     "missing coreVersion/harness keys"))
        except (OSError, ValueError) as exc:
            checks.append(_check("project-config", "fail", "unreadable: %s" % exc))

        manifest_path = shiploom_dir / "manifest.json"
        try:
            manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
            if core_version and manifest.get("coreVersion") != core_version:
                checks.append(_check("project-manifest", "fail",
                                     "core %s != tool core %s (run upgrade)"
                                     % (manifest.get("coreVersion"), core_version)))
            elif "workflow" in manifest and "budgets" in manifest:
                checks.append(_check("project-manifest", "pass",
                                     manifest.get("workflow", "")))
            else:
                checks.append(_check("project-manifest", "fail",
                                     "missing workflow/budgets keys"))
        except (OSError, ValueError) as exc:
            checks.append(_check("project-manifest", "fail", "unreadable: %s" % exc))

        try:
            sys.path.insert(0, str(TOOL_ROOT))
            from cli import auditlog
            ok, errors = auditlog.verify(root)
            checks.append(_check("project-audit",
                                 "pass" if ok else "fail",
                                 "%s" % ("chain ok" if ok else "; ".join(errors))))
        except Exception as exc:
            checks.append(_check("project-audit", "fail", "verify crashed: %s" % exc))

        oracle = shiploom_dir / ".oracle"
        if oracle.is_dir():
            mode = oracle.stat().st_mode & 0o777
            checks.append(_check("project-oracle",
                                 "pass" if mode == 0o700 else "warn",
                                 "mode %o%s" % (mode, "" if mode == 0o700 else " (want 700)")))
        else:
            checks.append(_check("project-oracle", "warn", "no .oracle/ yet (created by init)"))

        registry = shiploom_dir / "mcp-registry.json"
        if registry.exists():
            try:
                sys.path.insert(0, str(TOOL_ROOT))
                from validators.validate import load_schema, validate_against_schema
                doc = json.loads(registry.read_text(encoding="utf-8"))
                errs = validate_against_schema(doc, load_schema("mcp-registry"), "$")
                caps = len(doc.get("capabilities", {})) if not errs else 0
                if errs:
                    checks.append(_check("project-mcp-registry", "fail",
                                         "; ".join(errs[:3])))
                else:
                    from cli import mcp as mcp_mod
                    summary = mcp_mod.attestation_status(doc)
                    counts = summary["counts"]
                    detail = ("%d capabilities, attestation: %d attested / %d unverified / %d unattested"
                              % (caps, counts.get("attested", 0),
                                 counts.get("unverified", 0), counts.get("unattested", 0)))
                    risky = sorted(n for n, s in summary["servers"].items()
                                   if s in ("unverified", "unattested"))
                    if risky:
                        detail += " (unprovenanced: %s)" % ", ".join(risky[:5])
                        checks.append(_check("project-mcp-registry", "warn", detail))
                    else:
                        checks.append(_check("project-mcp-registry", "pass", detail))
            except (OSError, ValueError) as exc:
                checks.append(_check("project-mcp-registry", "fail", "unreadable: %s" % exc))
        else:
            checks.append(_check("project-mcp-registry", "warn", "none configured"))
    else:
        if shiploom_dir.is_dir():
            detail = ".shiploom/ holds an install, not a project (run shiploom init)"
        else:
            detail = "no ./.shiploom/ (run shiploom init)"
        checks.append(_check("project", "warn", detail))

    # --- toolchain (informational) ---
    checks.append(_check("toolchain-git",
                         "pass" if shutil.which("git") else "warn",
                         "on PATH" if shutil.which("git") else "not on PATH"))

    failures = sum(1 for c in checks if c["status"] == "fail")
    return {"ok": failures == 0, "checks": checks, "failures": failures,
            "warnings": sum(1 for c in checks if c["status"] == "warn")}


def format_human(report):
    lines = ["shiploom doctor: %s (%d fail, %d warn)"
             % ("OK" if report["ok"] else "PROBLEMS", report["failures"], report["warnings"])]
    for check in report["checks"]:
        lines.append("  [%-4s] %-20s %s" % (check["status"].upper(), check["name"], check["detail"]))
    return "\n".join(lines) + "\n"
