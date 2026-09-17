---
name: idea-shaping
description: Shape a raw product idea into problem, users, falsifiable hypotheses, scope, and open questions. Use when starting from idea.md or a founder narrative before any requirements exist.
license: MIT
compatibility: base-spec
metadata:
  domain: product
  version: "1.0.0"
---

# Idea Shaping

## Purpose

Turn a founder narrative into a scoped, question-annotated `idea.md`
that a Specifier can research without inventing requirements.

## Inputs

- Raw narrative (chat, notes, or existing `idea.md` draft).
- Optional: target user hints, constraints (budget, timeline, stack).

## Outputs

- `idea.md` per `core/artifacts-templates/idea.md` (problem, users,
  HYP-xxx hypotheses, in/out scope, open questions).

## Prerequisites

- None. This skill runs before research and must not browse
  competitors (that biases hypotheses; research comes next).

## Methodology

1. Elicit the problem: who hurts, how often, what it costs today.
   Refuse placeholders ("users want...") — name the user.
2. Draft falsifiable hypotheses as HYP-001, HYP-002, ... Each must be
   disprovable by a stated observation within the MVP horizon.
3. Draw the first scope cut: in-scope items plus explicitly excluded
   items, each exclusion with a one-line reason.
4. List open questions with owner + answer-by date. The list must be
   non-empty: an idea with no questions is an idea nobody interrogated.
5. Write `idea.md`. Keep it plain-language; approvals are read by
   non-technical humans.

## Constraints

- Do not write requirements, architecture, or acceptance criteria.
- Do not resolve open questions by assumption; record them.
- Hypotheses live in `idea.md` until promoted, never silently dropped.

## Tools / MCP

- None required. Human interview only. (`cap.web.search` is forbidden
  in this skill to keep hypotheses unbiased.)

## Verification

- `shiploom validate` passes on `idea.md` (frontmatter + links).
- Open-questions section present and non-empty.
- Every hypothesis falsifiable (reviewer can state what disproves it).

## Examples

- Input: "Uber for dog walking." Output: problem (owners miss midday
  walks), users (urban professionals), HYP-001 ("owners pay ≥$15/walk
  for GPS-tracked walks"), out-of-scope (grooming, vet), questions
  (insurance owner, launch city).
