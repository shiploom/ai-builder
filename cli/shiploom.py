#!/usr/bin/env python3
"""shiploom CLI (Python+uv MVP, stdlib argparse only).

Entry point: `shiploom` -> `cli.shiploom:main`.
Offline except for steps declaring `needs: [network|mcp:*]` (none in PR4).

Exit codes: 0 pass, 2 validation fail, 3 policy deny, 4 budget exceeded,
5 harness mismatch.
"""

import argparse
import json
import shutil
import sys
from pathlib import Path

TOOL_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(TOOL_ROOT))

from cli import adapters as adapters_mod  # noqa: E402
from cli import add as add_mod  # noqa: E402
from cli import approvals as approvals_mod  # noqa: E402
from cli import auditlog  # noqa: E402
from cli import doctor as doctor_mod  # noqa: E402
from cli import gates as gates_mod  # noqa: E402
from cli import manifest as manifest_mod  # noqa: E402
from cli import oracle as oracle_mod  # noqa: E402
from cli import run as run_mod  # noqa: E402
from validators import status as status_mod  # noqa: E402
from validators import trace as trace_mod  # noqa: E402
from validators import validate as validator  # noqa: E402

EXIT_OK = 0
EXIT_VALIDATION = 2
EXIT_POLICY = 3
EXIT_BUDGET = 4
EXIT_HARNESS = 5

SUPPORTED_HARNESSES = ("claude", "opencode", "auto")


def core_version():
    return (TOOL_ROOT / "core" / "VERSION").read_text(encoding="utf-8").strip()


def _fail(message, code=EXIT_VALIDATION):
    sys.stderr.write("shiploom: error: %s\n" % message)
    return code


# ---------------------------------------------------------------- install ---

def cmd_install(args):
    version = args.version or core_version()
    if version != core_version():
        return _fail("version %s != packaged core %s (offline MVP: no download)"
                     % (version, core_version()))
    dest = (Path.home() / ".shiploom" / "core" / version if args.to_global
            else Path.cwd() / ".shiploom" / "core" / version)

    # Verify the packaged core before copying (code is truth must hold).
    errors, warnings, _ = validator.validate_path(TOOL_ROOT / "core", strict=True)
    errors += validator.validate_path(TOOL_ROOT / "examples", strict=True)[0]
    if errors:
        sys.stderr.write("shiploom: packaged core fails strict validation:\n")
        for err in errors[:10]:
            sys.stderr.write("  %s: %s\n" % (err["path"], err["message"]))
        return EXIT_VALIDATION

    for sub in ("core", "schemas", "validators"):
        src = TOOL_ROOT / sub
        target = dest / sub
        shutil.copytree(src, target, dirs_exist_ok=True,
                        ignore=shutil.ignore_patterns("__pycache__"))
    receipt = {"version": version, "installedAt": manifest_mod.utcnow(),
               "source": str(TOOL_ROOT), "warnings": len(warnings)}
    dest.mkdir(parents=True, exist_ok=True)
    (dest / "receipt.json").write_text(json.dumps(receipt, indent=2, sort_keys=True) + "\n",
                                       encoding="utf-8")
    sys.stdout.write("installed shiploom core %s -> %s\n" % (version, dest))
    return EXIT_OK


# ------------------------------------------------------------------- init ---

def _seed_file(root, rel, template_rel, force):
    dest = Path(root) / rel
    if dest.exists() and not force:
        return "kept %s (exists)" % rel
    src = TOOL_ROOT / "core" / "artifacts-templates" / template_rel
    dest.parent.mkdir(parents=True, exist_ok=True)
    dest.write_bytes(src.read_bytes())
    return "seeded %s" % rel


