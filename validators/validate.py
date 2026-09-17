#!/usr/bin/env python3
"""Shiploom offline validator (stdlib-only).

Validates schemas + artifact frontmatter + links per MASTER_SPEC 11.1/11.2/16
and BUILD_PLAN PR2.

Usage:
    python3 validators/validate.py [--strict] [path]

Output (stdout, always JSON):
    {"ok": bool, "errors": [{"path","message","rule?"}], "warnings": [...]}

Exit codes: 0 pass (warnings allowed), 2 validation fail.
No third-party imports. No network. No writes (trace index builds
in-memory; `validators/trace.py --out` writes trace.json).
"""

import argparse
import json
import os
import re
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
SCHEMAS_DIR = REPO_ROOT / "schemas"

SCHEMA_FILES = {
    "artifact-frontmatter": "artifact-frontmatter.schema.json",
    "acceptance": "acceptance.schema.json",
    "skill": "skill.schema.json",
    "workflow": "workflow.schema.json",
    "hook": "hook.schema.json",
    "policy": "policy.schema.json",
    "mcp-registry": "mcp-registry.schema.json",
    "verification-report": "verification-report.schema.json",
    "trace": "trace-link.schema.json",
}

# Directories / files never validated as data (schemas are contracts, not data;
# caches, VCS, oracle vault, and venvs are never data).
EXCLUDE_DIRS = {
    ".git", ".venv", ".conda", "__pycache__", "node_modules",
    "dist", "build", ".oracle", ".validator-cache",
}
EXCLUDE_FILES = {"trace.json"}  # validated separately if explicitly passed

# Vague adjectives that fail `validate --strict` unless paired with a
# measurable howToVerify (MASTER_SPEC 32.2, 11.2).
VAGUE_TERMS = [
    "fast", "secure", "user-friendly", "user friendly", "scalable", "robust",
    "intuitive", "seamless", "high-performance", "high performance",
    "blazing", "easy", "simple", "quickly", "best", "state-of-the-art",
    "military-grade", "bank-grade",
]
_VAGUE_RES = [re.compile(r"\b%s\b" % re.escape(t), re.IGNORECASE) for t in VAGUE_TERMS]

_VALID_ID = re.compile(r"^[A-Z]{2,6}-[0-9]{3,}$")

_schemas_cache = {}


# ----------------------------------------------------------------------------
# Minimal JSON Schema (draft 2020-12) subset evaluator
# ----------------------------------------------------------------------------

def load_schema(name):
    """Load and cache a normative schema by short name."""
    if name in _schemas_cache:
        return _schemas_cache[name]
    path = SCHEMAS_DIR / SCHEMA_FILES[name]
    with open(path, "r", encoding="utf-8") as fh:
        _schemas_cache[name] = json.load(fh)
    return _schemas_cache[name]


def _type_ok(value, expected):
    if expected == "object":
        return isinstance(value, dict)
    if expected == "array":
        return isinstance(value, list)
    if expected == "string":
        return isinstance(value, str)
    if expected == "integer":
        return isinstance(value, int) and not isinstance(value, bool)
    if expected == "number":
        return isinstance(value, (int, float)) and not isinstance(value, bool)
    if expected == "boolean":
        return isinstance(value, bool)
    if expected == "null":
        return value is None
    return False


