#!/usr/bin/env python3
"""Pending-approval discovery (stdlib-only).

Lists gates that still need a human decision: human-approval steps
without a recorded pass, and policy steps currently requiring approval
(or blocked by deny). Full plain-language cards (rationale + rejected
alternatives) need metadata the manifest doesn't store yet — this
module reports gate identity, kind, state, attempts, and policy
message; card enrichment is a schema follow-up.
"""

from pathlib import Path

from cli import manifest as manifest_mod
from cli import run as run_mod


def pending_approvals(project_dir):
    """Return pending gate entries. Raises ValueError without manifest/workflow.

    Entry: {"gate", "kind", "step", "state", "attempts", "message"} where
    state is awaiting|denied|blocked|error.
    """
    root = Path(project_dir)
    try:
        data = manifest_mod.load(root)
    except (FileNotFoundError, ValueError) as exc:
        raise ValueError("no manifest: %s (run shiploom init)" % exc)
    wf_path = run_mod.find_workflow(data.get("workflow") or "", root)
    if wf_path is None:
        raise ValueError("manifest workflow %r not found" % data.get("workflow"))
    fm, wf_errors = run_mod.load_workflow(wf_path)
    if fm is None:
        raise ValueError("; ".join(wf_errors))

    entries = []
    gates = data.get("gates", {})
    retries = data.get("retries", {})
    for step in fm.get("steps", []):
        if not isinstance(step, dict):
            continue
        sid = step.get("id")
        gate = step.get("gate") or "none"
        if gate == "human-approval":
            state = (gates.get(sid) or {}).get("state", "awaiting")
            if state != "passed":
                entries.append({"gate": sid, "kind": "human-approval", "step": sid,
                                "state": state if state in ("denied",) else "awaiting",
                                "attempts": retries.get(sid, 0), "message": ""})
        elif gate == "policy":
            from cli import policy as policy_mod
            try:
                decision, rule_id, message = policy_mod.check_step(
                    root, step.get("policy"), fm["name"], sid, "human",
                    action=step.get("action"), resource=step.get("resource"))
            except ValueError as exc:
                entries.append({"gate": sid, "kind": "policy", "step": sid,
                                "state": "error", "attempts": retries.get(sid, 0),
                                "message": str(exc)})
                continue
            gate_state = (gates.get(sid) or {}).get("state")
            if decision == "deny":
                entries.append({"gate": sid, "kind": "policy", "step": sid,
                                "state": "blocked",
                                "attempts": retries.get(sid, 0),
                                "message": "%s%s" % (rule_id + ": " if rule_id else "",
                                                     message)})
            elif decision == "require-approval" and gate_state != "passed":
                entries.append({"gate": sid, "kind": "policy", "step": sid,
                                "state": "denied" if gate_state == "denied" else "awaiting",
                                "attempts": retries.get(sid, 0), "message": message})
    return entries
