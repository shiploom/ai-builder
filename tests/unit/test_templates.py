"""Template self-checks: every committed template must be strict-clean.

Code is truth — templates that fail our own validator would teach
users to ship failing artifacts.
"""

from pathlib import Path

from validators.validate import validate_path

TEMPLATES = Path(__file__).resolve().parents[2] / "core" / "artifacts-templates"


def _read(rel):
    return (TEMPLATES / rel).read_text(encoding="utf-8")


def test_templates_strict_clean_as_tree():
    errors, warnings, artifacts = validate_path(TEMPLATES, strict=True)
    assert errors == [], errors
    assert warnings == [], warnings
    assert len(artifacts) >= 15  # guard against accidental deletion


def test_templates_strict_clean_per_file():
    md_files = sorted(TEMPLATES.rglob("*.md"))
    json_files = sorted(TEMPLATES.rglob("*.json"))
    assert len(md_files) >= 15 and len(json_files) >= 1
    for path in md_files + json_files:
        errors, warnings, _ = validate_path(path, strict=True)
        # Dangling links are a tree property (siblings absent per-file);
        # the tree-level test above enforces them strictly.
        errors = [e for e in errors if e.get("rule") != "links.dangling"]
        assert errors == [], (path, errors)
        assert warnings == [], (path, warnings)


def test_research_evidence_table_mandatory():
    body = _read("research/evidence.md")
    for grade in ("Fact", "Inferred", "Opinion", "Marketing", "Conflicting", "Uncertain"):
        assert grade in body, grade
    assert "Source URL" in body


def test_competitor_teardown_grading():
    body = _read("research/competitors.md")
    assert "Marketing" in body and "Fact" in body


def test_adr_sections():
    body = _read("architecture/decisions.md")
    for section in ("## Context", "## Options", "## Chosen", "## Rejected", "## Consequences"):
        assert section in body, section


def test_requirements_anchor_acceptance_and_questions():
    body = _read("product/requirements.md")
    assert "Acceptance" in body
    assert "## Open questions" in body
    assert "## Success metrics" in body


def test_architecture_open_questions_gate():
    assert "## Open questions" in _read("architecture/architecture.md")


def test_verification_report_has_verdict_and_quality():
    body = _read("verification/verification-report.md")
    assert "## Verdict" in body
    assert "determinism" in body or "Test-quality" in body


def test_attempt_log_retry_bound():
    body = _read("implementation/attempt-log.md")
    assert "retry" in body.lower() and "replan" in body.lower()


def test_acceptance_example_measurable_and_relative_oracle():
    import json
    docs = json.loads(_read("acceptance/example.json"))
    assert len(docs) >= 2
    for doc in docs:
        assert len(doc["statement"]) >= 10
        assert doc["howToVerify"]["type"] == "script"
        assert len(doc["howToVerify"]["expect"]) >= 10
        assert not doc["oracleRef"].startswith("/") and ".." not in doc["oracleRef"]