def cmd_init(args):
    root = Path.cwd()
    dot = root / ".shiploom"
    if dot.exists() and not args.force:
        return _fail("%s exists (use --force to re-initialize)" % dot)
    green = not args.existing
    harness = args.harness
    if harness not in SUPPORTED_HARNESSES:
        return _fail("unsupported harness %r (choose claude|opencode|auto)"
                     % harness, EXIT_HARNESS)

    dot.mkdir(parents=True, exist_ok=True)
    oracle = dot / ".oracle"
    oracle.mkdir(parents=True, exist_ok=True)
    try:
        oracle.chmod(0o700)
    except OSError:
        pass  # non-POSIX filesystems: doctor reports actual mode
    (dot / ".gitignore").write_text(".oracle/\n", encoding="utf-8")

    version = core_version()
    workflow = manifest_mod.default_workflow(green)
    config = {
        "coreVersion": version,
        "harness": harness,
        "stack": args.stack,
        "workflow": workflow,
        "budgets": {"tokens": 800000, "spendUSD": 25.0, "wallClockH": 8.0},
        "policyPack": "default",
        "adapterTargets": (["claude", "opencode"] if harness == "auto" else [harness]),
        "mcpRegistry": "./.shiploom/mcp-registry.json",
    }
    (dot / "config.json").write_text(json.dumps(config, indent=2, sort_keys=True) + "\n",
                                     encoding="utf-8")
    (dot / "mcp-registry.json").write_text(
        json.dumps({"capabilities": {}}, indent=2, sort_keys=True) + "\n", encoding="utf-8")

    manifest_mod.save(root, manifest_mod.genesis(version, workflow))
    auditlog.init_log(root)
    auditlog.append(root, actor="system", action="project.init",
                    target=".", policy=config["policyPack"])

    if green:
        seed_note = _seed_file(root, "idea.md", "idea.md", args.force)
    else:
        seed_note = _seed_file(root, "brownfield/repo-map.md",
                               "brownfield/repo-map.md", args.force)

    sys.stdout.write("initialized %s project in %s\n" % (
        "greenfield" if green else "brownfield", root))
    sys.stdout.write("  workflow: %s\n  harness: %s\n  %s\n" % (workflow, harness, seed_note))
    sys.stdout.write("next: edit idea.md, then run `shiploom validate --strict .`\n")
    return EXIT_OK


# --------------------------------------------------------------- validate ---

def cmd_validate(args):
    errors, warnings, _ = validator.validate_path(args.path, strict=args.strict)
    json.dump({"ok": not errors, "errors": errors, "warnings": warnings},
              sys.stdout, indent=2, sort_keys=True)
    sys.stdout.write("\n")
    return EXIT_OK if not errors else EXIT_VALIDATION


# ----------------------------------------------------------------- status ---

def cmd_status(args):
    payload, error = status_mod.status_of(args.path)
    if error:
        json.dump({"ok": False, "error": error}, sys.stdout, indent=2, sort_keys=True)
        sys.stdout.write("\n")
        return EXIT_VALIDATION
    try:
        manifest = manifest_mod.load(args.path)
        payload["manifest"] = {
            "workflow": manifest.get("workflow"),
            "workflowVersion": manifest.get("workflowVersion"),
            "gates": manifest.get("gates", {}),
            "checkpoints": len(manifest.get("checkpoints", [])),
            "steps": {sid: st.get("state")
                      for sid, st in (manifest.get("steps") or {}).items()},
        }
        payload["budgets"] = manifest.get("budgets", payload["budgets"])
    except (FileNotFoundError, ValueError):
        pass  # file-only status (PR3 behavior) when no manifest exists
    if args.json:
        json.dump({"ok": True, **payload}, sys.stdout, indent=2, sort_keys=True)
        sys.stdout.write("\n")
    else:
        sys.stdout.write(status_mod.format_human(payload))
        if "manifest" in payload:
            steps = payload["manifest"].get("steps", {})
            done = sum(1 for s in steps.values() if s == "done")
            sys.stdout.write("workflow: %s  gates: %d  checkpoints: %d  steps: %d/%d done\n" % (
                payload["manifest"]["workflow"], len(payload["manifest"]["gates"]),
                payload["manifest"]["checkpoints"], done, len(steps)))
    return EXIT_OK


def _print_run_report(report):
    if report["advanced"]:
        sys.stdout.write("advanced: %s\n" % ", ".join(report["advanced"]))
    if report["completed"]:
        sys.stdout.write("workflow %s complete\n" % report["workflow"])
    elif report["paused"]:
        sys.stdout.write("paused: %s\n" % report["paused"])
    for err in report["errors"]:
        sys.stdout.write("  fail: %s\n" % err)