def validate_against_schema(data, schema, path="$"):
    """Validate data against a schema subset. Returns list of error strings.

    Supported keywords: type, enum, pattern, minLength, maxLength, minimum,
    maximum, minItems, maxItems, required, properties, patternProperties,
    additionalProperties, items. Unknown keywords are ignored (spec:
    forward-compatible, never error on unknown keywords).
    """
    errors = []
    if not isinstance(schema, dict):
        return errors

    expected = schema.get("type")
    if expected is not None:
        types = expected if isinstance(expected, list) else [expected]
        if not any(_type_ok(data, t) for t in types):
            errors.append(
                "%s: expected type %s, got %s"
                % (path, expected, type(data).__name__)
            )
            return errors  # further checks are meaningless on type mismatch

    if "enum" in schema:
        if data not in schema["enum"]:
            errors.append("%s: %r not in enum %r" % (path, data, schema["enum"]))

    if isinstance(data, str):
        if "pattern" in schema:
            try:
                if not re.search(schema["pattern"], data):
                    errors.append(
                        "%s: %r does not match pattern %r"
                        % (path, data, schema["pattern"])
                    )
            except re.error as exc:
                errors.append("%s: invalid schema pattern: %s" % (path, exc))
        if "minLength" in schema and len(data) < schema["minLength"]:
            errors.append(
                "%s: string shorter than minLength %d" % (path, schema["minLength"])
            )
        if "maxLength" in schema and len(data) > schema["maxLength"]:
            errors.append(
                "%s: string longer than maxLength %d" % (path, schema["maxLength"])
            )

    if isinstance(data, bool):
        pass  # numbers below must not treat bools as numbers
    elif isinstance(data, (int, float)):
        if "minimum" in schema and data < schema["minimum"]:
            errors.append(
                "%s: %r below minimum %r" % (path, data, schema["minimum"])
            )
        if "maximum" in schema and data > schema["maximum"]:
            errors.append(
                "%s: %r above maximum %r" % (path, data, schema["maximum"])
            )

    if isinstance(data, list):
        if "minItems" in schema and len(data) < schema["minItems"]:
            errors.append(
                "%s: fewer than minItems %d" % (path, schema["minItems"])
            )
        if "maxItems" in schema and len(data) > schema["maxItems"]:
            errors.append(
                "%s: more than maxItems %d" % (path, schema["maxItems"])
            )
        if "items" in schema and isinstance(schema["items"], dict):
            for i, item in enumerate(data):
                errors.extend(
                    validate_against_schema(item, schema["items"], "%s[%d]" % (path, i))
                )

    if isinstance(data, dict):
        for key in schema.get("required", []):
            if key not in data:
                errors.append("%s: missing required property %r" % (path, key))
        props = schema.get("properties", {})
        pat_props = schema.get("patternProperties", {})
        for key, value in data.items():
            sub = None
            sub_path = "%s.%s" % (path, key)
            if key in props:
                sub = props[key]
            else:
                for pat, pat_schema in pat_props.items():
                    try:
                        matched = re.search(pat, key) is not None
                    except re.error:
                        matched = False
                    if matched:
                        sub = pat_schema
                        break
                if sub is None and "additionalProperties" in schema:
                    extra = schema["additionalProperties"]
                    if extra is False:
                        errors.append(
                            "%s: unexpected property %r" % (path, key)
                        )
                        continue
                    if isinstance(extra, dict):
                        sub = extra
            if sub is not None:
                errors.extend(validate_against_schema(value, sub, sub_path))

    return errors


# ----------------------------------------------------------------------------
# Minimal YAML-subset frontmatter reader (stdlib only)
# ----------------------------------------------------------------------------

def _strip_comment(line):
    # Full-line comments only: inline '#' is legal inside URLs / strings.
    stripped = line.lstrip()
    if stripped.startswith("#"):
        return ""
    return line


def _parse_scalar(raw):
    s = raw.strip()
    if s == "" or s == "~" or s.lower() == "null":
        return None
    if len(s) >= 2 and s[0] == s[-1] and s[0] in ("'", '"'):
        return s[1:-1]
    low = s.lower()
    if low == "true":
        return True
    if low == "false":
        return False
    if s.startswith("[") and not s.endswith("]"):
        raise ValueError("unclosed flow list: %r" % raw)
    if s.startswith("[") and s.endswith("]"):
        inner = s[1:-1].strip()
        if not inner:
            return []
        parts = []
        buf, quote = "", None
        for ch in inner:
            if quote:
                buf += ch
                if ch == quote:
                    quote = None
            elif ch in ("'", '"'):
                quote = ch
                buf += ch
            elif ch == ",":
                parts.append(_parse_scalar(buf))
                buf = ""
            else:
                buf += ch
        if buf.strip():
            parts.append(_parse_scalar(buf))
        return parts
    try:
        return int(s)
    except ValueError:
        pass
    try:
        return float(s)
    except ValueError:
        pass
    return s


