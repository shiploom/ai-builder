#!/usr/bin/env python3
"""Harness adapters: single-source core -> generated files (stdlib-only).

Conventions (MASTER_SPEC 11.11):
- `shiploom adapters --generate` is idempotent; generated files carry
  `DO NOT EDIT` headers and are never hand-edited.
- Skills ship as base-spec byte copies; the header is an HTML comment
  inserted AFTER the frontmatter block so validators keep passing.
- Templates use Mustache-free `{{var}}` substitution with
  {projectName, coreVersion, workflow}. Unknown vars are left verbatim.
"""

import json
from pathlib import Path

TOOL_ROOT = Path(__file__).resolve().parent.parent
ADAPTERS_DIR = TOOL_ROOT / "adapters"
CORE_SKILLS = TOOL_ROOT / "core" / "skills"

TEMPLATE_VARS = ("projectName", "coreVersion", "workflow")


def _core_version():
    return (TOOL_ROOT / "core" / "VERSION").read_text(encoding="utf-8").strip()


def _default_workflow(project_dir):
    try:
        config = json.loads((Path(project_dir) / ".shiploom" / "config.json")
                            .read_text(encoding="utf-8"))
        workflow = config.get("workflow")
        if workflow:
            return workflow.split("/")[-1]
    except (OSError, ValueError):
        pass
    return "greenfield-full-lite"


def _context(project_dir):
    return {"projectName": Path(project_dir).resolve().name,
            "coreVersion": _core_version(),
            "workflow": _default_workflow(project_dir)}


def _substitute(template, context):
    for key in TEMPLATE_VARS:
        template = template.replace("{{" + key + "}}", str(context[key]))
    return template


def _skill_with_header(source):
    """Copy a SKILL.md, inserting the DO NOT EDIT comment after frontmatter."""
    text = source.read_text(encoding="utf-8")
    lines = text.splitlines(keepends=True)
    header = ("<!-- DO NOT EDIT — generated from Shiploom core %s; "
              "edit core source, then re-run `shiploom adapters --generate`. -->\n"
              % _core_version())
    if lines and lines[0].strip() == "---":
        for i in range(1, len(lines)):
            if lines[i].strip() in ("---", "..."):
                return "".join(lines[:i + 1]) + header + "".join(lines[i + 1:])
    return header + text


def list_adapters():
    """Describe available adapters from their mapping.json files."""
    adapters = []
    for child in sorted(ADAPTERS_DIR.iterdir()):
        mapping = child / "mapping.json"
        if not child.is_dir() or not mapping.is_file():
            continue
        try:
            doc = json.loads(mapping.read_text(encoding="utf-8"))
        except ValueError:
            continue
        adapters.append({
            "adapter": doc.get("adapter", child.name),
            "version": doc.get("version", "?"),
            "description": doc.get("description", ""),
            "outputs": [o.get("target", "?") for o in doc.get("outputs", [])]
                       + (["<skillsTarget>/*/SKILL.md"] if doc.get("skills") else []),
        })
    return adapters


def generate(harness, project_dir):
    """Generate harness files. Returns (report, errors).

    report: {"adapter", "created": [], "updated": [], "unchanged": []}
    with paths relative to the project root.
    """
    if harness == "all":
        full = {"created": [], "updated": [], "unchanged": []}
        errors = []
        for adapter in list_adapters():
            report, errs = generate(adapter["adapter"], project_dir)
            for key in full:
                full[key].extend(report[key])
            errors.extend(errs)
        return {"adapter": "all", **full}, errors

    src = ADAPTERS_DIR / harness
    mapping_path = src / "mapping.json"
    if not mapping_path.is_file():
        return None, ["unknown adapter %r" % harness]
    try:
        mapping = json.loads(mapping_path.read_text(encoding="utf-8"))
    except ValueError as exc:
        return None, ["bad mapping.json for %r: %s" % (harness, exc)]

    root = Path(project_dir)
    context = _context(project_dir)
    report = {"adapter": harness, "created": [], "updated": [], "unchanged": []}
    errors = []

    def _write(rel, content):
        dest = root / rel
        dest.parent.mkdir(parents=True, exist_ok=True)
        if dest.is_file() and dest.read_text(encoding="utf-8") == content:
            report["unchanged"].append(rel)
        else:
            created = not dest.exists()
            dest.write_text(content, encoding="utf-8")
            report["created" if created else "updated"].append(rel)

    for output in mapping.get("outputs", []):
        template_path = src / output.get("template", "")
        target = output.get("target", "")
        if not template_path.is_file() or not target:
            errors.append("bad output mapping in %s adapter" % harness)
            continue
        try:
            _write(target, _substitute(template_path.read_text(encoding="utf-8"), context))
        except OSError as exc:
            errors.append("cannot write %s: %s" % (target, exc))

    if mapping.get("skills"):
        skills_target = mapping.get("skillsTarget", "")
        if not skills_target:
            errors.append("skills adapter %r misses skillsTarget" % harness)
        elif not CORE_SKILLS.is_dir():
            errors.append("core skills missing: %s" % CORE_SKILLS)
        else:
            for skill in sorted(CORE_SKILLS.iterdir()):
                source = skill / "SKILL.md"
                if not skill.is_dir() or not source.is_file():
                    continue
                try:
                    _write("%s/%s/SKILL.md" % (skills_target, skill.name),
                           _skill_with_header(source))
                except OSError as exc:
                    errors.append("cannot write %s skill: %s" % (skill.name, exc))

    for key in ("created", "updated", "unchanged"):
        report[key].sort()
    return report, errors
