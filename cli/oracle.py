#!/usr/bin/env python3
"""Acceptance + oracle locking (stdlib-only, MASTER_SPEC 17 layer 1).

`lock` records sha256 hashes of `acceptance/*.json` and their oracle
implementations in the manifest so later tampering or redefinition is
detectable. `check_lock` replays the comparison. Builders see
statements only; the vault (`./.shiploom/.oracle/`, 0700, gitignored)
never enters builder context.
"""

import json
import os
import stat as statmod
import subprocess
from pathlib import Path

from cli import auditlog
from cli import manifest as manifest_mod
from validators.validate import collect_files, validate_path

ORACLE_DIRNAME = ".oracle"
LOCK_ACTION = "acceptance.lock"


def oracle_dir(project_dir):
    return Path(project_dir) / ".shiploom" / ORACLE_DIRNAME


def discover_acceptance(project_dir):
    """Find acceptance JSON files (any */acceptance/*.json, excluding
    vaults, caches, and VCS dirs). Returns sorted Paths."""
    root = Path(project_dir)
    files = collect_files(root)
    return sorted(p for p in files
                  if p.suffix == ".json" and "acceptance" in p.parts
                  and ORACLE_DIRNAME not in p.parts)


def load_criteria(files):
    """Load criteria from acceptance files (single object or array form).

    Returns (criteria, errors). criteria maps id -> (file, obj)."""
    criteria, errors = {}, []
    for path in files:
        try:
            doc = json.loads(Path(path).read_text(encoding="utf-8"))
        except (OSError, ValueError) as exc:
            errors.append({"path": str(path), "message": "invalid JSON: %s" % exc})
            continue
        objs = doc if isinstance(doc, list) else [doc]
        for obj in objs:
            if not isinstance(obj, dict) or "id" not in obj:
                continue
            cid = obj["id"]
            if cid in criteria:
                errors.append({"path": str(path),
                               "message": "duplicate acceptance id %r (also %s)"
                                          % (cid, criteria[cid][0])})
            else:
                criteria[cid] = (str(path), obj)
    return criteria, errors


def _vault_mode_ok(vault):
    if os.name != "posix":
        return True
    try:
        return statmod.S_IMODE(vault.stat().st_mode) == 0o700
    except OSError:
        return False


def _git_status(path, project_dir):
    """Return 'ignored' / 'tracked' / 'untracked' / 'unknown' for a path."""
    git = _which_git()
    if git is None:
        return "unknown"
    try:
        rel = os.path.relpath(path, project_dir)
    except ValueError:
        return "unknown"
    try:
        ignored = subprocess.run(
            [git, "-C", str(project_dir), "check-ignore", "-q", rel],
            capture_output=True, timeout=15).returncode == 0
        if ignored:
            return "ignored"
        tracked = subprocess.run(
            [git, "-C", str(project_dir), "ls-files", "--error-unmatch", rel],
            capture_output=True, timeout=15).returncode == 0
        return "tracked" if tracked else "untracked"
    except (OSError, subprocess.SubprocessError):
        return "unknown"


def _which_git():
    from shutil import which
    return which("git")


def _resolve_inside(vault, ref):
    """Resolve an oracleRef against the vault; None if it escapes."""
    candidate = (vault / ref).resolve()
    try:
        candidate.relative_to(vault.resolve())
    except ValueError:
        return None
    return candidate


