"""Hermetic unit tests for validators/validate.py (no network, stdlib only)."""

import json
import subprocess
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))

from validators.validate import (  # noqa: E402
    VAGUE_TERMS,
    check_acceptance_semantics,
    check_trace_links,
    load_schema,
    parse_frontmatter,
    validate_against_schema,
    validate_path,
)

REPO = Path(__file__).resolve().parents[2]

ARTIFACT_FM = """\
---
id: REQ-001
kind: requirement
title: "User can reset password via email"
status: proposed
provenance:
  - type: human
    ref: "idea.md:3"
    confidence: high
    date: 2026-09-17
links:
  requires: []
  decided_by: []
  implemented_by: []
  tested_by: []
  verified_by: []
owner: specifier
version: 1
---

Body text.
"""


def write(path, content):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")
    return path


# --- schema subset evaluator ------------------------------------------------

@pytest.mark.parametrize("data,schema,ok", [
    ({"a": 1}, {"type": "object", "required": ["a"]}, True),
    ({"b": 1}, {"type": "object", "required": ["a"]}, False),
    ("x", {"type": "string", "minLength": 2}, False),
    ("xyz", {"type": "string", "maxLength": 2}, False),
    ("abc-123", {"type": "string", "pattern": "^[a-z]+-[0-9]+$"}, True),
    ("ABC", {"type": "string", "pattern": "^[a-z]+$"}, False),
    ("b", {"type": "string", "enum": ["a", "b"]}, True),
    ("c", {"type": "string", "enum": ["a", "b"]}, False),
    (True, {"type": "integer"}, False),   # bool is not integer
    (True, {"type": "boolean"}, True),
    (1.5, {"type": "number", "minimum": 2}, False),
    ([1], {"type": "array", "minItems": 2}, False),
    ({"x": 1}, {"type": "object", "properties": {"x": {"type": "string"}}}, False),
    ({"z": 1}, {"type": "object", "properties": {"a": {"type": "string"}},
                "additionalProperties": False}, False),
    ({"cap.x": {"ttlS": 1}}, {"type": "object",
     "patternProperties": {"^cap\\.": {"type": "object"}}}, True),
])
def test_schema_subset_table(data, schema, ok):
    errors = validate_against_schema(data, schema)
    assert (errors == []) == ok


def test_unknown_keywords_ignored():
    assert validate_against_schema(1, {"type": "integer", "default": 5, "format": "x"}) == []


def test_all_normative_schemas_load():
    for name in ["artifact-frontmatter", "acceptance", "skill", "workflow",
                 "hook", "policy", "mcp-registry", "verification-report", "trace"]:
        schema = load_schema(name)
        assert schema["$schema"] == "https://json-schema.org/draft/2020-12/schema"
        assert schema["$id"].startswith("https://shiploom.dev/schemas/")


# --- frontmatter parser ------------------------------------------------------

def test_no_frontmatter_skipped():
    fm, body, err = parse_frontmatter("# Just markdown\n\ntext\n")
    assert fm is None and err is None and "text" in body


def test_valid_artifact_frontmatter_parses():
    fm, body, err = parse_frontmatter(ARTIFACT_FM)
    assert err is None, err
    assert fm["id"] == "REQ-001"
    assert fm["provenance"][0]["confidence"] == "high"
    assert fm["links"]["requires"] == []
    assert fm["version"] == 1
    assert "Body text" in body


def test_unterminated_frontmatter_errors():
    _, _, err = parse_frontmatter("---\nid: REQ-001\n")
    assert err and "unterminated" in err


def test_tabs_rejected():
    _, _, err = parse_frontmatter("---\n\tid: REQ-001\n---\nbody\n")
    assert err and "tabs" in err


def test_unclosed_flow_list_errors():
    _, _, err = parse_frontmatter("---\nid: [unclosed\n---\nbody\n")
    assert err and "unclosed flow list" in err


def test_flow_list_and_quotes():
    fm, _, err = parse_frontmatter(
        "---\na: [X-001, 'Y-002']\nb: \"quoted\"\nc: 3\n---\nbody\n")
    assert err is None, err
    assert fm["a"] == ["X-001", "Y-002"] and fm["b"] == "quoted" and fm["c"] == 3


# --- end-to-end file validation ----------------------------------------------

def test_valid_artifact_tree_passes(tmp_path):
    write(tmp_path / "product" / "requirements.md", ARTIFACT_FM)
    errors, warnings, artifacts = validate_path(tmp_path)
    assert errors == [] and artifacts == {"REQ-001": str(tmp_path / "product" / "requirements.md")}


