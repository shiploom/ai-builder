---
name: competitor-teardown
description: Tear down competitors by capability, pricing, UX, strengths, weaknesses, and differentiation with evidence grades. Use when market research exists and positioning must be decided.
license: MIT
compatibility: base-spec
metadata:
  domain: research
  version: "1.0.0"
---

# Competitor Teardown

## Purpose

Produce a capability-by-capability teardown that justifies what we
build, what we skip, and what differentiates us — without copying
competitor feature lists into our requirements.

## Inputs

- `research/market.md` and candidate competitor list.
- `idea.md` hypotheses the teardown must confirm or kill.

## Outputs

- `research/competitors.md` (capability matrix + evidence + differentiation).
- Killed hypotheses annotated in `idea.md` (supersede, never delete).

## Prerequisites

- Market landscape exists. Teardown without landscape cherry-picks rivals.

## Methodology

1. Select 3–5 competitors covering direct rivals and adjacent substitutes.
2. Score capabilities (present / partial / absent) plus pricing, UX
   notes, strengths, and weaknesses per competitor.
3. Grade every cell's backing claim: Fact / Inferred / Opinion /
   Marketing / Conflicting / Uncertain, with source + date.
4. Write differentiation: what we do that competitors cannot copy
   cheaply, and why (distribution, data, workflow, cost structure).
5. Anti-pattern check: if our draft requirements mirror a competitor's
   feature list, flag competitor-feature dumping and re-derive each
   feature from a persona need or drop it.

## Constraints

- No scraping behind auth walls or against terms of service; public
  pages and docs only.
- External snippets are verify-before-use: confirm versioned behavior
  against primary docs before citing.
- Quarantine rules from `market-research` apply unchanged.

## Tools / MCP

- Quarantined `cap.web.search` / `cap.web.fetch` only.

## Verification

- `shiploom validate --strict` passes on `research/competitors.md`.
- Every matrix claim has an evidence row; differentiation is falsifiable
  (a skeptic can state what would disprove it).

## Examples

- "Rival Y lacks SSO on team tiers (pricing page, 2026-08-01, Fact)" →
  differentiation "SSO on every tier" tied to enterprise persona need.