def lock(project_dir, actor="human"):
    """Lock acceptance + oracles. Returns (ok, errors, warnings, summary)."""
    errors, warnings = [], []
    root = Path(project_dir)
    files = discover_acceptance(root)
    if not files:
        return False, [{"path": str(root), "message": "no acceptance/*.json found"}], [], {}

    for path in files:
        file_errors, file_warnings, _ = validate_path(path, strict=True)
        errors.extend(file_errors)
        warnings.extend(file_warnings)
    if errors:
        return False, errors, warnings, {}

    criteria, dup_errors = load_criteria(files)
    errors.extend(dup_errors)
    if errors:
        return False, errors, warnings, {}

    vault = oracle_dir(root)
    if not vault.is_dir():
        errors.append({"path": str(vault),
                       "message": "missing oracle vault (run shiploom init)"})
        return False, errors, warnings, {}
    if not _vault_mode_ok(vault):
        errors.append({"path": str(vault),
                       "message": "vault mode is not 0700 (run chmod 700 %s)" % vault})
        return False, errors, warnings, {}

    locked = {}
    for cid in sorted(criteria):
        path, obj = criteria[cid]
        ref = obj.get("oracleRef", "")
        target = _resolve_inside(vault, ref)
        if target is None:
            errors.append({"path": path,
                           "message": "%s: oracleRef escapes the vault: %r" % (cid, ref)})
            continue
        if not target.is_file():
            errors.append({"path": path,
                           "message": "%s: missing oracle implementation: %s" % (cid, target)})
            continue
        leak = _git_status(target, root)
        if leak == "tracked":
            errors.append({"path": path,
                           "message": "%s: oracle is tracked by git (leak): %s" % (cid, target)})
            continue
        if leak in ("untracked", "unknown"):
            warnings.append({"path": path,
                             "message": "%s: oracle git-ignore unverified (%s)" % (cid, leak)})
        locked[cid] = {
            "file": os.path.relpath(path, root),
            "hash": manifest_mod.sha256_file(path),
            "oracle": os.path.relpath(target, root),
            "oracleHash": manifest_mod.sha256_file(target),
        }
    if errors:
        return False, errors, warnings, {}

    try:
        data = manifest_mod.load(root)
    except (FileNotFoundError, ValueError) as exc:
        return False, [{"path": str(root), "message": "no manifest: %s (run shiploom init)" % exc}], warnings, {}
    data["acceptance"] = {"lockedAt": manifest_mod.utcnow(), "lockedBy": actor,
                          "criteria": locked}
    manifest_mod.save(root, data)
    auditlog.append(root, actor=actor, action=LOCK_ACTION,
                    target="acceptance (%d criteria)" % len(locked))
    summary = {"criteria": len(locked), "files": len(files)}
    return True, [], warnings, summary


def check_lock(project_dir):
    """Verify current files against the recorded lock. Returns (ok, errors, warnings)."""
    errors, warnings = [], []
    root = Path(project_dir)
    try:
        data = manifest_mod.load(root)
    except (FileNotFoundError, ValueError) as exc:
        return False, [{"path": str(root), "message": "no manifest: %s" % exc}], []
    locked = (data.get("acceptance") or {}).get("criteria")
    if not locked:
        return False, [{"path": str(root),
                        "message": "no acceptance lock (run shiploom lock)"}], []

    vault = oracle_dir(root)
    if vault.is_dir() and not _vault_mode_ok(vault):
        errors.append({"path": str(vault), "message": "vault mode is not 0700"})

    seen_files = set()
    for cid in sorted(locked):
        entry = locked[cid]
        fpath = root / entry["file"]
        opath = root / entry["oracle"]
        seen_files.add(entry["file"])
        if not fpath.is_file():
            errors.append({"path": entry["file"],
                           "message": "%s: locked acceptance file missing" % cid})
        elif manifest_mod.sha256_file(fpath) != entry["hash"]:
            errors.append({"path": entry["file"],
                           "message": "%s: acceptance redefined after lock" % cid})
        if not opath.is_file():
            errors.append({"path": entry["oracle"],
                           "message": "%s: locked oracle missing" % cid})
        elif manifest_mod.sha256_file(opath) != entry["oracleHash"]:
            errors.append({"path": entry["oracle"],
                           "message": "%s: oracle changed after lock" % cid})
        elif _git_status(opath, root) == "tracked":
            errors.append({"path": entry["oracle"],
                           "message": "%s: oracle is tracked by git (leak)" % cid})

    for path in discover_acceptance(root):
        rel = os.path.relpath(path, root)
        if rel not in seen_files:
            errors.append({"path": rel,
                           "message": "acceptance file not covered by lock (re-run shiploom lock)"})
    return (not errors), errors, warnings