def _parse_mapping(lines, pos, indent):
    """Parse indented mapping block. Returns (dict, next_pos)."""
    out = {}
    n = len(lines)
    while pos < n:
        ind, text = lines[pos]
        if ind != indent:
            break
        if text.startswith("- ") or text == "-":
            break
        if ":" not in text:
            # Not a mapping line at this level; caller handles the error.
            break
        key, _, rest = text.partition(":")
        key = key.strip()
        rest = rest.strip()
        pos += 1
        if rest != "":
            out[key] = _parse_scalar(rest)
        else:
            # Nested block (list or mapping) or empty value.
            while pos < n and lines[pos][0] is None:
                pos += 1
            if pos >= n or lines[pos][0] <= indent:
                out[key] = None
            elif lines[pos][1].startswith("- ") or lines[pos][1] == "-":
                val, pos = _parse_list(lines, pos, lines[pos][0])
                out[key] = val
            else:
                val, pos = _parse_mapping(lines, pos, lines[pos][0])
                out[key] = val
    return out, pos


def _parse_list(lines, pos, indent):
    """Parse indented '- ' list block. Supports scalar items and
    dash-led mapping items with deeper continuation lines."""
    out = []
    n = len(lines)
    while pos < n:
        ind, text = lines[pos]
        if ind != indent or not (text.startswith("- ") or text == "-"):
            break
        item_text = text[1:].strip() if text != "-" else ""
        pos += 1
        if item_text == "":
            # Nested block item or null.
            while pos < n and lines[pos][0] is None:
                pos += 1
            if pos < n and lines[pos][0] > indent:
                if lines[pos][1].startswith("- ") or lines[pos][1] == "-":
                    val, pos = _parse_list(lines, pos, lines[pos][0])
                else:
                    val, pos = _parse_mapping(lines, pos, lines[pos][0])
                out.append(val)
            else:
                out.append(None)
        elif ":" in item_text and not item_text.startswith("["):
            # Dash-led mapping: first pair on the dash line, rest deeper.
            key, _, rest = item_text.partition(":")
            item = {key.strip(): _parse_scalar(rest)}
            while pos < n and lines[pos][0] is None:
                pos += 1
            while pos < n and lines[pos][0] > indent:
                cind, ctext = lines[pos]
                if ctext.startswith("- ") or ctext == "-":
                    break
                if ":" not in ctext:
                    break
                k2, _, r2 = ctext.partition(":")
                item[k2.strip()] = _parse_scalar(r2)
                pos += 1
                # consume any deeper nesting under this pair as opaque? Not
                # needed for MVP shapes; loop handles one level.
                while pos < n and lines[pos][0] is not None and lines[pos][0] > cind:
                    # Deeper than one level: unsupported; stop list parsing
                    # so the caller reports unparsed lines.
                    break
            out.append(item)
        else:
            out.append(_parse_scalar(item_text))
    return out, pos


