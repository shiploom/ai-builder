#!/usr/bin/env python3
"""JSON-policy evaluation + hook matching (stdlib-only, MASTER_SPEC 22).

Packs follow schemas/policy.schema.json (default-deny). Hooks follow
schemas/hook.schema.json. Decisions: allow | deny | require-approval.

Rule matching: action and resource support `*` globs (fnmatch).
First deny wins, else first require-approval, else first allow;
no match falls back to the pack's defaultEffect (deny when absent).
Optional per-rule `condition` maps context keys to required values
(equality; a missing key never matches).
"""

import fnmatch
import json
from pathlib import Path

TOOL_ROOT = Path(__file__).resolve().parent.parent
DEFAULT_PACK = TOOL_ROOT / "core" / "policies" / "default.json"
DEFAULT_HOOKS = TOOL_ROOT / "core" / "hooks" / "registry.json"

DECISIONS = ("allow", "deny", "require-approval")


def load_pack(path):
    """Load a policy pack file. Raises ValueError on bad JSON/shape."""
    try:
        doc = json.loads(Path(path).read_text(encoding="utf-8"))
    except (OSError, ValueError) as exc:
        raise ValueError("unreadable policy pack %s: %s" % (path, exc))
    if not isinstance(doc, dict) or not isinstance(doc.get("rules"), list):
        raise ValueError("policy pack %s misses rules[]" % path)
    return doc


def _rule_matches(rule, action, resource, context):
    if not isinstance(rule, dict):
        return False
    actions = rule.get("actions") or []
    resources = rule.get("resources") or []
    if not any(fnmatch.fnmatchcase(action, pattern) for pattern in actions):
        return False
    if not any(fnmatch.fnmatchcase(resource, pattern) for pattern in resources):
        return False
    condition = rule.get("condition") or {}
    if not isinstance(condition, dict):
        return False
    scope = dict(context or {})
    scope["action"] = action
    scope["resource"] = resource
    return all(scope.get(key) == value for key, value in condition.items())


def evaluate(pack_or_path, action, resource, context=None):
    """Evaluate a pack. Returns (decision, matched_rule_id|None, message)."""
    pack = load_pack(pack_or_path) if isinstance(pack_or_path, (str, Path)) else pack_or_path
    if not isinstance(pack, dict) or not isinstance(pack.get("rules"), list):
        raise ValueError("policy pack misses rules[]")
    hits = [rule for rule in pack["rules"]
            if _rule_matches(rule, action, resource, context)]
    # Deny beats require-approval beats allow; within an effect,
    # conditional (more specific) rules beat unconditional ones.
    ranked = sorted(enumerate(hits),
                    key=lambda pair: ({"deny": 0, "require-approval": 1,
                                       "allow": 2}.get(pair[1].get("effect"), 3),
                                      0 if pair[1].get("condition") else 1,
                                      pair[0]))
    for _, rule in ranked:
        effect = rule.get("effect")
        if effect in DECISIONS:
            return effect, rule.get("id"), rule.get("message", "")
    default = pack.get("defaultEffect", "deny")
    if default not in DECISIONS:
        default = "deny"
    return default, None, "no rule matched (default %s)" % default


def load_hooks(path=None):
    """Load a hooks registry (list of hook entries)."""
    target = Path(path) if path else DEFAULT_HOOKS
    try:
        doc = json.loads(target.read_text(encoding="utf-8"))
    except (OSError, ValueError) as exc:
        raise ValueError("unreadable hooks registry %s: %s" % (target, exc))
    if not isinstance(doc, list):
        raise ValueError("hooks registry %s must be a list" % target)
    return doc


def match_hooks(hooks, event, target):
    """Return registry entries whose event equals and matcher globs target."""
    matched = []
    for entry in hooks:
        if not isinstance(entry, dict):
            continue
        if entry.get("event") != event:
            continue
        if fnmatch.fnmatchcase(target, entry.get("matcher", "")):
            matched.append(entry)
    return matched


def check_step(project_dir, pack_ref, workflow, step_id, actor="human",
               action=None, resource=None):
    """Evaluate a workflow policy gate. Returns (decision, rule_id, message).

    pack_ref: project-relative pack path from the step, or None for the
    default pack. Steps declare the guarded `action`/`resource`
    (e.g. deploy.prod / prod); without them the step itself is evaluated
    (action workflow.step), which the default pack allows.
    """
    if pack_ref:
        pack_path = Path(project_dir) / pack_ref
        if not pack_path.is_file():
            raise ValueError("policy pack not found: %s" % pack_ref)
    else:
        pack_path = DEFAULT_PACK
    action = action or "workflow.step"
    resource = resource or "%s:%s" % (workflow, step_id)
    return evaluate(pack_path, action, resource,
                    {"actor": actor, "workflow": workflow, "step": step_id})
