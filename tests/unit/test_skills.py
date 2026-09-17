"""Content contracts for roles and skills (no network, hermetic)."""

from pathlib import Path

from validators.validate import parse_frontmatter, validate_path

CORE = Path(__file__).resolve().parents[2] / "core"

EXPECTED_SKILLS = ["idea-shaping", "product-definition", "architecture-design",
                   "market-research", "competitor-teardown", "spec-to-acceptance",
                   "implement-scoped-diff", "verify-independent",
                   "brownfield-map", "impact-analysis"]
SKILL_SECTIONS = ["## Purpose", "## Inputs", "## Outputs", "## Prerequisites",
                  "## Methodology", "## Constraints", "## Tools",
                  "## Verification", "## Examples"]
ROLE_SECTIONS = ["## Objective", "## Allowed", "## Forbidden",
                 "## Context budget", "## Output contract", "## Escalation"]


def test_expected_skills_present():
    found = sorted(p.parent.name for p in (CORE / "skills").rglob("SKILL.md"))
    for name in EXPECTED_SKILLS:
        assert name in found, name


def test_skills_validate_clean():
    for name in EXPECTED_SKILLS:
        path = CORE / "skills" / name / "SKILL.md"
        errors, warnings, _ = validate_path(path, strict=True)
        assert errors == [], (name, errors)
        assert warnings == [], (name, warnings)


def test_skill_name_matches_directory():
    for path in (CORE / "skills").rglob("SKILL.md"):
        fm, _, err = parse_frontmatter(path.read_text(encoding="utf-8"))
        assert err is None, (path, err)
        assert fm["name"] == path.parent.name, path


def test_skill_required_sections():
    for name in EXPECTED_SKILLS:
        body = (CORE / "skills" / name / "SKILL.md").read_text(encoding="utf-8")
        for section in SKILL_SECTIONS:
            assert section in body, (name, section)


def test_skill_base_spec_pure():
    """MVP skills run unmodified on all harnesses: no harness extras."""
    for name in EXPECTED_SKILLS:
        fm, _, _ = parse_frontmatter(
            (CORE / "skills" / name / "SKILL.md").read_text(encoding="utf-8"))
        assert "x-shiploom-harness" not in fm, name
        assert 1 <= len(fm["description"]) <= 1024, name
        assert fm.get("compatibility") == "base-spec", name


def test_skill_bodies_under_limit():
    for path in (CORE / "skills").rglob("SKILL.md"):
        _, body, _ = parse_frontmatter(path.read_text(encoding="utf-8"))
        assert len(body.splitlines()) < 500, path


def test_roles_present_with_contract_sections():
    for role in ("specifier", "implementer", "verifier"):
        path = CORE / "roles" / (role + ".md")
        assert path.exists(), role
        body = path.read_text(encoding="utf-8")
        fm, _, _ = parse_frontmatter(body)
        assert fm is None, (role, "roles carry no frontmatter by design")
        for section in ROLE_SECTIONS:
            assert section in body, (role, section)


def test_role_separation_invariants():
    """The anti-circularity rules must be stated, not implied."""
    specifier = (CORE / "roles" / "specifier.md").read_text(encoding="utf-8")
    implementer = (CORE / "roles" / "implementer.md").read_text(encoding="utf-8")
    verifier = (CORE / "roles" / "verifier.md").read_text(encoding="utf-8")
    assert "oracle" in implementer.lower()  # implementer never sees it
    assert ".oracle" in implementer
    assert "fresh" in verifier.lower()  # verifier runs in fresh context
    assert "never edits code" in verifier.lower() or "cannot edit code" in verifier.lower()
    assert "≤4" in implementer or "4" in implementer  # bounded retries stated
