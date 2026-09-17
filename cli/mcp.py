#!/usr/bin/env python3
"""MCP capability resolution (stdlib-only, MASTER_SPEC 11.12/20).

Core references `cap.*` only; server names live in the project
registry. Resolution order: capability -> attested server ->
scope-minimal -> budget/TTL -> call (by the harness) -> quarantine
if untrusted -> log. This module implements the registry half
(discovery data + ordered provider chain); the harness MCP client
performs calls.
"""

import json
from pathlib import Path

REGISTRY_NAME = "mcp-registry.json"


def registry_path(project_dir):
    return Path(project_dir) / ".shiploom" / REGISTRY_NAME


def load_registry(project_dir):
    """Load the project registry. Raises ValueError when missing/bad."""
    path = registry_path(project_dir)
    try:
        doc = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError) as exc:
        raise ValueError("unreadable MCP registry %s: %s" % (path, exc))
    if not isinstance(doc, dict) or not isinstance(doc.get("capabilities"), dict):
        raise ValueError("MCP registry %s misses capabilities{}" % path)
    return doc


def resolve(doc, capability):
    """Resolve a capability to an ordered provider chain (doc from load_registry).

    Returns None when unknown. Otherwise:
    {"capability", "providers": [names], "ttlS", "trust",
     "fallback": name|None, "servers": {name: server entry or None},
     "quarantine": bool} — quarantine is True unless every involved
    server is attested.
    """
    if not isinstance(doc, dict):
        return None
    cap = (doc.get("capabilities") or {}).get(capability)
    if not isinstance(cap, dict):
        return None
    providers = list(cap.get("providers") or [])
    servers = doc.get("servers") or {}
    chain = {"capability": capability, "providers": providers,
             "ttlS": cap.get("ttlS", 0), "trust": cap.get("trust", "community"),
             "fallback": cap.get("fallback"),
             "servers": {name: servers.get(name) for name in providers},
             "quarantine": False}
    involved = [servers.get(name) or {} for name in providers]
    if cap.get("fallback") and cap["fallback"] not in providers:
        involved.append(servers.get(cap["fallback"]) or {})
    chain["quarantine"] = any((entry.get("attested") is not True)
                              for entry in involved)
    return chain
