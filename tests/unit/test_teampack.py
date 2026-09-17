"""Hermetic unit tests for the team policy pack template (no network)."""

import json
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))

from cli import policy as policy_mod  # noqa: E402
from cli.shiploom import main  # noqa: E402

REPO = Path(__file__).resolve().parents[2]
PACK_SRC = REPO / "examples" / "team-policy-pack" / "team.json"


def write(path, content):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")
    return path


@pytest.fixture()
def proj(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    assert main(["init"]) == 0
    return tmp_path


def test_team_pack_strict_clean():
    from validators.validate import load_schema, validate_against_schema
    doc = json.loads(PACK_SRC.read_text(encoding="utf-8"))
    assert validate_against_schema(doc, load_schema("policy"), "$") == []


def test_actor_conditional_deploys():
    decision, rule, _ = policy_mod.check_step(
        ".", str(PACK_SRC), "w", "s",
        actor="agent", action="deploy.prod", resource="prod")
    assert (decision, rule) == ("require-approval", "approve-agent-deploy")
    decision, rule, _ = policy_mod.check_step(
        ".", str(PACK_SRC), "w", "s",
        actor="human", action="deploy.prod", resource="prod")
    assert (decision, rule) == ("allow", "allow-human-deploy")


def test_named_migration_deny():
    decision, rule, _ = policy_mod.check_step(
        ".", str(PACK_SRC), "w", "s",
        actor="human", action="db.migrate", resource="staging")
    assert (decision, rule) == ("deny", "deny-destructive")


def test_unknown_action_falls_to_default():
    decision, rule, _ = policy_mod.check_step(
        ".", str(PACK_SRC), "w", "s",
        actor="human", action="frobnicate.widgets", resource="dev")
    assert decision == "deny" and rule is None


def test_add_installs_to_overlay(proj, capsys):
    capsys.readouterr()
    assert main(["add", "policy", "team", "--from", str(PACK_SRC.parent)]) == 0
    dest = proj / ".shiploom" / "policies" / "team.json"
    assert dest.is_file()
    assert json.loads(dest.read_text())["policyId"] == "team-example"
    prov = json.loads((proj / ".shiploom" / "policies" / "_provenance.json")
                      .read_text())
    assert prov["team"]["signed"] is False
    assert "unsigned" in capsys.readouterr().out


TEAM_WF = ("---\nname: teamdep\nversion: 1.0.0\nkind: sequential\nresume: true\n"
           "budgets:\n  tokens: 1\n  spendUSD: 1\n  wallClockH: 1\nsteps:\n"
           "  - id: ship\n    consumes: [idea.md]\n    produces: [idea.md]\n"
           "    gate: policy\n    policy: .shiploom/policies/team.json\n"
           "    action: deploy.staging\n    resource: staging\n---\nBody\n")


def test_agent_pauses_human_proceeds(proj, capsys):
    assert main(["add", "policy", "team", "--from", str(PACK_SRC.parent)]) == 0
    write(proj / ".shiploom" / "workflows" / "teamdep.md", TEAM_WF)
    capsys.readouterr()
    assert main(["run", "teamdep", "--actor", "agent"]) == 0
    out = capsys.readouterr().out
    assert "policy requires approval: ship" in out
    assert "agent-driven deployments" in out
    assert main(["approve", "ship", "--actor", "human:priya"]) == 0
    capsys.readouterr()
    assert main(["run", "teamdep", "--actor", "agent"]) == 0
    assert "advanced: ship" in capsys.readouterr().out


def test_human_deploys_frictionless(proj, capsys):
    assert main(["add", "policy", "team", "--from", str(PACK_SRC.parent)]) == 0
    write(proj / ".shiploom" / "workflows" / "teamdep.md", TEAM_WF)
    capsys.readouterr()
    assert main(["run", "teamdep", "--actor", "human"]) == 0
    assert "advanced: ship" in capsys.readouterr().out