def cmd_run(args):
    try:
        budgets = run_mod.parse_budget_flags(args.budget)
    except ValueError as exc:
        return _fail(str(exc))
    code, report = run_mod.run_workflow(".", args.workflow, from_step=args.from_step,
                                        only=args.only_step, budget_overrides=budgets,
                                        actor=args.actor)
    if args.json:
        json.dump({"ok": code == EXIT_OK, "exit": code, **report},
                  sys.stdout, indent=2, sort_keys=True)
        sys.stdout.write("\n")
    else:
        _print_run_report(report)
    return code


def cmd_trace(args):
    trace, errors, warnings = trace_mod.build_trace(args.path)
    if errors:
        for err in errors:
            sys.stdout.write("  fail: %s: %s\n" % (err.get("path", "?"), err["message"]))
        return EXIT_VALIDATION
    if args.id not in trace:
        return _fail("unknown id %r in trace index" % args.id)
    links = trace[args.id]
    incoming = []
    for src in sorted(trace):
        for rel in sorted(trace[src]):
            if args.id in trace[src][rel]:
                incoming.append({"from": src, "relation": rel})
    if args.json:
        json.dump({"ok": True, "id": args.id, "links": links,
                   "referencedBy": incoming, "warnings": warnings},
                  sys.stdout, indent=2, sort_keys=True)
        sys.stdout.write("\n")
    else:
        sys.stdout.write("%s:\n" % args.id)
        for rel in sorted(links):
            sys.stdout.write("  %s: %s\n" % (rel, ", ".join(links[rel]) or "-"))
        sys.stdout.write("  referenced by:\n")
        if incoming:
            for ref in incoming:
                sys.stdout.write("    %s (%s)\n" % (ref["from"], ref["relation"]))
        else:
            sys.stdout.write("    -\n")
    return EXIT_OK


def cmd_budget(args):
    try:
        data = manifest_mod.load(args.path)
    except (FileNotFoundError, ValueError) as exc:
        return _fail("no manifest: %s (run shiploom init)" % exc)
    if args.set:
        try:
            overrides = run_mod.parse_budget_flags(args.set)
        except ValueError as exc:
            return _fail(str(exc))
        for key in overrides:
            if key not in ("tokens", "spendUSD", "wallClockH"):
                return _fail("unknown budget key %r" % key)
        budgets = data.setdefault("budgets", manifest_mod.default_budgets())
        for key, value in overrides.items():
            budgets.setdefault(key, {"limit": value, "used": 0})
            budgets[key]["limit"] = value
        manifest_mod.save(args.path, data)
        auditlog.append(args.path, actor=args.actor, action="budget.set",
                        target=",".join(sorted(overrides)))
    budgets = data.get("budgets", {})
    if args.json:
        json.dump({"ok": True, "budgets": budgets},
                  sys.stdout, indent=2, sort_keys=True)
        sys.stdout.write("\n")
    else:
        if not budgets:
            sys.stdout.write("no budgets tracked\n")
        for key in sorted(budgets):
            slot = budgets[key]
            sys.stdout.write("%s: %s/%s\n" % (key, slot.get("used"), slot.get("limit")))
    return EXIT_OK


def cmd_resume(args):
    try:
        data = manifest_mod.load(".")
    except (FileNotFoundError, ValueError) as exc:
        return _fail("no manifest: %s (run shiploom init)" % exc)
    workflow = data.get("workflow")
    if not workflow:
        return _fail("no workflow bound (run shiploom init or shiploom run <workflow>)")
    try:
        budgets = run_mod.parse_budget_flags(args.budget)
    except ValueError as exc:
        return _fail(str(exc))
    wf_path = run_mod.find_workflow(workflow, ".")
    order, pending_gates = [], []
    if wf_path is not None:
        fm, _ = run_mod.load_workflow(wf_path)
        if fm is not None:
            order = [s.get("id") for s in fm.get("steps", []) if isinstance(s, dict)]
    try:
        pending_gates = approvals_mod.pending_approvals(".")
    except ValueError as exc:
        return _fail(str(exc))
    states = data.get("steps", {})
    done = sum(1 for sid in order if states.get(sid, {}).get("state") == "done")
    position = {"workflow": workflow, "done": done, "total": len(order),
                "next": next((sid for sid in order
                              if states.get(sid, {}).get("state") != "done"), None),
                "pendingGates": [e["gate"] for e in pending_gates]}
    code, report = run_mod.run_workflow(".", workflow, budget_overrides=budgets,
                                        actor=args.actor)
    if args.json:
        json.dump({"ok": code == EXIT_OK, "exit": code, "position": position, **report},
                  sys.stdout, indent=2, sort_keys=True)
        sys.stdout.write("\n")
    else:
        sys.stdout.write("resume %s: %d/%d done, next: %s\n" % (
            workflow, done, len(order), position["next"] or "complete"))
        if position["pendingGates"]:
            sys.stdout.write("pending gates: %s\n" % ", ".join(position["pendingGates"]))
        _print_run_report(report)
    return code


