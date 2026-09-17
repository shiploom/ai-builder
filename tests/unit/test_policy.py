"""Hermetic unit tests for cli/policy.py (no network, stdlib only)."""

import ast
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))
from _stdlib import assert_stdlib_only

from cli import policy as policy_mod  # noqa: E402

REPO = Path(__file__).resolve().parents[2]

PACK = {
    "policyId": "t", "version": "1.0.0", "defaultEffect": "deny",
    "rules": [
        {"id": "deny-prod-migrate", "effect": "deny",
         "actions": ["db.migrate-prod"], "resources": ["*"]},
        {"id": "approve-deploy", "effect": "require-approval",
         "actions": ["deploy.*"], "resources": ["*"]},
        {"id": "approve-human-prod", "effect": "require-approval",
         "actions": ["deploy.prod"], "resources": ["prod"],
         "condition": {"actor": "agent"}},
        {"id": "allow-reads", "effect": "allow",
         "actions": ["repo.*"], "resources": ["*"]},
    ],
}


def test_deny_wins_over_approval():
    decision, rule, _ = policy_mod.evaluate(PACK, "db.migrate-prod", "prod")
    assert (decision, rule) == ("deny", "deny-prod-migrate")


def test_glob_action_matches():
    decision, rule, _ = policy_mod.evaluate(PACK, "deploy.preview", "preview")
    assert (decision, rule) == ("require-approval", "approve-deploy")


def test_condition_selects_rule():
    decision, rule, _ = policy_mod.evaluate(PACK, "deploy.prod", "prod", {"actor": "agent"})
    assert rule == "approve-human-prod" and decision == "require-approval"
    decision, rule, _ = policy_mod.evaluate(PACK, "deploy.prod", "prod", {"actor": "human"})
    assert rule == "approve-deploy"  # condition misses, glob rule still hits


def test_allow_and_default_deny():
    assert policy_mod.evaluate(PACK, "repo.read", "x")[0] == "allow"
    decision, rule, message = policy_mod.evaluate(PACK, "unknown.thing", "x")
    assert decision == "deny" and rule is None and "default" in message


def test_default_pack_loads_and_denies_destructive():
    pack = policy_mod.load_pack(policy_mod.DEFAULT_PACK)
    assert pack["defaultEffect"] == "deny"
    for action in ("infra.destroy", "db.destroy", "secret.exfiltrate"):
        assert policy_mod.evaluate(pack, action, "prod")[0] == "deny"
    assert policy_mod.evaluate(pack, "deploy.prod", "prod")[0] == "require-approval"
    assert policy_mod.evaluate(pack, "repo.read", "repo")[0] == "allow"


def test_load_pack_rejects_garbage(tmp_path):
    bad = tmp_path / "p.json"
    bad.write_text("{nope", encoding="utf-8")
    with pytest.raises(ValueError):
        policy_mod.load_pack(bad)
    bad.write_text("[]", encoding="utf-8")
    with pytest.raises(ValueError):
        policy_mod.load_pack(bad)


def test_check_step_uses_and_defaults(tmp_path):
    decision, _, _ = policy_mod.check_step(tmp_path, None, "wf", "s",
                                           action="db.destroy", resource="prod")
    assert decision == "deny"
    decision, _, _ = policy_mod.check_step(tmp_path, None, "wf", "s")
    assert decision == "allow"  # bare workflow.step is a read-local action
    with pytest.raises(ValueError):
        policy_mod.check_step(tmp_path, "policies/missing.json", "wf", "s")


def test_match_hooks():
    hooks = policy_mod.load_hooks()
    assert hooks  # registry ships content
    hits = policy_mod.match_hooks(hooks, "before_file_write", ".shiploom/.oracle/x.sh")
    assert any(h["action"] == "deny" for h in hits)
    assert policy_mod.match_hooks(hooks, "before_merge", "main")
    assert policy_mod.match_hooks(hooks, "before_merge", "feature/x") == []
    assert policy_mod.match_hooks(hooks, "nope", "*") == []


def test_policy_module_stdlib_only():
    assert_stdlib_only(REPO / "cli" / "policy.py")