def parse_frontmatter(text):
    """Split Markdown frontmatter. Returns (fm_dict|None, body, error|None).

    - No leading '---' -> (None, text, None): file is skipped, not an error.
    - Malformed YAML subset -> (None, body, error string).
    """
    lines = text.splitlines()
    if not lines or lines[0].strip() != "---":
        return None, text, None
    end = None
    for i in range(1, len(lines)):
        if lines[i].strip() in ("---", "..."):
            end = i
            break
    if end is None:
        return None, "\n".join(lines[1:]), "unterminated frontmatter block"
    fm_lines, body = lines[1:end], "\n".join(lines[end + 1:])

    indexed = []
    for raw in fm_lines:
        if raw.strip() == "" or raw.lstrip().startswith("#"):
            indexed.append((None, ""))
            continue
        nospace = raw.lstrip(" ")
        if nospace != raw.lstrip():
            # Leading tab (or other non-space whitespace): reject loudly.
            return None, body, "tabs not allowed in frontmatter indentation"
        stripped = nospace.strip()
        indent = len(raw) - len(nospace)
        indexed.append((indent, _strip_comment(stripped).strip()))

    indexed = [(i, t) for i, t in indexed if i is not None]
    if not indexed:
        return None, body, "empty frontmatter block"
    try:
        data, pos = _parse_mapping(indexed, 0, indexed[0][0])
    except Exception as exc:  # never crash validation on parse bugs
        return None, body, "frontmatter parse failure: %s" % exc
    if pos != len(indexed):
        return None, body, "unparsed frontmatter near: %r" % (indexed[pos][1],)
    if not isinstance(data, dict):
        return None, body, "frontmatter must be a mapping"
    return data, body, None


# ----------------------------------------------------------------------------
# Per-kind semantic checks (beyond JSON Schema shape)
# ----------------------------------------------------------------------------

def vague_terms_in(statement):
    return [t for t, rx in zip(VAGUE_TERMS, _VAGUE_RES) if rx.search(statement or "")]


def check_acceptance_semantics(obj, strict):
    """Returns (errors, warnings) for one acceptance object."""
    errors, warnings = [], []
    hov = obj.get("howToVerify") or {}
    htype = hov.get("type")
    command = hov.get("command")
    expect = hov.get("expect")
    if htype in ("script", "http", "browser") and not command:
        errors.append("howToVerify.command required for type %r" % htype)
    vague = vague_terms_in(obj.get("statement", ""))
    if vague:
        measurable = bool(expect) and len(str(expect)) >= 10 and htype != "human"
        msg = "vague term(s) %s without measurable howToVerify" % (", ".join(vague))
        if measurable and not strict:
            warnings.append(msg + " (strict will fail)")
        elif measurable and strict:
            warnings.append(msg + " (measurable verifier present)")
        elif strict:
            errors.append(msg)
        else:
            warnings.append(msg + " (use --strict to enforce)")
    oracle = obj.get("oracleRef", "")
    if isinstance(oracle, str) and oracle.startswith("/"):
        errors.append("oracleRef must be relative, got absolute path")
    if isinstance(oracle, str) and ".." in oracle.split("/"):
        errors.append("oracleRef must not contain '..'")
    return errors, warnings


def check_skill_semantics(fm, file_path, body):
    errors, warnings = [], []
    name = fm.get("name", "")
    parent = Path(file_path).parent.name
    if name != parent:
        errors.append(
            "skill name %r must equal directory name %r" % (name, parent)
        )
    body_lines = body.splitlines()
    if len(body_lines) > 500:
        errors.append(
            "skill body %d lines exceeds 500-line limit" % len(body_lines)
        )
    return errors, warnings


def check_workflow_semantics(fm):
    errors, warnings = [], []
    steps = fm.get("steps", []) or []
    seen = set()
    for step in steps:
        if not isinstance(step, dict):
            continue
        sid = step.get("id")
        if sid in seen:
            errors.append("duplicate step id %r" % sid)
        seen.add(sid)
    return errors, warnings


def check_hook_semantics(obj):
    errors, warnings = [], []
    action = obj.get("action")
    if action in ("run-validator", "run-script") and "run" not in obj:
        errors.append("hook action %r requires 'run' {kind, ref}" % action)
    if action in ("deny", "notify") and "run" in obj:
        warnings.append("hook action %r ignores 'run'" % action)
    return errors, warnings


def check_policy_semantics(obj):
    errors, warnings = [], []
    if obj.get("defaultEffect", "deny") != "deny":
        warnings.append("policy defaultEffect should be 'deny'")
    return errors, warnings