def cmd_approvals(args):
    if args.interval <= 0:
        return _fail("--interval must be positive")
    try:
        entries = approvals_mod.pending_approvals(".")
    except ValueError as exc:
        return _fail(str(exc))
    if args.json:
        json.dump({"ok": True, "pending": entries},
                  sys.stdout, indent=2, sort_keys=True)
        sys.stdout.write("\n")
        return EXIT_OK
    def _show(entries):
        if not entries:
            sys.stdout.write("no pending approvals\n")
            return
        for entry in entries:
            sys.stdout.write("%-12s %-14s %-8s attempts=%d %s\n" % (
                entry["gate"], entry["kind"], entry["state"],
                entry["attempts"], entry["message"] or "awaiting human decision"))
        sys.stdout.write("note: full rationale cards need manifest rationale"
                         " (schema follow-up)\n")
    _show(entries)
    if args.watch and entries:
        import time
        try:
            while True:
                time.sleep(args.interval)
                try:
                    entries = approvals_mod.pending_approvals(".")
                except ValueError as exc:
                    return _fail(str(exc))
                _show(entries)
                if not entries:
                    return EXIT_OK
        except KeyboardInterrupt:
            return EXIT_OK
    return EXIT_OK


def cmd_add(args):
    try:
        dest, warnings = add_mod.install(args.kind, args.name, args.source, ".",
                                         tag=args.tag, force=args.force)
    except ValueError as exc:
        return _fail(str(exc))
    auditlog.append(".", actor=args.actor, action="content.add",
                    target="%s:%s" % (args.kind, args.name))
    if args.json:
        json.dump({"ok": True, "kind": args.kind, "name": args.name,
                   "dest": dest, "warnings": warnings},
                  sys.stdout, indent=2, sort_keys=True)
        sys.stdout.write("\n")
    else:
        sys.stdout.write("installed %s %s -> %s\n" % (args.kind, args.name, dest))
        for warn in warnings:
            sys.stdout.write("  warn: %s\n" % warn)
    return EXIT_OK


def cmd_verify(args):
    selected = [g.strip() for g in (args.gates.split(",") if args.gates else []) if g.strip()]
    report = gates_mod.run_gates(".", selected or None)
    if args.report:
        out = Path(".") / "verification" / "gate-report.json"
        out.parent.mkdir(parents=True, exist_ok=True)
        out.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    if args.json:
        json.dump(report, sys.stdout, indent=2, sort_keys=True)
        sys.stdout.write("\n")
    else:
        sys.stdout.write("verify: %s\n" % report["verdict"])
        for gate_id, result in report["gates"].items():
            sys.stdout.write("  [%-4s] %-10s %s\n"
                             % (result["status"].upper(), gate_id, result["detail"]))
        for key, value in report["quality"].items():
            sys.stdout.write("  quality %-12s %s\n" % (key, value))
        for err in report["errors"]:
            sys.stdout.write("  fail: %s\n" % err)
        if args.report:
            sys.stdout.write("  wrote verification/gate-report.json\n")
    return EXIT_OK if report["ok"] else EXIT_VALIDATION


