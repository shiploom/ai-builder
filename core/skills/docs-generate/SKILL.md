---
name: docs-generate
description: Regenerate task documentation from verified artifacts with link checks. Use when artifacts change and docs must follow without chat archaeology.
license: MIT
compatibility: base-spec
metadata:
  domain: documentation
  version: "1.0.0"
---

# Docs Generate

## Purpose

Regenerate consumer and contributor docs from verified artifacts so docs
track the system instead of someone's memory of a chat session.

## Inputs

- Verified artifacts at required state (approved arch, locked acceptance,
  verification reports).
- Doc templates or existing pages to refresh.

## Outputs

- Refreshed Markdown docs with valid frontmatter, working cross-links,
  and evidence-graded claims where research is cited.

## Prerequisites

- Source artifacts verified; generating from unapproved drafts propagates
  fiction with nice formatting.

## Methodology

1. Derive each doc section from exactly one artifact (requirements →
   usage, arch → structure, verification → guarantees); record the
   mapping so staleness is traceable.
2. Re-grade any research claims carried over (Fact/Inferred/Opinion/
   Marketing/Conflicting/Uncertain + source + date); drop claims whose
   sources expired.
3. Run link checks (`shiploom validate` covers artifact links; check doc
   cross-links the same way) and fail the generation on dangling refs.
4. Keep approved wording stable: regenerate structure and facts, never
   silently rewrite human-approved narrative — surface diffs for review.
5. Version docs with the artifacts they describe (`supersedes` chain).

## Constraints

- Downstream consumes verified artifacts, never raw history: no quoting
  chat logs, no "as discussed" without an artifact link.
- No new requirements smuggled in via documentation; discoveries become
  new artifacts through the normal flow.

## Tools / MCP

- Local validators only (`cap.docs.linkcheck` if the harness provides
  one, else the repo's link scripts).

## Verification

- `shiploom validate --strict` passes on generated artifacts-adjacent
  files; zero dangling cross-links; claim grades current.

## Examples

- Regenerating the manual's J2 flow after brownfield-fix hardened: steps
  re-derived from the workflow definition, approvals list re-checked
  against the CLI surface.