# ----------------------------------------------------------------------------
# File discovery + validation
# ----------------------------------------------------------------------------

def _is_excluded(path, root):
    try:
        rel = Path(path).relative_to(root)
    except ValueError:
        rel = Path(path)
    return any(part in EXCLUDE_DIRS for part in rel.parts)


def collect_files(root):
    root = Path(root)
    if root.is_file():
        return [root]
    out = []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in EXCLUDE_DIRS]
        for fn in filenames:
            p = Path(dirpath) / fn
            if p.suffix not in (".md", ".json"):
                continue
            if p.name in EXCLUDE_FILES and p.parent != root:
                # trace.json files found during a tree walk are validated
                # only via the trace pass, not as generic JSON.
                if p.name == "trace.json":
                    continue
            out.append(p)
    return sorted(out)


def _looks_like_schema_doc(doc):
    return isinstance(doc, dict) and "$schema" in doc and "$id" in doc


def classify_json(path, doc):
    """Return schema short-name for a JSON doc, or None to skip."""
    name = Path(path).name
    if _looks_like_schema_doc(doc):
        return None  # normative contracts are not data
    if name == "trace.json":
        return "trace"
    if isinstance(doc, dict):
        if "policyId" in doc:
            return "policy"
        if "capabilities" in doc:
            return "mcp-registry"
        if "results" in doc and "verdict" in doc:
            return "verification-report"
        if "event" in doc and "matcher" in doc:
            return "hook"
        if "howToVerify" in doc or (
            isinstance(doc.get("id"), str) and doc.get("id", "").startswith("ACC-")
        ):
            return "acceptance"
    if isinstance(doc, list) and doc:
        first = doc[0]
        if isinstance(first, dict):
            if "event" in first and "matcher" in first:
                return "hook"
            if "howToVerify" in first:
                return "acceptance"
    # Path-convention fallback
    parts = [p.lower() for p in Path(path).parts]
    if "policies" in parts:
        return "policy"
    if "acceptance" in parts:
        return "acceptance"
    if "hooks" in parts:
        return "hook"
    return None


def validate_markdown_file(path, strict, errors, warnings, artifacts):
    try:
        text = Path(path).read_text(encoding="utf-8")
    except OSError as exc:
        errors.append({"path": str(path), "message": "unreadable: %s" % exc})
        return
    fm, body, perr = parse_frontmatter(text)
    if fm is None and perr is None:
        return  # no frontmatter: not an artifact (e.g. README, spec)
    if perr:
        errors.append({"path": str(path), "message": perr, "rule": "frontmatter"})
        return

    fname = Path(path).name
    if fname == "SKILL.md" or ("name" in fm and "description" in fm and "id" not in fm
                               and "steps" not in fm):
        for msg in validate_against_schema(fm, load_schema("skill"), "$"):
            errors.append({"path": str(path), "message": msg, "rule": "skill.schema"})
        for msg in check_skill_semantics(fm, path, body)[0]:
            errors.append({"path": str(path), "message": msg, "rule": "skill.semantics"})
        for msg in check_skill_semantics(fm, path, body)[1]:
            warnings.append({"path": str(path), "message": msg, "rule": "skill.semantics"})
        return

    if "steps" in fm or (fname.endswith(".md") and "workflows" in Path(path).parts):
        for msg in validate_against_schema(fm, load_schema("workflow"), "$"):
            errors.append({"path": str(path), "message": msg, "rule": "workflow.schema"})
        for msg in check_workflow_semantics(fm)[0]:
            errors.append({"path": str(path), "message": msg, "rule": "workflow.semantics"})
        return

    if "id" in fm and "kind" in fm:
        for msg in validate_against_schema(fm, load_schema("artifact-frontmatter"), "$"):
            errors.append({"path": str(path), "message": msg, "rule": "artifact-frontmatter.schema"})
        aid = fm.get("id")
        if isinstance(aid, str):
            if aid in artifacts:
                errors.append({
                    "path": str(path),
                    "message": "duplicate artifact id %r (also %s)" % (aid, artifacts[aid]),
                    "rule": "artifact.duplicate",
                })
            else:
                artifacts[aid] = str(path)
        return

    # Frontmatter present but unrecognized shape: workflows/skills without
    # required keys still get schema-checked for a precise message.
    if "name" in fm and "steps" in fm:
        for msg in validate_against_schema(fm, load_schema("workflow"), "$"):
            errors.append({"path": str(path), "message": msg, "rule": "workflow.schema"})
    elif "name" in fm:
        for msg in validate_against_schema(fm, load_schema("skill"), "$"):
            errors.append({"path": str(path), "message": msg, "rule": "skill.schema"})
    else:
        warnings.append({
            "path": str(path),
            "message": "frontmatter ignored: no id/kind (artifact), name (skill), or steps (workflow)",
            "rule": "frontmatter.shape",
        })


