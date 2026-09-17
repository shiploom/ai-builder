#!/usr/bin/env python3
"""Deterministic workflow orchestrator (stdlib-only, MASTER_SPEC 10/14).

`run` advances a declarative workflow as far as mechanics allow and
pauses when human/file input is needed. It is idempotent: re-running
never redoes verified steps. LLM work (`uses:`) is performed by the
harness; the orchestrator enforces order, inputs (`consumes`), outputs
(`produces`), gates, budgets, retries, and checkpoints around it.

Step completion requires, in order:
  1. `uses:` reference resolves (skill/role pack exists),
  2. `consumes` globs match (inputs present),
  3. `gate` passes (none / human-approval / verification),
  4. `produces` globs match and validate (non-strict).

Exit codes: 0 advanced-or-paused (see report), 2 validation/failure,
4 wall-clock budget exceeded.
"""

import fnmatch
import os
from pathlib import Path

from cli import auditlog
from cli import manifest as manifest_mod
from cli import oracle as oracle_mod
from validators.validate import load_schema, parse_frontmatter, validate_against_schema, validate_path

TOOL_ROOT = Path(__file__).resolve().parent.parent
CORE_WORKFLOWS = TOOL_ROOT / "core" / "workflows"

EXIT_OK = 0
EXIT_VALIDATION = 2
EXIT_POLICY = 3
EXIT_BUDGET = 4


def find_workflow(name, project_dir):
    """Overlay shadows core. Accepts bare names, 'name.md', or legacy
    'workflows/name' paths (stored by early init versions)."""
    base = name[:-3] if name.endswith(".md") else name
    if base.startswith("workflows/"):
        base = base[len("workflows/"):]
    candidates = [Path(project_dir) / ".shiploom" / "workflows" / (base + ".md"),
                  CORE_WORKFLOWS / (base + ".md")]
    for candidate in candidates:
        if candidate.is_file():
            return candidate
    return None


def load_workflow(path):
    """Parse + schema-validate a workflow file. Returns (fm, errors)."""
    try:
        text = Path(path).read_text(encoding="utf-8")
    except OSError as exc:
        return None, ["unreadable workflow %s: %s" % (path, exc)]
    fm, _, perr = parse_frontmatter(text)
    if perr:
        return None, [perr]
    if fm is None:
        return None, ["workflow %s has no frontmatter" % path]
    schema_errors = validate_against_schema(fm, load_schema("workflow"), "$")
    if schema_errors:
        return None, schema_errors
    return fm, []


def resolve_uses(ref, project_dir):
    """Resolve a `uses:` ref to a skill/role file. Returns Path|None."""
    candidates = [Path(project_dir) / ".shiploom" / ref,
                  TOOL_ROOT / "core" / ref]
    for candidate in candidates:
        if candidate.is_file():
            return candidate
        if candidate.is_dir() and (candidate / "SKILL.md").is_file():
            return candidate / "SKILL.md"
    return None


def _glob_hits(pattern, project_dir):
    root = Path(project_dir)
    if any(ch in pattern for ch in "*?["):
        return sorted(p for p in root.glob(pattern) if p.exists())
    candidate = root / pattern
    return [candidate] if candidate.exists() else []


def _outputs_hold(step, project_dir):
    """Re-check a done step's produces (existence + validity)."""
    for pattern in (step.get("produces") or []):
        hits = _glob_hits(pattern, project_dir)
        if not hits:
            return False
        for path in hits:
            if path.suffix not in (".md", ".json"):
                continue
            file_errors, _, _ = validate_path(path, strict=False)
            if file_errors:
                return False
    return True


def _gate_holds(step, manifest_data):
    """Re-check a done step's approval gate (still recorded as passed)."""
    if (step.get("gate") or "none") not in ("human-approval", "policy"):
        return True
    return ((manifest_data.get("gates", {}).get(step.get("id")) or {}).get("state")
            == "passed")