def cmd_approve(args):
    try:
        data = manifest_mod.load(".")
    except (FileNotFoundError, ValueError) as exc:
        return _fail("no manifest: %s (run shiploom init)" % exc)
    wf_path = run_mod.find_workflow(data.get("workflow") or "", ".")
    if wf_path is None:
        return _fail("manifest workflow %r not found" % data.get("workflow"))
    fm, wf_errors = run_mod.load_workflow(wf_path)
    if fm is None:
        return _fail("; ".join(wf_errors))
    step = next((s for s in fm.get("steps", []) if s.get("id") == args.gate_id), None)
    if step is None:
        return _fail("unknown gate %r in workflow %s" % (args.gate_id, fm["name"]))
    if (step.get("gate") or "none") not in ("human-approval", "policy"):
        return _fail("gate %r is not an approvable gate (human-approval|policy)" % args.gate_id)
    if args.deny and not args.reason:
        return _fail("denying requires --reason")
    state = "denied" if args.deny else "passed"
    entry = {"state": state, "by": args.actor, "at": manifest_mod.utcnow()}
    if args.reason:
        entry["reason"] = args.reason
    data.setdefault("gates", {})[args.gate_id] = entry
    manifest_mod.save(".", data)
    auditlog.append(".", actor=args.actor,
                    action=("deny.%s" % args.gate_id if args.deny
                            else "approve.%s" % args.gate_id),
                    target=args.gate_id)
    if args.json:
        json.dump({"ok": True, "gate": args.gate_id, **entry},
                  sys.stdout, indent=2, sort_keys=True)
        sys.stdout.write("\n")
    else:
        sys.stdout.write("%s %s by %s\n" % (state, args.gate_id, args.actor))
    return EXIT_OK


# ----------------------------------------------------------------- doctor ---

def cmd_doctor(args):
    report = doctor_mod.run_checks(".")
    if args.json:
        json.dump({"ok": report["ok"], **report}, sys.stdout, indent=2, sort_keys=True)
        sys.stdout.write("\n")
    else:
        sys.stdout.write(doctor_mod.format_human(report))
    return EXIT_OK if report["ok"] else EXIT_HARNESS


# ------------------------------------------------------------------ audit ---

def cmd_audit(args):
    ok, errors = auditlog.verify(".")
    if args.export == "json":
        try:
            entries = auditlog.read_all(".")
        except (OSError, ValueError) as exc:
            return _fail(str(exc))
        json.dump({"ok": ok, "errors": errors, "entries": entries},
                  sys.stdout, indent=2, sort_keys=True)
        sys.stdout.write("\n")
    elif args.export == "md":
        try:
            sys.stdout.write(auditlog.export_md("."))
        except (OSError, ValueError) as exc:
            return _fail(str(exc))
    else:
        try:
            count = len(auditlog.read_all("."))
        except (OSError, ValueError) as exc:
            return _fail(str(exc))
        sys.stdout.write("audit: %d entries, chain %s\n" % (count, "ok" if ok else "BROKEN"))
        for err in errors:
            sys.stdout.write("  %s\n" % err)
    return EXIT_OK if ok else EXIT_VALIDATION


# ------------------------------------------------------------------ parser ---

def cmd_lock(args):
    if args.check:
        ok, errors, warnings = oracle_mod.check_lock(".")
        if args.json:
            json.dump({"ok": ok, "errors": errors, "warnings": warnings},
                      sys.stdout, indent=2, sort_keys=True)
            sys.stdout.write("\n")
        else:
            sys.stdout.write("acceptance lock: %s\n" % ("ok" if ok else "BROKEN"))
            for err in errors:
                sys.stdout.write("  fail: %s: %s\n" % (err["path"], err["message"]))
            for warn in warnings:
                sys.stdout.write("  warn: %s: %s\n" % (warn["path"], warn["message"]))
        return EXIT_OK if ok else EXIT_VALIDATION
    ok, errors, warnings, summary = oracle_mod.lock(".", actor=args.actor)
    if args.json:
        json.dump({"ok": ok, "errors": errors, "warnings": warnings, "summary": summary},
                  sys.stdout, indent=2, sort_keys=True)
        sys.stdout.write("\n")
    else:
        if ok:
            sys.stdout.write("locked %d criteria from %d files (by %s)\n"
                             % (summary["criteria"], summary["files"], args.actor))
        else:
            sys.stdout.write("lock failed:\n")
        for err in errors:
            sys.stdout.write("  fail: %s: %s\n" % (err["path"], err["message"]))
        for warn in warnings:
            sys.stdout.write("  warn: %s: %s\n" % (warn.get("path", "."), warn["message"]))
    return EXIT_OK if ok else EXIT_VALIDATION