def validate_json_file(path, strict, errors, warnings):
    try:
        doc = json.loads(Path(path).read_text(encoding="utf-8"))
    except (OSError, ValueError) as exc:
        errors.append({"path": str(path), "message": "invalid JSON: %s" % exc})
        return
    kind = classify_json(path, doc)
    if kind is None:
        return  # not a Shiploom data file (e.g. package.json, schemas)

    if kind == "hook" and isinstance(doc, list):
        schema = load_schema("hook")
        for i, item in enumerate(doc):
            for msg in validate_against_schema(item, schema, "$[%d]" % i):
                errors.append({"path": str(path), "message": msg, "rule": "hook.schema"})
            if isinstance(item, dict):
                for msg in check_hook_semantics(item)[0]:
                    errors.append({"path": str(path), "message": msg, "rule": "hook.semantics"})
                for msg in check_hook_semantics(item)[1]:
                    warnings.append({"path": str(path), "message": msg, "rule": "hook.semantics"})
        return

    if kind == "acceptance" and isinstance(doc, list):
        schema = load_schema("acceptance")
        for i, item in enumerate(doc):
            for msg in validate_against_schema(item, schema, "$[%d]" % i):
                errors.append({"path": str(path), "message": msg, "rule": "acceptance.schema"})
            if isinstance(item, dict):
                es, ws = check_acceptance_semantics(item, strict)
                for msg in es:
                    errors.append({"path": str(path), "message": "$[%d]: %s" % (i, msg),
                                           "rule": "acceptance.semantics"})
                for msg in ws:
                    warnings.append({"path": str(path), "message": "$[%d]: %s" % (i, msg),
                                             "rule": "acceptance.semantics"})
        return

    schema = load_schema(kind)
    for msg in validate_against_schema(doc, schema, "$"):
        errors.append({"path": str(path), "message": msg, "rule": "%s.schema" % kind})

    if kind == "acceptance" and isinstance(doc, dict):
        es, ws = check_acceptance_semantics(doc, strict)
        for msg in es:
            errors.append({"path": str(path), "message": msg, "rule": "acceptance.semantics"})
        for msg in ws:
            warnings.append({"path": str(path), "message": msg, "rule": "acceptance.semantics"})
    elif kind == "hook" and isinstance(doc, dict):
        for msg in check_hook_semantics(doc)[0]:
            errors.append({"path": str(path), "message": msg, "rule": "hook.semantics"})
        for msg in check_hook_semantics(doc)[1]:
            warnings.append({"path": str(path), "message": msg, "rule": "hook.semantics"})
    elif kind == "policy" and isinstance(doc, dict):
        for msg in check_policy_semantics(doc)[1]:
            warnings.append({"path": str(path), "message": msg, "rule": "policy.semantics"})


