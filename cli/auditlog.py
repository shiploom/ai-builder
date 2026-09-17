#!/usr/bin/env python3
"""Append-only hash-chained audit log (stdlib-only).

Each entry commits to the previous entry's hash, so tampering with any
line breaks verification of every later line (MASTER_SPEC 10.1, 22).
"""

import hashlib
import json
from datetime import datetime, timezone
from pathlib import Path

AUDIT_NAME = "audit.jsonl"
GENESIS_PREV = "GENESIS"


def utcnow():
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def audit_path(project_dir):
    return Path(project_dir) / ".shiploom" / AUDIT_NAME


def _entry_hash(prev, body):
    canonical = json.dumps(body, sort_keys=True, separators=(",", ":")).encode("utf-8")
    return hashlib.sha256(prev.encode("utf-8") + b"\n" + canonical).hexdigest()


def _make_entry(prev, actor, action, target, policy, ts):
    body = {"ts": ts, "actor": actor, "action": action,
            "target": target, "policy": policy, "prev": prev}
    return dict(body, hash="sha256:" + _entry_hash(prev, body))


def init_log(project_dir):
    """Create a fresh audit log with a genesis entry (overwrites)."""
    path = audit_path(project_dir)
    path.parent.mkdir(parents=True, exist_ok=True)
    genesis = _make_entry(GENESIS_PREV, "system", "log.genesis", ".", "-", utcnow())
    path.write_text(json.dumps(genesis, sort_keys=True) + "\n", encoding="utf-8")
    return genesis


def read_all(project_dir):
    path = audit_path(project_dir)
    entries = []
    with open(path, "r", encoding="utf-8") as fh:
        for lineno, line in enumerate(fh, 1):
            line = line.strip()
            if not line:
                continue
            try:
                entries.append(json.loads(line))
            except ValueError:
                raise ValueError("audit log corrupt at line %d: not JSON" % lineno)
    return entries


def append(project_dir, actor, action, target, policy="-"):
    """Append one entry, chaining to the current tail. Returns the entry."""
    path = audit_path(project_dir)
    if not path.exists():
        init_log(project_dir)
    entries = read_all(project_dir)
    prev = entries[-1]["hash"] if entries else GENESIS_PREV
    entry = _make_entry(prev, actor, action, target, policy, utcnow())
    with open(path, "a", encoding="utf-8") as fh:
        fh.write(json.dumps(entry, sort_keys=True) + "\n")
    return entry


def verify(project_dir):
    """Replay the chain. Returns (ok, errors[]) — errors is empty when ok."""
    try:
        entries = read_all(project_dir)
    except (OSError, ValueError) as exc:
        return False, ["unreadable audit log: %s" % exc]
    if not entries:
        return False, ["audit log is empty (missing genesis)"]
    errors = []
    prev = GENESIS_PREV
    for i, entry in enumerate(entries):
        for key in ("ts", "actor", "action", "target", "policy", "prev", "hash"):
            if key not in entry:
                errors.append("line %d: missing key %r" % (i + 1, key))
        if entry.get("prev") != prev:
            errors.append("line %d: prev-link broken (reordered or deleted entries?)"
                          % (i + 1))
            break
        body = {k: entry[k] for k in ("ts", "actor", "action", "target", "policy", "prev")
                if k in entry}
        if entry.get("hash") != "sha256:" + _entry_hash(entry.get("prev", ""), body):
            errors.append("line %d: hash mismatch (tampered entry?)" % (i + 1))
            break
        prev = entry.get("hash", "")
    return (not errors), errors


def export_md(project_dir):
    """Human-readable summary table (for `audit --export md` / SIEM paste)."""
    entries = read_all(project_dir)
    lines = ["# Audit log (%d entries)" % len(entries), "",
             "| ts | actor | action | target | hash |",
             "|---|---|---|---|---|"]
    for entry in entries:
        short = str(entry.get("hash", ""))[-12:]
        lines.append("| %s | %s | %s | %s | `…%s` |" % (
            entry.get("ts"), entry.get("actor"), entry.get("action"),
            entry.get("target"), short))
    return "\n".join(lines) + "\n"
