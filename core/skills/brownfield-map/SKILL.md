---
name: brownfield-map
description: Map an unfamiliar repository structure, architecture, conventions, dependencies, and risks with confidence notes. Use when starting any brownfield analysis before impact planning.
license: MIT
compatibility: base-spec
metadata:
  domain: brownfield
  version: "1.0.0"
---

# Brownfield Map

## Purpose

Produce `brownfield/repo-map.md` grounded in the symbol graph plus
build and test evidence — never a file list — with low-confidence
areas marked so downstream write scopes narrow accordingly.

## Inputs

- Repository read access (default read-only recon).
- Build/test commands if runnable locally (evidence, not faith).

## Outputs

- `brownfield/repo-map.md` per core template (structure, arch,
  conventions, deps, behavior, debt refs, confidence notes).

## Prerequisites

- None. Map before impact; never plan a change on an unmapped area.

## Methodology

1. Record top-level layout, ownership, and entry points.
2. Recover layering and conventions from imports, configs, and tests —
   prefer the symbol graph (`search_symbols`/`find_usages` equivalents)
   over directory names.
3. List external services, data flows, jobs, and flags with file:line refs.
4. Run the build and test suite when possible; record what passes,
   what is skipped, and what cannot run here.
5. Mirror debt into `brownfield/debt-register.md` with interest notes.
6. Mark every low-confidence area explicitly with what would raise
   confidence. Muted-repo masks (`archive/`, generated code) apply:
   never extrapolate current conventions from versioned history.

## Constraints

- Read-only recon by default; no edits, no dependency installs that
  mutate the repo.
- A map without confidence notes is fiction: fail validation by
  returning it for revision.

## Tools / MCP

- Read-only repo tools; scoped `cap.repo.symbols` equivalent where the
  harness provides one.

## Verification

- `shiploom validate --strict` passes on the map.
- Every structural claim resolves to a symbol, build output, or test;
  confidence notes present and non-empty.

## Examples

- Map marks `billing/` "low confidence: reflection-based dispatch,
  raise by adding contract stubs" → impact plan narrows write scope to
  the single touched handler plus its direct callers.