def build_parser():
    parser = argparse.ArgumentParser(prog="shiploom", description="Shiploom Core CLI (offline-first).")
    sub = parser.add_subparsers(dest="command", required=True)

    install = sub.add_parser("install", help="copy + verify core (offline MVP)")
    install.add_argument("--global", dest="to_global", action="store_true",
                         help="install to ~/.shiploom/core/<version>/ (default)")
    install.add_argument("--local", dest="to_local", action="store_true",
                         help="install to ./.shiploom/core/<version>/")
    install.add_argument("--version", default=None, help="required core version")
    install.set_defaults(func=cmd_install)

    init = sub.add_parser("init", help="scaffold ./.shiploom/ + seed starter artifact")
    mode = init.add_mutually_exclusive_group()
    mode.add_argument("--green", action="store_true", help="greenfield (default)")
    mode.add_argument("--existing", action="store_true", help="brownfield")
    init.add_argument("--harness", default="auto", choices=SUPPORTED_HARNESSES)
    init.add_argument("--stack", default=None, help="stack hint, e.g. python|node (advisory only)")
    init.add_argument("--force", action="store_true", help="re-initialize over existing state")
    init.set_defaults(func=cmd_init)

    validate = sub.add_parser("validate", help="schemas + frontmatter + links")
    validate.add_argument("--strict", action="store_true")
    validate.add_argument("path", nargs="?", default=".")
    validate.set_defaults(func=cmd_validate)

    status = sub.add_parser("status", help="manifest + artifact states + budgets")
    status.add_argument("--json", action="store_true")
    status.add_argument("path", nargs="?", default=".")
    status.set_defaults(func=cmd_status)

    doctor = sub.add_parser("doctor", help="harness + MCP + toolchain compat (offline)")
    doctor.add_argument("--json", action="store_true")
    doctor.set_defaults(func=cmd_doctor)

    audit = sub.add_parser("audit", help="verify + export the audit log")
    audit.add_argument("--export", default=None, choices=("json", "md"))
    audit.set_defaults(func=cmd_audit)

    lock = sub.add_parser("lock", help="hash-lock acceptance + oracles (or --check)")
    lock.add_argument("--check", action="store_true", help="verify against the recorded lock")
    lock.add_argument("--actor", default="human", help="principal recorded in manifest/audit")
    lock.add_argument("--json", action="store_true")
    lock.set_defaults(func=cmd_lock)

    run = sub.add_parser("run", help="advance a workflow (idempotent stepper)")
    run.add_argument("workflow", help="workflow name (overlay shadows core)")
    run.add_argument("--from", dest="from_step", default=None, help="reset from STEP onward")
    run.add_argument("--only", dest="only_step", default=None, help="advance only STEP")
    run.add_argument("--resume", action="store_true", help="explicit resume (default behavior)")
    run.add_argument("--budget", action="append", default=[],
                     help="repeatable KEY=VALUE (tokens|spendUSD|wallClockH)")
    run.add_argument("--actor", default="human")
    run.add_argument("--json", action="store_true")
    run.set_defaults(func=cmd_run)

    approve = sub.add_parser("approve", help="record a human-approval gate (policy enforced by `run` gates; deny exits 3)")
    approve.add_argument("gate_id", help="gate id (= approval step id)")
    approve.add_argument("--deny", action="store_true")
    approve.add_argument("--reason", default=None)
    approve.add_argument("--actor", default="human")
    approve.add_argument("--json", action="store_true")
    approve.set_defaults(func=cmd_approve)

    verify = sub.add_parser("verify", help="run deterministic gates (+ quality table)")
    verify.add_argument("--report", action="store_true",
                        help="write verification/gate-report.json")
    verify.add_argument("--gates", default=None,
                        help="comma-separated subset (build,typecheck,lint,test,contract,secrets,depAudit,compile)")
    verify.add_argument("--json", action="store_true")
    verify.set_defaults(func=cmd_verify)

    adapters = sub.add_parser("adapters", help="list / generate harness adapters")
    adapters.add_argument("--list", action="store_true", help="list available adapters")
    adapters.add_argument("--generate", default=None, metavar="HARNESS",
                          help="harness name or 'all' (base|claude|opencode)")
    adapters.add_argument("--json", action="store_true")
    adapters.set_defaults(func=cmd_adapters)

    trace = sub.add_parser("trace", help="show trace subgraph for an artifact id")
    trace.add_argument("id", help="artifact id, e.g. REQ-001")
    trace.add_argument("path", nargs="?", default=".")
    trace.add_argument("--json", action="store_true")
    trace.set_defaults(func=cmd_trace)

    budget = sub.add_parser("budget", help="show or set manifest budget limits")
    budget.add_argument("--set", action="append", default=[],
                        help="repeatable KEY=VALUE (tokens|spendUSD|wallClockH)")
    budget.add_argument("--actor", default="human")
    budget.add_argument("--json", action="store_true")
    budget.add_argument("path", nargs="?", default=".")
    budget.set_defaults(func=cmd_budget)

    resume = sub.add_parser("resume", help="report position and advance the bound workflow")
    resume.add_argument("--budget", action="append", default=[],
                        help="repeatable KEY=VALUE (tokens|spendUSD|wallClockH)")
    resume.add_argument("--actor", default="human")
    resume.add_argument("--json", action="store_true")
    resume.set_defaults(func=cmd_resume)

    approvals = sub.add_parser("approvals", help="list pending human gates")
    approvals.add_argument("--watch", action="store_true",
                           help="poll until no gates remain (Ctrl-C stops)")
    approvals.add_argument("--interval", type=float, default=5.0,
                           help="poll seconds for --watch (default 5)")
    approvals.add_argument("--json", action="store_true",
                           help="single snapshot (implies no watch)")
    approvals.set_defaults(func=cmd_approvals)

    add = sub.add_parser("add", help="install a content pack into the project overlay")
    add.add_argument("kind", choices=sorted(add_mod.KINDS),
                     help="skill|workflow|hook|policy|adapter")
    add.add_argument("name", help="destination name (skills: must match SKILL.md name)")
    add.add_argument("--from", dest="source", required=True,
                     help="local path or git URL (URLs require --tag)")
    add.add_argument("--tag", default=None, help="semver tag for git sources")
    add.add_argument("--force", action="store_true", help="overwrite existing dest")
    add.add_argument("--actor", default="human")
    add.add_argument("--json", action="store_true")
    add.set_defaults(func=cmd_add)

    return parser


