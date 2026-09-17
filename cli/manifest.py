#!/usr/bin/env python3
"""Orchestrator-owned manifest helpers (stdlib-only).

Manifest tracks workflow state per MASTER_SPEC 11.1. All writes are
atomic (tmp file + os.replace) so kill -9 cannot leave a half-written
manifest (stepper + resume live in `cli/run.py` since PR7).
"""

import hashlib
import json
import os
import tempfile
from datetime import datetime, timezone
from pathlib import Path

MANIFEST_NAME = "manifest.json"
WORKFLOW_VERSION = "1.0.0"

GREENFIELD_WORKFLOW = "greenfield-full-lite"
BROWNFIELD_WORKFLOW = "brownfield-fix"


def utcnow():
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def default_workflow(green=True):
    return GREENFIELD_WORKFLOW if green else BROWNFIELD_WORKFLOW


def default_budgets():
    return {
        "tokens": {"limit": 800000, "used": 0},
        "spendUSD": {"limit": 25.0, "used": 0.0},
    }


def genesis(core_version, workflow, budgets=None):
    """Build a genesis manifest dict (not yet saved)."""
    return {
        "coreVersion": core_version,
        "workflow": workflow,
        "workflowVersion": WORKFLOW_VERSION,
        "artifacts": {},
        "gates": {},
        "budgets": budgets if budgets is not None else default_budgets(),
        "retries": {},
        "checkpoints": [],
        "initializedAt": utcnow(),
    }


def manifest_path(project_dir):
    return Path(project_dir) / ".shiploom" / MANIFEST_NAME


def load(project_dir):
    """Load manifest or raise (FileNotFoundError / ValueError on bad JSON)."""
    path = manifest_path(project_dir)
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError:
        raise
    except (OSError, ValueError) as exc:
        raise ValueError("unreadable manifest %s: %s" % (path, exc))


def save(project_dir, data):
    """Atomically write the manifest (kill-safe)."""
    path = manifest_path(project_dir)
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, tmp = tempfile.mkstemp(dir=str(path.parent), prefix=".manifest-", suffix=".tmp")
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as fh:
            json.dump(data, fh, indent=2, sort_keys=True)
            fh.write("\n")
        os.replace(tmp, path)
    except BaseException:
        try:
            os.unlink(tmp)
        except OSError:
            pass
        raise
    return path


def sha256_file(path):
    digest = hashlib.sha256()
    with open(path, "rb") as fh:
        for chunk in iter(lambda: fh.read(65536), b""):
            digest.update(chunk)
    return "sha256:" + digest.hexdigest()
