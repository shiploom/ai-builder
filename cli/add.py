#!/usr/bin/env python3
"""Install content packs into the project overlay (stdlib-only).

`add` copies a skill/workflow/hook/policy/adapter from a local path or a
git URL (+ semver tag) into `./.shiploom/<kind>s/`, schema-validates the
result with `validate --strict` semantics, rolls back on failure, and
records provenance. Marketplace trust per §33-D4 is git + semver tag +
schema check here; signatures are recorded as `unsigned` (threshold-sigs
and transparency log stay Advanced) with a printed warning.
"""

import json
import shutil
import subprocess
import tempfile
from pathlib import Path

from cli import manifest as manifest_mod
from validators.validate import validate_path

KINDS = {
    "skill": {"overlay": "skills", "style": "dir", "marker": "SKILL.md"},
    "workflow": {"overlay": "workflows", "style": "file", "suffix": ".md"},
    "hook": {"overlay": "hooks", "style": "file", "suffix": ".json"},
    "policy": {"overlay": "policies", "style": "file", "suffix": ".json"},
    "adapter": {"overlay": "adapters", "style": "dir", "marker": "mapping.json"},
}

PROVENANCE_FILE = "_provenance.json"


def _is_url(ref):
    return "://" in ref or ref.startswith("git@") or ref.endswith(".git")


def _fetch(source, tag, workdir):
    """Return a payload directory path. Raises ValueError."""
    if not _is_url(source):
        payload = Path(source)
        if not payload.exists():
            raise ValueError("source not found: %s" % source)
        return payload
    if not tag:
        raise ValueError("git sources require --tag (marketplace pins semver tags)")
    git = shutil.which("git")
    if git is None:
        raise ValueError("git not on PATH (needed for URL sources)")
    dest = Path(workdir) / "source"
    try:
        proc = subprocess.run(
            [git, "clone", "--quiet", "--depth", "1", "--branch", tag, source, str(dest)],
            capture_output=True, text=True, timeout=120)
    except (OSError, subprocess.SubprocessError) as exc:
        raise ValueError("git clone failed: %s" % exc)
    if proc.returncode != 0:
        raise ValueError("git clone failed: %s" % (proc.stderr.strip() or "exit %d" % proc.returncode))
    return dest


def _locate(kind, name, payload):
    """Find the payload root inside a source dir. Raises ValueError."""
    spec = KINDS[kind]
    base = Path(payload)
    if spec["style"] == "dir":
        if (base / spec["marker"]).is_file():
            return base
        if (base / name / spec["marker"]).is_file():
            return base / name
        raise ValueError("no %s found under %s (want %s or %s/%s)"
                         % (spec["marker"], payload, spec["marker"], name, spec["marker"]))
    if base.is_file():
        return base
    candidate = base / (name + spec["suffix"])
    if candidate.is_file():
        return candidate
    raise ValueError("no %s file found under %s (want %s%s)"
                     % (kind, payload, name, spec["suffix"]))


def _dest_for(kind, name, project_dir):
    spec = KINDS[kind]
    if spec["style"] == "dir":
        return Path(project_dir) / ".shiploom" / spec["overlay"] / name
    return Path(project_dir) / ".shiploom" / spec["overlay"] / (name + spec["suffix"])


def _record_provenance(project_dir, kind, name, source, tag):
    spec = KINDS[kind]
    overlay = Path(project_dir) / ".shiploom" / spec["overlay"]
    overlay.mkdir(parents=True, exist_ok=True)
    record_path = overlay / PROVENANCE_FILE
    try:
        records = json.loads(record_path.read_text(encoding="utf-8"))
        if not isinstance(records, dict):
            records = {}
    except (OSError, ValueError):
        records = {}
    records[name] = {"kind": kind, "source": source, "tag": tag,
                     "signed": False, "installedAt": manifest_mod.utcnow()}
    record_path.write_text(json.dumps(records, indent=2, sort_keys=True) + "\n",
                           encoding="utf-8")


def install(kind, name, source, project_dir, tag=None, force=False):
    """Install a pack into the overlay. Returns (dest_rel, warnings).

    Validates the installed result (strict); rolls back and raises
    ValueError on failure. Provenance is recorded as unsigned.
    """
    if kind not in KINDS:
        raise ValueError("unknown kind %r (choose %s)" % (kind, "|".join(sorted(KINDS))))
    if not name or "/" in name or name in (".", ".."):
        raise ValueError("bad name %r" % name)
    dest = _dest_for(kind, name, project_dir)
    if dest.exists() and not force:
        raise ValueError("%s exists (use --force to overwrite)" % dest)
    with tempfile.TemporaryDirectory(prefix="shiploom-add-") as workdir:
        payload = _fetch(source, tag, workdir)
        located = _locate(kind, name, payload)
        if dest.exists():
            if dest.is_dir() and not dest.is_symlink():
                shutil.rmtree(dest)
            else:
                dest.unlink()
        dest.parent.mkdir(parents=True, exist_ok=True)
        if located.is_dir():
            shutil.copytree(located, dest, ignore=shutil.ignore_patterns("__pycache__"))
        else:
            shutil.copy2(located, dest)
    errors, _, _ = validate_path(dest, strict=True)
    # Ignore tree-property dangling links when validating an isolated pack;
    # shape/semantic errors still fail the install.
    errors = [e for e in errors if e.get("rule") != "links.dangling"]
    if errors:
        if dest.is_dir() and not dest.is_symlink():
            shutil.rmtree(dest, ignore_errors=True)
        elif dest.exists() or dest.is_symlink():
            dest.unlink(missing_ok=True)
        raise ValueError("installed %s fails validation: %s"
                         % (name, errors[0]["message"]))
    _record_provenance(project_dir, kind, name, source, tag)
    warnings = ["unsigned provenance (threshold-sigs post-MVP)"]
    return str(dest.relative_to(project_dir)), warnings