def cmd_adapters(args):
    if args.list or not args.generate:
        adapters = adapters_mod.list_adapters()
        if args.json:
            json.dump({"ok": True, "adapters": adapters},
                      sys.stdout, indent=2, sort_keys=True)
            sys.stdout.write("\n")
        else:
            for adapter in adapters:
                sys.stdout.write("%-8s v%-6s %s\n    outputs: %s\n" % (
                    adapter["adapter"], adapter["version"],
                    adapter["description"], ", ".join(adapter["outputs"]) or "none"))
        return EXIT_OK
    report, errors = adapters_mod.generate(args.generate, ".")
    if report is None:
        return _fail(errors[0] if errors else "adapter failed")
    if args.json:
        json.dump({"ok": not errors, "errors": errors, **report},
                  sys.stdout, indent=2, sort_keys=True)
        sys.stdout.write("\n")
    else:
        for key in ("created", "updated", "unchanged"):
            for rel in report[key]:
                sys.stdout.write("%-9s %s\n" % (key, rel))
        for err in errors:
            sys.stdout.write("  fail: %s\n" % err)
    return EXIT_OK if not errors else EXIT_VALIDATION


def main(argv=None):
    parser = build_parser()
    args = parser.parse_args(argv)
    if getattr(args, "to_local", False) and getattr(args, "to_global", False):
        return _fail("choose one of --global / --local")
    if not getattr(args, "to_local", False) and not getattr(args, "to_global", False):
        args.to_global = True  # install defaults to global
    try:
        return args.func(args)
    except BrokenPipeError:
        return EXIT_OK


if __name__ == "__main__":
    sys.exit(main())