def check_trace_links(artifacts, files, strict, errors, warnings):
    """Cross-file link check: every links{} target should resolve to a known
    artifact id. Unknown targets warn (error under --strict) so incremental
    authoring is not blocked."""
    known = set(artifacts)
    # Also collect acceptance / test / VR ids from JSON docs so links to
    # ACC-*/TEST-* do not warn when those files exist.
    for path in files:
        if Path(path).suffix != ".json":
            continue
        try:
            doc = json.loads(Path(path).read_text(encoding="utf-8"))
        except (OSError, ValueError):
            continue
        if _looks_like_schema_doc(doc):
            continue
        objs = doc if isinstance(doc, list) else [doc]
        for obj in objs:
            if isinstance(obj, dict) and isinstance(obj.get("id"), str):
                known.add(obj["id"])
            if isinstance(obj, dict) and isinstance(obj.get("acceptanceId"), str):
                known.add(obj["acceptanceId"])

    return known


def validate_path(target, strict=False):
    """Validate a file or directory tree. Returns (errors, warnings, artifacts)."""
    errors, warnings, artifacts = [], [], {}
    root = Path(target)
    base = root if root.is_dir() else root.parent
    files = collect_files(root)
    for path in files:
        if _is_excluded(path, base):
            continue
        if path.suffix == ".md":
            validate_markdown_file(str(path), strict, errors, warnings, artifacts)
        elif path.suffix == ".json":
            # trace.json files encountered in a walk are handled in the
            # trace pass below; explicit file targets still validate.
            if path.name == "trace.json" and root.is_dir():
                continue
            validate_json_file(str(path), strict, errors, warnings)

    # Explicit trace.json target validates against the trace schema.
    if root.is_file() and root.name == "trace.json":
        pass  # already handled by validate_json_file above

    # Link pass over collected artifact frontmatter links.
    known = check_trace_links(artifacts, files, strict, errors, warnings)
    # Second walk to read links (kept separate so tests can inspect `known`).
    for aid, apath in sorted(artifacts.items()):
        try:
            fm, _, perr = parse_frontmatter(Path(apath).read_text(encoding="utf-8"))
        except OSError:
            continue
        if fm is None or perr:
            continue
        links = fm.get("links") or {}
        if not isinstance(links, dict):
            continue
        for rel, targets in links.items():
            if not isinstance(targets, list):
                continue
            for tgt in targets:
                if tgt not in known:
                    msg = "%s: dangling link %s -> %r" % (aid, rel, tgt)
                    if strict:
                        errors.append({"path": apath, "message": msg, "rule": "links.dangling"})
                    else:
                        warnings.append({"path": apath, "message": msg, "rule": "links.dangling"})

    # Trace index files present in the tree must also schema-validate.
    for path in files:
        if Path(path).name != "trace.json" or not root.is_dir():
            continue
        try:
            doc = json.loads(Path(path).read_text(encoding="utf-8"))
        except (OSError, ValueError) as exc:
            errors.append({"path": str(path), "message": "invalid JSON: %s" % exc})
            continue
        for msg in validate_against_schema(doc, load_schema("trace"), "$"):
            errors.append({"path": str(path), "message": msg, "rule": "trace.schema"})

    errors.sort(key=lambda e: (e["path"], e["message"]))
    warnings.sort(key=lambda e: (e["path"], e["message"]))
    return errors, warnings, artifacts


def main(argv=None):
    parser = argparse.ArgumentParser(
        description="Shiploom offline validator (stdlib-only)."
    )
    parser.add_argument("path", nargs="?", default=".",
                        help="file or directory to validate (default: .)")
    parser.add_argument("--strict", action="store_true",
                        help="vague acceptance + dangling links fail instead of warn")
    args = parser.parse_args(argv)

    errors, warnings, _ = validate_path(args.path, strict=args.strict)
    result = {"ok": not errors, "errors": errors, "warnings": warnings}
    json.dump(result, sys.stdout, indent=2, sort_keys=True)
    sys.stdout.write("\n")
    return 0 if not errors else 2


if __name__ == "__main__":
    sys.exit(main())