def test_artifact_missing_required_fails(tmp_path):
    bad = ARTIFACT_FM.replace("owner: specifier\n", "")
    write(tmp_path / "req.md", bad)
    errors, _, _ = validate_path(tmp_path)
    assert any("owner" in e["message"] for e in errors)


def test_duplicate_artifact_id_fails(tmp_path):
    write(tmp_path / "a.md", ARTIFACT_FM)
    write(tmp_path / "b.md", ARTIFACT_FM)
    errors, _, _ = validate_path(tmp_path)
    assert any("duplicate artifact id" in e["message"] for e in errors)


def test_readme_without_frontmatter_ignored(tmp_path):
    write(tmp_path / "README.md", "# Hi\n")
    errors, warnings, artifacts = validate_path(tmp_path)
    assert errors == [] and warnings == [] and artifacts == {}


def test_schema_docs_skipped(tmp_path):
    schema = json.loads((REPO / "schemas" / "policy.schema.json").read_text())
    write(tmp_path / "my-policy.schema.json", json.dumps(schema))
    errors, warnings, _ = validate_path(tmp_path)
    assert errors == [] and warnings == []


GOOD_ACC = {
    "id": "ACC-001",
    "statement": "Password reset email arrives within 60 seconds for valid accounts",
    "howToVerify": {"type": "script", "command": "pytest tests/e2e_reset.py",
                    "expect": "exit 0 within 60s"},
    "oracleRef": "oracle/ACC-001.sh",
}


def test_acceptance_valid_and_array_form(tmp_path):
    write(tmp_path / "acceptance" / "auth.json", json.dumps(GOOD_ACC))
    write(tmp_path / "acceptance" / "more.json", json.dumps([GOOD_ACC, dict(GOOD_ACC, id="ACC-002",
          oracleRef="oracle/ACC-002.sh")]))
    errors, _, _ = validate_path(tmp_path)
    assert errors == []


def test_acceptance_script_without_command_fails(tmp_path):
    bad = dict(GOOD_ACC, howToVerify={"type": "script", "expect": "works"})
    write(tmp_path / "acceptance" / "a.json", json.dumps(bad))
    errors, _, _ = validate_path(tmp_path)
    assert any("howToVerify.command" in e["message"] for e in errors)


def test_acceptance_absolute_oracle_fails(tmp_path):
    bad = dict(GOOD_ACC, oracleRef="/etc/oracle.sh")
    write(tmp_path / "acceptance" / "a.json", json.dumps(bad))
    errors, _, _ = validate_path(tmp_path)
    assert any("oracleRef" in e["message"] for e in errors)


def test_vague_acceptance_warns_then_strict_fails(tmp_path):
    vague = dict(GOOD_ACC, id="ACC-009",
                 statement="Login should be fast and secure",
                 howToVerify={"type": "human", "expect": "looks fine"})
    assert "fast" in VAGUE_TERMS
    write(tmp_path / "acceptance" / "a.json", json.dumps(vague))
    errors, warnings, _ = validate_path(tmp_path)
    assert errors == [] and any("vague" in w["message"] for w in warnings)
    errors, _, _ = validate_path(tmp_path, strict=True)
    assert any("vague" in e["message"] for e in errors)


def test_vague_with_measurable_verifier_only_warns_in_strict(tmp_path):
    ok_vague = dict(GOOD_ACC, statement="Cache makes dashboard fast under load")
    write(tmp_path / "acceptance" / "a.json", json.dumps(ok_vague))
    errors, warnings, _ = validate_path(tmp_path, strict=True)
    assert errors == []


def test_check_acceptance_semantics_direct():
    es, ws = check_acceptance_semantics(GOOD_ACC, strict=True)
    assert es == [] and ws == []


def test_skill_name_must_match_dir(tmp_path):
    write(tmp_path / "wrong-dir" / "SKILL.md",
          "---\nname: right-name\ndescription: Does a thing.\n---\n# Skill\n")
    errors, _, _ = validate_path(tmp_path)
    assert any("must equal directory name" in e["message"] for e in errors)


def test_skill_body_limit(tmp_path):
    body = "\n".join("line %d" % i for i in range(501))
    write(tmp_path / "big-skill" / "SKILL.md",
          "---\nname: big-skill\ndescription: Big.\n---\n" + body + "\n")
    errors, _, _ = validate_path(tmp_path)
    assert any("exceeds 500" in e["message"] for e in errors)