def _elapsed_h(manifest_data):
    started = manifest_data.get("startedAt")
    if not started:
        return 0.0
    try:
        from datetime import datetime, timezone
        start = datetime.strptime(started, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)
        now = datetime.now(timezone.utc)
        return max(0.0, (now - start).total_seconds() / 3600.0)
    except ValueError:
        return 0.0


def _check_verification_gate(step_id, project_dir):
    """Dispatch verification gates to registered checkers.

    Returns (ok|None, errors, warnings). None means no checker wired."""
    if step_id == "acceptance-lock":
        ok, errors, warnings = oracle_mod.check_lock(project_dir)
        return ok, errors, warnings
    if step_id in ("verify", "regression-verify"):
        from cli import gates as gates_mod
        return gates_mod.verify_step_check(project_dir)
    return None, [], []


def _pause(report, reason):
    report["paused"] = reason
    return report


def run_workflow(project_dir, workflow_name, from_step=None, only=None,
                 budget_overrides=None, actor="human"):
    """Advance the workflow. Returns (exit_code, report).

    report: {"workflow", "advanced": [step ids completed this run],
             "paused": reason|None, "completed": bool, "errors": []}.
    """
    report = {"workflow": workflow_name, "advanced": [], "paused": None,
              "completed": False, "errors": []}
    root = Path(project_dir)

    wf_path = find_workflow(workflow_name, root)
    if wf_path is None:
        report["errors"].append("unknown workflow %r" % workflow_name)
        return EXIT_VALIDATION, report
    fm, wf_errors = load_workflow(wf_path)
    if fm is None:
        report["errors"].extend(wf_errors)
        return EXIT_VALIDATION, report
    steps = fm.get("steps", [])
    step_ids = [s.get("id") for s in steps if isinstance(s, dict)]

    try:
        data = manifest_mod.load(root)
    except (FileNotFoundError, ValueError) as exc:
        report["errors"].append("no manifest: %s (run shiploom init)" % exc)
        return EXIT_VALIDATION, report

    # Adopt or guard the workflow binding.
    if data.get("workflow") != fm["name"]:
        progress = data.get("steps", {})
        if any(v.get("state") == "done" for v in progress.values()):
            report["errors"].append(
                "workflow %r in progress (manifest binds %r)"
                % (data.get("workflow"), fm["name"]))
            return EXIT_VALIDATION, report
        data["workflow"] = fm["name"]
        data["workflowVersion"] = fm.get("version", "1.0.0")
        data["steps"] = {}

    # Budgets: adopt wallClockH default, apply overrides.
    budgets = data.setdefault("budgets", manifest_mod.default_budgets())
    wf_budgets = fm.get("budgets") or {}
    if "wallClockH" in wf_budgets and "wallClockH" not in budgets:
        budgets["wallClockH"] = {"limit": wf_budgets["wallClockH"], "used": 0.0}
    for key, value in (budget_overrides or {}).items():
        if key not in ("tokens", "spendUSD", "wallClockH"):
            report["errors"].append("unknown budget key %r" % key)
            return EXIT_VALIDATION, report
        budgets.setdefault(key, {"limit": value, "used": 0})
        budgets[key]["limit"] = value
    if "startedAt" not in data:
        data["startedAt"] = manifest_mod.utcnow()

    def budget_exceeded():
        slot = budgets.get("wallClockH")
        if not slot:
            return False
        try:
            return _elapsed_h(data) > float(slot["limit"])
        except (TypeError, ValueError):
            return False

    if budget_exceeded():
        data["budgets"] = budgets
        manifest_mod.save(root, data)
        _pause(report, "wall-clock budget exceeded (exit 4)")
        return EXIT_BUDGET, report

    # Slice selection.
    if only is not None:
        if only not in step_ids:
            report["errors"].append("unknown step %r" % only)
            return EXIT_VALIDATION, report
        idx = step_ids.index(only)
        for prev in step_ids[:idx]:
            if data.get("steps", {}).get(prev, {}).get("state") != "done":
                report["errors"].append(
                    "predecessor %r incomplete (run without --only first)" % prev)
                return EXIT_VALIDATION, report
        targets = [only]
    else:
        targets = list(step_ids)
        if from_step is not None:
            if from_step not in step_ids:
                report["errors"].append("unknown step %r" % from_step)
                return EXIT_VALIDATION, report
            reset_from = step_ids.index(from_step)
            for sid in step_ids[reset_from:]:
                data.get("steps", {}).pop(sid, None)
                data.get("retries", {}).pop(sid, None)

    auditlog.append(root, actor=actor, action="run.start",
                    target="%s (targets: %s)" % (fm["name"], ",".join(targets)))

    for sid in targets:
        if budget_exceeded():
            manifest_mod.save(root, data)
            _pause(report, "wall-clock budget exceeded (exit 4)")
            auditlog.append(root, actor="system", action="run.paused",
                            target="budget exceeded")
            return EXIT_BUDGET, report
        step = next(s for s in steps if s.get("id") == sid)
        state = data.setdefault("steps", {}).setdefault(sid, {"state": "pending"})
        if state.get("state") == "done" and only is None:
            # Done is system state, not a stored claim: re-verify the
            # step's outputs and approval on every run so hand-edited
            # manifests cannot skip gates (spec 32.11).
            if not _outputs_hold(step, root) or not _gate_holds(step, data):
                state["state"] = "pending"
                auditlog.append(root, actor="system", action="run.reopened", target=sid)
            else:
                continue

        ref = step.get("uses")
        if ref and resolve_uses(ref, root) is None:
            report["errors"].append("step %r: unknown uses ref %r" % (sid, ref))
            return EXIT_VALIDATION, report

        missing_inputs = [c for c in (step.get("consumes") or [])
                          if not _glob_hits(c, root)]
        if missing_inputs:
            report["errors"].append("step %r: missing inputs: %s"
                                    % (sid, ", ".join(missing_inputs)))
            return EXIT_VALIDATION, report

        gate = step.get("gate", "none") or "none"
        if gate == "human-approval":
            gate_state = (data.get("gates", {}).get(sid) or {}).get("state")
            if gate_state == "passed":
                pass
            elif gate_state == "denied":
                report["errors"].append("step %r: gate denied (re-approve to unblock)" % sid)
                return EXIT_VALIDATION, report
            else:
                manifest_mod.save(root, data)
                _pause(report, "awaiting approval: %s (shiploom approve %s)" % (sid, sid))
                auditlog.append(root, actor="system", action="run.paused",
                                target="awaiting approval: %s" % sid)
                return EXIT_OK, report
        elif gate == "policy":
            from cli import policy as policy_mod
            try:
                decision, rule_id, message = policy_mod.check_step(
                    root, step.get("policy"), fm["name"], sid, actor,
                    action=step.get("action"), resource=step.get("resource"))
            except ValueError as exc:
                report["errors"].append("step %r: %s" % (sid, exc))
                return EXIT_VALIDATION, report
            if decision == "deny":
                auditlog.append(root, actor="system", action="run.policy.deny", target=sid)
                manifest_mod.save(root, data)
                report["errors"].append("step %r: policy deny%s" % (
                    sid, " (%s: %s)" % (rule_id, message) if rule_id else ""))
                return EXIT_POLICY, report
            if decision == "require-approval":
                gate_state = (data.get("gates", {}).get(sid) or {}).get("state")
                if gate_state == "passed":
                    pass
                elif gate_state == "denied":
                    report["errors"].append("step %r: gate denied (re-approve to unblock)" % sid)
                    return EXIT_VALIDATION, report
                else:
                    manifest_mod.save(root, data)
                    _pause(report, "policy requires approval: %s (shiploom approve %s)%s" % (
                        sid, sid, " [%s]" % message if message else ""))
                    auditlog.append(root, actor="system", action="run.paused",
                                    target="policy approval: %s" % sid)
                    return EXIT_OK, report
            # allow: proceed; fall through to produces check
        elif gate == "verification":
            outcome, verrs, _ = _check_verification_gate(sid, root)
            if outcome is None:
                manifest_mod.save(root, data)
                _pause(report, "no verification checker wired for %r (PR8)" % sid)
                return EXIT_OK, report
            if not outcome:
                if any("no acceptance lock" in e.get("message", "") for e in verrs):
                    # Nothing has been attempted yet: pause without
                    # consuming a retry or triggering replan.
                    manifest_mod.save(root, data)
                    _pause(report, "acceptance not locked yet "
                                   "(run shiploom lock, then re-run)")
                    return EXIT_OK, report
                attempts = data.setdefault("retries", {}).get(sid, 0) + 1
                data["retries"][sid] = attempts
                limit = step.get("retries", 0) or 0
                detail = "; ".join(e["message"] for e in verrs[:3])
                if attempts <= limit:
                    manifest_mod.save(root, data)
                    _pause(report, "verification failed on %r (attempt %d/%d): %s"
                                   % (sid, attempts, limit, detail))
                    auditlog.append(root, actor="system", action="run.retry",
                                    target="%s attempt %d" % (sid, attempts))
                    return EXIT_OK, report
                if (step.get("onFail") or "replan") == "abort":
                    report["errors"].append("step %r failed verification: %s" % (sid, detail))
                    return EXIT_VALIDATION, report
                for later in step_ids[step_ids.index(sid):]:
                    data.get("steps", {}).pop(later, None)
                data.get("retries", {}).pop(sid, None)
                manifest_mod.save(root, data)
                data["checkpoints"].append({"id": "cp-replan-%s" % sid,
                                           "at": manifest_mod.utcnow(),
                                           "manifestHash": "replan"})
                manifest_mod.save(root, data)
                _pause(report, "replan required after %d attempts on %r: %s"
                               % (attempts - 1, sid, detail))
                auditlog.append(root, actor="system", action="run.replan", target=sid)
                return EXIT_OK, report
            data.setdefault("retries", {}).pop(sid, None)

        missing_outputs = []
        produced_files = []
        for pattern in (step.get("produces") or []):
            hits = _glob_hits(pattern, root)
            if not hits:
                missing_outputs.append(pattern)
            else:
                produced_files.extend(hits)
        if missing_outputs:
            manifest_mod.save(root, data)
            if ref:
                resolved = resolve_uses(ref, root)
                hint = " (harness executes %s, then re-run)" % resolved.name
            else:
                hint = ""
            _pause(report, "step %r incomplete, missing outputs: %s%s"
                           % (sid, ", ".join(missing_outputs), hint))
            return EXIT_OK, report
        for path in produced_files:
            if path.suffix not in (".md", ".json"):
                continue
            file_errors, _, _ = validate_path(path, strict=False)
            if file_errors:
                manifest_mod.save(root, data)
                _pause(report, "step %r outputs fail validation: %s"
                               % (sid, file_errors[0]["message"]))
                return EXIT_OK, report

        state["state"] = "done"
        state["at"] = manifest_mod.utcnow()
        report["advanced"].append(sid)
        data["checkpoints"].append({"id": "cp-%s" % sid, "at": state["at"],
                                   "manifestHash": "pending"})
        manifest_mod.save(root, data)
        data["checkpoints"][-1]["manifestHash"] = manifest_mod.sha256_file(
            manifest_mod.manifest_path(root))
        manifest_mod.save(root, data)
        auditlog.append(root, actor=actor, action="run.step.done", target=sid)

    states = data.get("steps", {})
    if all(states.get(sid, {}).get("state") == "done" for sid in step_ids):
        report["completed"] = True
        auditlog.append(root, actor=actor, action="run.complete", target=fm["name"])
    manifest_mod.save(root, data)
    return EXIT_OK, report


def parse_budget_flags(flags):
    """Parse repeatable --budget KEY=VALUE into {key: number}."""
    out = {}
    for flag in flags or []:
        if "=" not in flag:
            raise ValueError("bad --budget %r (want KEY=VALUE)" % flag)
        key, _, raw = flag.partition("=")
        try:
            value = float(raw) if "." in raw else int(raw)
        except ValueError:
            raise ValueError("bad --budget %r (value must be numeric)" % flag)
        out[key.strip()] = value
    return out