def test_workflow_retries_capped_and_dup_steps(tmp_path):
    wf = ("---\nname: demo\nversion: 1.0.0\nkind: sequential\nsteps:\n"
          "  - id: a\n    retries: 9\n  - id: a\n---\nBody\n")
    write(tmp_path / "workflows" / "demo.md", wf)
    errors, _, _ = validate_path(tmp_path)
    assert any("maximum" in e["message"] or "retries" in e["message"] for e in errors)
    assert any("duplicate step id" in e["message"] for e in errors)


def test_hook_run_required_for_run_script(tmp_path):
    write(tmp_path / "hooks" / "registry.json", json.dumps([
        {"event": "before_deploy", "matcher": "main", "action": "run-script",
         "scope": "project", "version": "1.0.0"}]))
    errors, _, _ = validate_path(tmp_path)
    assert any("requires 'run'" in e["message"] for e in errors)


def test_hook_bad_event_rejected(tmp_path):
    write(tmp_path / "hooks" / "registry.json", json.dumps(
        {"event": "on_launch", "matcher": "*", "action": "deny",
         "scope": "project", "version": "1.0.0"}))
    errors, _, _ = validate_path(tmp_path)
    assert any("enum" in e["message"] for e in errors)


def test_policy_allow_default_warns(tmp_path):
    write(tmp_path / "policies" / "p.json", json.dumps({
        "policyId": "team", "version": "1.0.0", "defaultEffect": "allow",
        "rules": [{"id": "r1", "effect": "allow", "actions": ["read"],
                   "resources": ["*"]}]}))
    errors, warnings, _ = validate_path(tmp_path)
    assert errors == [] and any("deny" in w["message"] for w in warnings)


def test_mcp_auth_must_be_ref(tmp_path):
    write(tmp_path / "mcp-registry.json", json.dumps({
        "capabilities": {"cap.web.search": {"providers": ["tavily"], "ttlS": 3600}},
        "servers": {"tavily": {"transport": "http", "version": "1.2.0",
                    "attested": True, "scopes": ["search:read"],
                    "auth": "RAW-SECRET-KEY"}}}))
    errors, _, _ = validate_path(tmp_path)
    assert any("pattern" in e["message"] for e in errors)


def test_dangling_link_warns_then_strict_errors(tmp_path):
    fm = ARTIFACT_FM.replace("requires: []", "requires: [REQ-999]")
    write(tmp_path / "req.md", fm)
    _, warnings, _ = validate_path(tmp_path)
    assert any("dangling link" in w["message"] for w in warnings)
    errors, _, _ = validate_path(tmp_path, strict=True)
    assert any("dangling link" in e["message"] for e in errors)


def test_trace_json_schema_validated(tmp_path):
    write(tmp_path / "trace.json", json.dumps({"REQ-001": {"bogus_rel": []}}))
    errors, _, _ = validate_path(tmp_path)
    assert any("trace.schema" in (e.get("rule") or "") or "unexpected property" in e["message"]
               for e in errors)


def test_check_trace_links_collects_json_ids(tmp_path):
    write(tmp_path / "acceptance" / "a.json", json.dumps(GOOD_ACC))
    files = [tmp_path / "acceptance" / "a.json"]
    known = check_trace_links({"REQ-001": "req.md"}, files, False, [], [])
    assert "REQ-001" in known  # artifact ids seed the known set
    assert "ACC-001" in known


def test_cli_exit_codes_and_json_shape(tmp_path):
    ok_dir = tmp_path / "ok"
    write(ok_dir / "req.md", ARTIFACT_FM)
    proc = subprocess.run([sys.executable, str(REPO / "validators" / "validate.py"), str(ok_dir)],
                          capture_output=True, text=True)
    assert proc.returncode == 0
    assert json.loads(proc.stdout)["ok"] is True

    bad_dir = tmp_path / "bad"
    write(bad_dir / "req.md", ARTIFACT_FM.replace("owner: specifier\n", ""))
    proc = subprocess.run([sys.executable, str(REPO / "validators" / "validate.py"), str(bad_dir)],
                          capture_output=True, text=True)
    assert proc.returncode == 2
    payload = json.loads(proc.stdout)
    assert payload["ok"] is False and payload["errors"]


def test_validator_stdlib_only():
    """validators/validate.py must import stdlib modules only (AGENTS.md)."""
    import ast
    tree = ast.parse((REPO / "validators" / "validate.py").read_text(encoding="utf-8"))
    imports = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            imports.update(a.name.split(".")[0] for a in node.names)
        elif isinstance(node, ast.ImportFrom) and node.module:
            imports.add(node.module.split(".")[0])
    stdlib = getattr(sys, "stdlib_module_names", None)
    if stdlib is None:  # Python 3.9 fallback
        stdlib = {"argparse", "json", "os", "re", "sys", "pathlib"}
    assert imports - set(stdlib) == set()
