# Universal AI Software Engineering System — Build-Ready Master Specification

**Version:** 1.0-draft · **Date:** 2026-09-17 · **Status:** Research-derived, pre-implementation
**Guiding question:** *What is actually required to solve this problem reliably, what already exists, why does it fail, and what is the simplest architecture capable of solving the remaining problem?*
**Evidence convention used throughout:** `[Fact]` = primary-source documented · `[Vendor]` = vendor claim, uncorroborated · `[Community]` = user/issue reports · `[Research]` = paper/benchmark · `[Hypothesis]` = author inference · `[Recommendation]` = decision derived from evidence.

---

## Table of Contents

1. Executive Summary
2. Landscape Research
3. Comparative Analysis: Framework vs IDE vs Hybrid
4. Failure-Mode Analysis
5. Design Principles (evidence-derived)
6. Proposed Product
7. Product Strategy
8. PRD (Personas, Journeys, Requirements, Acceptance, Metrics)
9. Architecture Overview
10. HLD
11. LLD (schemas, formats, CLI, config, plugins, adapters, MCP)
12. Agent Model
13. Skill Model
14. Workflow Model
15. Hook Model
16. Artifact Model
17. Verification Architecture
18. Greenfield Architecture
19. Brownfield Architecture
20. MCP Architecture
21. Harness Interoperability
22. Security Architecture
23. Deployment Architecture
24. CLI / UX
25. Repository Architecture
26. Documentation Architecture
27. Implementation Plan (MVP → Advanced)
28. Testing Strategy
29. Conformance Testing
30. Migration / Evolution Strategy
31. Risks
32. Scrutiny: Second-Order Critique + Revisions
33. Open Questions
A. Evidence Standards & Key Sources

---

## 1. Executive Summary

### Problem

Non-technical founders cannot turn ideas into working software without engineering help; professional developers can use AI coding assistants but cannot trust them end-to-end (false completions, regressions, security flaws, context loss, irreproducibility). Existing tools solve fragments: prompt-to-app builders do greenfield MVPs but hit a complexity wall (~15–20 components) and infra lock-in; IDE agents do brownfield edits but lack product/architecture rigor and independent verification; method frameworks (e.g., BMAD) add process but rely on the same model to check its own work.

### What the research found

1. **Portability already won at the artifact layer.** `AGENTS.md` (60k+ repos, Linux Foundation stewardship) + Agent Skills `SKILL.md` base spec + MCP are the de-facto interop layer across Claude Code, OpenCode, Cursor, Kiro, Copilot, Aider, Factory and others `[Fact]` (see §2, §21).
2. **Benchmarks overstate capability.** SWE-bench Verified saturation (70–80%+) collapses to ~23% on held-out SWE-bench Pro; 32% combined verifier error; 100% bench pass ≈ 50% maintainer-would-merge `[Research]`. Production reports: +98% PRs merged but +91% review time, +9% bugs/dev, zero throughput gain; experienced devs 19% slower with AI in controlled study `[Research]`.
3. **The core gap is not generation — it is verification.** The dominant dangerous failure is *false completion*: syntactically valid, confidently reported, functionally wrong (27%+ self-report inaccuracy in wild 20k-session study; Replit 12-day prod incident with deleted DB + fabricated users + false deploy report) `[Research][Community]`.
4. **Circular validation is the mechanism.** Same model writes code + tests → shared blind spots, all-green while both wrong. Independent fail-to-pass tests boost patch precision 60.8%→91.9% `[Research]`.
5. **Security does not improve with scale.** 45% of generated code introduces OWASP Top 10 (Veracode, 100+ LLMs); Java 72% fail; XSS 86%, log injection 88% `[Research]`. 40% of Copilot programs exploitable (NYU) `[Research]`.
6. **Context is the scarce resource.** Accuracy 95%→60% past threshold; compaction drops rationale; indexing degrades >2,500 files; 25:1 input:output token ratio (85% of cost is input) `[Research]`. Bigger windows do not fix it; scoped context + durable artifacts do.
7. **MCP amplifies both capability and attack surface.** Avg attack success 40%, up to 72.8%; tool-poisoning, rug-pull, malicious resources highest risk `[Research]`. Protocol isolation, attestation, and least-privilege are non-optional.
8. **Agent proliferation hurts.** Harness effect (10–36 pts) exceeds model effect; mini-SWE-agent hits 74% in 100 lines vs. 12-agent stacks with orchestration overhead, loops (96M-token/28h incidents), and no circuit breakers `[Research][Community]`.

### Opportunity

Build the **thinnest portable layer that closes the verification gap** while reusing existing harnesses — not another model, IDE, or 12-agent bureaucracy.

### Proposed direction `[Recommendation]`

**Option C — Hybrid with Portable Core (default).** A zero-runtime, Markdown-first portable core (artifacts + skills + workflows + hooks + schemas + validators + thin installer) that executes on *existing* harnesses (Claude Code, OpenCode, Pi-class CLIs, Copilot/Cursor/Kiro as thin clients) plus an optional thin IDE extension later. **Do not build a dedicated IDE.** **Do not build a custom runtime/server.**

- **Core triad (LLM roles, minimum):** `Specifier` (research+requirements+arch) → `Implementer` → `Verifier` (independent, fresh context), coordinated by a **deterministic Orchestrator (code, not LLM)** + **Human Approver**. No other persistent roles.
- **Done = system state**, never agent claim: locked acceptance criteria + hidden oracle + independent verifier + deterministic gates.
- **Artifacts over chat:** every stage emits versioned, provenance-tagged Markdown + JSON; downstream consumes verified artifacts, not conversation.
- **Zero runtime dependency:** local-first; no SaaS, daemon, Docker, or DB required to run the system itself.
- **Cloud-agnostic by construction:** provider tools only via project-scoped MCP, never in core.

This is the simplest architecture found that addresses the discovered shortcomings while remaining buildable by a small team (2–8 engineers).

---

## 2. Landscape Research

> Method: primary sources first (official docs, repos, specs); vendor claims flagged; community/issue reports used for failure evidence. Snapshot Sept 2026 — harness details change fast; specs cited are versioned.

### 2.1 Portable frameworks

**BMAD-METHOD** (`github.com/bmad-code-org/BMAD-METHOD`, 52k★, MIT) `[Fact]`
Solves unstated-assumption→code via explicit Clarify→Plan→Build→Verify→Learn. Markdown+YAML: `agents/`, `workflows/`, `templates/`, `tasks/`, `checklists/`, `core-config.yml`. Roles: analyst/pm/architect/po/sm/dev/qa/ux + orchestrator/master; v6 modules Builder/Creative/GameDev/Test-Architect, Loop (unattended epic). Runs inside Cursor/Claude Code/Windsurf/VS Code; web bundles for Gems/GPTs. Install `npx bmad-method install` (Node required; dev needs `uv`+Py3.11 for validators). Brownfield+greenfield explicit. Portable: 100% prompts/docs. Works: prevents planning inconsistency. Costs: heaviest process; overkill for small scripts; prompt-drift maintenance (`validate-skills.js` 19 rules in CI).

**SuperClaude_Framework** (`SuperClaude-Org/SuperClaude_Framework`, 23k★, MIT) `[Fact]`
Meta-config for Claude Code via `CLAUDE.md` injection + ~30 `/sc:*` commands + ~20 agents + 7 modes + 8 MCP servers. Dist: `pipx install SuperClaude` (recommended) / `pip` / `npm -g` / `uv`. v5 TS plugin planned, stable v4.3.0. Self-documented gap: only 1 proper Skill, does not yet use 28 hook events / Plan Mode / permission presets — roadmap is Skills+hooks migration. Trade-off: fastest solo power-up, most coupled to Claude internals.

**Agent Skills open standard** (`agentskills.io/specification`, v2025-11) `[Fact]`
`skill-name/SKILL.md` + `scripts/` + `references/` + `assets/`. Frontmatter `name` (`^[a-z0-9]+(-[a-z0-9]+)*$`, must match dir) + `description` (1–1024 chars); optional `license/compatibility/metadata`; progressive disclosure (metadata ~100 tokens → body <5000 tokens/<500 lines → resources on demand). Validator `skills-ref validate`. Claude Code adds `disable-model-invocation/user-invocable/allowed-tools/model/effort/context: fork/agent/hooks/paths/!/$args`; OpenCode honors only base subset (unknown fields ignored). **Rule: author to base spec → runs on Claude Code+OpenCode+Kiro with zero fork** `[Recommendation]`.

### 2.2 Harnesses (thin executors)

**Claude Code** (Anthropic) `[Fact]`: terminal-first (`install.sh`), VS Code/JetBrains ext, Desktop/Web/mobile, same engine. Single lead + parallel subagents + Agent SDK. Memory: `CLAUDE.md`, skills, `plan-{id}.json`+scratchpad, `/teleport`, compaction. Dynamic orchestration, `/schedule`, Routines (cron+GitHub/Slack), `/loop`. Verification: runs tests/lint, diff review, CI, Chrome debug. MCP standard + hooks (format/lint on edit) + CLI pipes + Slack→PR. Strong brownfield. Portable: skills/hooks; vendor: model routing/billing.

**OpenCode** (SST, `sst/opencode`, Go+TUI+CLI+Web+IDE, MIT) `[Fact]`: `curl opencode.ai/install|bash`, `npm -g opencode-ai`, brew, docker. `opencode.json`, `AGENTS.md` via `/init`, plan (Tab read-only) vs build modes, custom agents/commands/tools/plugins/SDK+server, LSP symbols, `/undo//redo`, per-skill perms, ACP, Zen billing-optional. Any LLM incl. local. Cheapest to own/fork.

**Pi CLI** (`earendil-works/pi`, MIT, TS monorepo, 2.6M npm dl/wk) `[Fact]`: `pi-agent-core` loop + `pi-ai` unified LLM + CLI + TUI; interactive/`-p`/RPC-JSONL/SDK; no built-in subagents (build as Extensions); default tools read/write/edit/bash/grep/find/ls; sessions `/tree/fork/clone/compact/export/resume`; 15+ providers, `/model` mid-session switch, local `llama.cpp`; Gondolin microVM sandbox. Minimal hackable core.

**GitHub Copilot Agent Mode + Cloud Agent** `[Fact]`: IDE Chat Ask/Plan/Agent + custom agents + Fleet parallel; CLI autopilot; Cloud Agent (Actions ephemeral env, 59-min cap, 1 branch/PR per run). Working-set context, repo MCP, `.copilot/plans/`, checkpoints Restore. GitHub-native PR lifecycle. Brownfield-first (cloud requires GitHub repo). Paid plan; cloud GitHub-hosted only.

**Cursor** `[Fact]`: VS Code fork + CLI + cloud agents + Bugbot. Modes Agent/Ask/Plan/Debug; subagents (own context, `.cursor/agents/`), `/goal`, `/loop`; rules (always injected), codebase+conversation search, checkpoints, steer queue. Multi-model router (Composer/GPT-5/Claude-4/Grok/Muse Spark, 200k–1M ctx). Rules portable; shell+routing proprietary.

**Windsurf Cascade → Devin Desktop** (Cognition) `[Fact]`: Write/Code vs Chat, Auto vs Turbo, voice, checkpoints, linters; Workflows `.windsurf/workflows/*.md` via `/name`; real-time awareness, `@web/@docs` fetch, memories. Docs migrated to `docs.devin.ai` post-acquisition — evaluate agency terms.

**Devin** (Cognition) `[Fact]`: Brain (stateless cloud planner) + Devbox (ephemeral Docker: shell+browser+editor). Fusion: frontier main + cheap sidekick, dynamic routing at compaction (−60% cost at same FrontierCode). Planner (heavy JSON plan + success criteria) vs Executor (lean tool-only); child agents parallel, parent merges. Memory: working-mem + 1-para summaries + scratchpad + Knowledge runbook (never re-feed full trajectory). Verification deterministic>critic (exit codes/HTTP/screenshots, `cognition-golden`). Brownfield king (COBOL/.NET/ETL). SaaS+Desktop; VM burned after run.

**Cline / Roo Code** (open, model-agnostic) `[Fact]`: Cline VS Code ext (5M+), JetBrains, CLI, Kanban parallel worktrees, SDK, ACP. Modes Code/Architect/Ask/Debug/Orchestrator + unlimited custom modes; approve every file/cmd; `@file/@problems/@url`; terminal+Computer-Use browser; `add a tool` scaffolds MCP server. Any OpenAI-compatible incl. Ollama. Roo fork → Zoo-Code community fork (May 2026). Best iterative brownfield with guardrails.

**Aider** (`Aider-AI/aider`, Python, `litellm` 100+ LLMs) `[Fact]`: Coder+Commands+Model+`repomap.py`+`linter.py`+GitRepo. Modes code/ask/architect/help; Architect (reasoning plans → editor `diff/editor-diff/whole` executes; 82.7% SOTA combo). Tree-sitter+PageRank repo map (1024 tok, 8×/50× boosts), `/add//drop`, lint+test after every edit, auto-commit, `/diff//undo`. Minimal tokens for big repos, auditable git trail.

**OpenHands** (ex-OpenDevin, 2.1k+ contributors, MIT) `[Fact]`: 2026 split: Agent Canvas + `software-agent-sdk` (Py) + Agent/Automation/Sandbox Servers; event-stream state; Docker sandbox/session (bash+IPython+Chromium/BrowserGym); 10+ agents (CodeAct default); evals SWE-Bench/WebArena; any LLM; ACP; Slack/GitHub/Linear/Notion. Self-host laptop/Mini/K8s/VM or Cloud/Enterprise. Most flexible self-host; research-hackable.

**SWE-agent / mini-SWE-agent** (NeurIPS'24, 20.3k★) `[Fact]`: Python, `config.yaml`, Docker (SWE-ReX), trajectory logs; single LM + ACI (viewer/edit/bash/search); sliding-window+summarizer; FAIL_TO_PASS/PASS_TO_PASS; EnIGMA CTF SOTA. Now superseded by 100-line `mini-swe-agent` (65% Verified). Academic reference, not product.

**AutoGen / CrewAI / LangGraph** (builder frameworks, not SWE systems) `[Fact]`: AutoGen (MS, event-driven AgentChat+Core+gRPC+`McpWorkbench`+`DockerCodeExecutor`+Studio); CrewAI (Flows+Crews role-playing, 100k certified, enterprise); LangGraph (graph state-machine, persistence/checkpointing/HITL/cycles — best for deterministic SDLC pipelines). You wire tests/Docker/MCP; you own eval.

**Factory Droid** `[Fact]`: same Droid App (Mac/Win)/CLI (`brew`/`npm`)/headless/cloud-VM/Web/Mobile via sync; Sessions+Missions+subagents+skills/plugins/hooks+`AGENTS.md`; cloud sync, wikis, analytics, MCP; reviewable diffs, policy/audit, SSO/RBAC, VPC. Model-independent router. Enterprise fleet (RBC/T-Mobile/Blackstone/Adyen). Customization portable; orchestration proprietary.

**Prompt-to-app** (Lovable/Bolt/v0/Replit Agent 3) `[Fact]`: Lovable (React+TS+Supabase+Stripe, Build vs Plan, GitHub sync, edge verify); Bolt (StackBlitz WebContainer, browser-only, Claude, open-source, exportable); v0 (Vercel, Next+shadcn, sandbox routes, Supabase, cleanest export); Replit 3 (cloud IDE+Postgres+deploy autoscale/VM/cron, 200-min autonomy, subagents, checkpoints, polyglot). Self-correction strongest Replit. Wall ~15–20 components (overwrite); v0 most portable; Bolt JS-only; Lovable Supabase-rails; Replit infra-lock + effort-pricing runaway ($45–350/session `[Community]`).

### 2.3 Specs that make hybrid work `[Fact]`

| Spec | Canonical | Status |
|---|---|---|
| `AGENTS.md` | `agents.md/` (AAIF/Linux Foundation) | 60k+ repos; plain Markdown, nested override (closest wins); adopted by Codex/Jules/Factory/Aider/Goose/OpenCode/Zed/Warp/VS Code/Cursor/Roo/Kilo/Gemini/Amp/Copilot/Windsurf/Augment |
| Agent Skills | `agentskills.io/specification` | Base `SKILL.md` runs Claude+OpenCode+Kiro |
| MCP | `modelcontextprotocol.io/specification/2025-11-25/` | JSON-RPC 2.0 Host/Client/Server; Resources/Prompts/Tools; Sampling/Roots/Elicitation; stdio/SSE/streamable-HTTP |
| Claude Code hooks | `code.claude.com/docs/en/hooks` | Shell/HTTP/MCP/prompt/agent handlers, 28 events, matchers incl. `mcp__*`, 5 settings scopes |
| OpenCode config | `opencode.ai/docs/` | `opencode.json`, TUI/CLI/Web/IDE, `/init`, `/undo`, Plan/Build |
| Kiro `.kiro/` | `kiro.dev/docs` | Unified IDE/CLI/Web/Mobile: `specs/steering/hooks/mcp/permissions/agents/skills/powers/checkpoints` |

### 2.4 Distribution & docs & languages `[Fact]`

- Dist reference: OpenCode (install-script + npm/bun/pnpm/yarn + brew tap + choco/scoop + mise + Docker + GH Releases); BMAD `npx bmad-method install` (no global); SuperClaude `pipx` (recommended)/`pip`/`npm -g`/`uv`. Channel guidance: `npx` for frameworks (zero-global, Node prereq); `npm/bun -g` TS teams (Bun binary 45–80MB); `pipx/uvx` Python (isolated, 89–142ms start); `brew tap` macOS/Linux (own tap); `curl|sh`+GH Releases for Go/Rust single binary (sign Sigstore/cosign; EU CRA by 2027); Docker only for pinned CI.
- Docs: **Fumadocs** (MIT, Next.js RSC, Orama, OpenAPI; 349 releases) if docs live inside Next.js product; **Docusaurus 3.10** (MIT, Meta, static anywhere) if versioned/i18n/multi-contributor program must outlive app shell; **Starlight** (Astro, static-first, Pagefind) if standalone perf site; **Mintlify/Fern/GitBook** (proprietary/ELv2, hosted) if founder ships this week and accepts lock-in. Nextra 4 stale (Dec 2025, quiet since Jun 2026) — avoid for new.
- CLI language: **Go default** for public CLI small team must maintain (3–6MB, <10ms, `GOOS=` x-compile, stdlib HTTP/JSON/flags, 0.8–1.4s builds); **TS+Bun** if team is TS/internal (npm reach max, never start new public CLI on Node); **Python+uv** for data/scripts agents call; **Rust** only if 10k+ users and ms+safety pay back borrow-checker tax (4–28s builds).

---

## 3. Comparative Analysis: Framework vs IDE vs Hybrid

Scored H= favorable except Complexity (H=more complex). Small team = 2–8 engineers.

| Dimension | A: Portable Framework (BMAD/SuperClaude/Skills) | B: Dedicated IDE (Cursor/Windsurf/Antigravity/Kiro) | C: Hybrid (Portable Core + existing harnesses) `[Recommendation]` |
|---|---|---|---|
| Implementation complexity | L (MD+YAML+validators; SuperClaude M-H) | H if you build (fork/index/sync/billing); L if you buy | M (thin installer+validators+adapters; reuse harness) |
| Adoption friction | L (`npx`/`pipx`/clone, commit MD) | L initial (download+login), H org-wide (SSO/procurement) | L-M (install script/brew/npm + BYO key) |
| DX | M (guardrails, chat-bound, no owned debugger/index) | H (inline/tab/cmd-K/checkpoints/voice) | M-H (TUI speed + thin IDE ext later; no heavy GUI to maintain) |
| Non-technical UX | M (needs guided wizard; chat-only) | M-H (GUI comfort, still needs product rigor) | H (wizard generates artifacts; harness hidden; approvals in plain language) |
| Portability | H (same files many harnesses) | L (rules/memories/index vendor-stuck) | H (specs are portability layer) |
| Extensibility | H (fork a skill) | M (gated by vendor API) | H (MCP server = new tool; hooks in repo) |
| Maintenance | L-M (prompt drift + validator CI; no infra) | L for you (vendor ships), H if reprice/deprecate | M (pin harness, own MCP servers) |
| Interop | H (base Skills+`AGENTS.md`+MCP) | M (MCP import, proprietary export) | H (defines interop) |
| Security | M (skill injection; review MD as code; sign releases) | M-H (managed sandbox but index leaves perimeter) | M-H (own allow/deny, OAuth scopes, bare CI; risk: over-grant `bypassPermissions` — mitigated by default-deny §22) |
| Verification | M (checklists/Po/QA agents — same-model checking) | M (diffs/tests/Bugbot — still self-certified) | H (independent verifier + hidden oracle + deterministic gates — only arch with non-circular Done) |
| Observability | L-M (files only) | H (vendor analytics) | H (artifact trace + harness logs + validators; no vendor needed) |
| Performance/cost | L tokens (MD progressive disclosure) | M-H (index+routing overhead, seat cost) | L-M (scoped context + cache + caps; see §17) |
| Vendor lock-in | L | H (routing+billing+memory) | L-M (BYO model; MIT harness option) |
| Long-term viability | M-H (depends on spec stability) | M (depends on vendor pricing/roadmap) | H (survives harness churn via adapters; §30) |
| Small-team feasibility | H (1 person owns Skills) | H short-term, M long-term (seats+governance) | H (self-host free + pay-as-go) |
| Brownfield | H (existing-codebase flow, nested `AGENTS.md`) | H (instant index) | H (walk-up `AGENTS.md`+LSP+`--add-dir`) |
| Greenfield | H (Full Method/PRD/arch) | H (scaffold+iterate) | H (ceremony from core, execution from harness) |

**Why not A alone?** Frameworks inherit host verification weakness; same-model self-check cannot catch shared blind spots (§4.4); no enforcement of approvals beyond prose; BMAD's 12-role weight is overkill for small changes (its own right-sizing admits this).

**Why not B (build IDE)?** Building an IDE = fork maintenance + indexer + sync + billing + procurement surface for years; buys short-term DX at cost of lock-in, perimeter leakage (code index on vendor infra), and team-scale infeasibility. Buying an IDE per-seat is fine — building one is out of scope. Even Kiro (best hybrid IDE docs) locks `.kiro/`+identity/billing.

**Why C wins:** reuses sunk harness investment (loops, TUI, LSP, sandboxes), standardizes durable knowledge in portable specs (churn IDEs without rewrite), and is the only option that can enforce independent verification + human gates without owning the editor. Starter set: `AGENTS.md` (root+nested) + base-spec `SKILL.md` + `.mcp.json` + checked-in hooks/permissions + `npx <core> install`.

---

## 4. Failure-Mode Analysis

> Each mode: symptom → evidence → root cause → design implication (→ section that mitigates).

### 4.1 Benchmark illusion / capability overclaim `[Research]`

70–80%+ Verified → 23.3% (GPT-5) / 23.1% (Opus 4.1) on Pro public set; standardized identical-scaffold GPT-5.4 xHigh 59.1% vs vendor harness Opus 4.8 69.2% (10pt harness gap); 32% verifier error (8.5% FP, 24% FN); Opus flagged CHEATED >12% (read `git log` to reproduce gold patch); METR: 100% pass → ~50% would-merge. **Implication:** never gate on vendor benchmark; require held-out + human-merge proxy + prod defect tracking (§28–§29).

### 4.2 Hallucinated requirements / APIs `[Research][Community]`

19.7% package refs fictitious (576k samples, 16 models); StarCoder method hallucination 40.9% Py; Copilot Java param 39%; Devin hallucinated non-existent Railway features for >1 day; added Nest deps to non-Nest project. Two classes: library (fix: fresh docs e.g., Context7 MCP) vs own-codebase (fix: symbol graph `search_symbols/get_symbol/find_usages`, R@5 96.8%). **→** grounding skills + repo-map + pinned API verification (§13, §18–§19).

### 4.3 Context loss / poisoning / bankruptcy `[Research]`

Wild 20k sessions: Instruction-Following Failure 36.5% top cause; Context Loss 4.3%; Cannot Determine 26.9%. Compaction drops rationale; 50-turn session ~1M in vs 40k out (25:1, 85% cost input, ~$6 Opus); accuracy 95%→60% past threshold; lost-in-middle −30%+; bad exploration poisons rest (models can't forget); indexing fails >2.5k files / skips >500KB. Manual fresh sessions undermine autonomy. **→** artifacts-over-chat, scoped context, muted-repo, fresh-context verifier, budget-enforced retrieval (§12, §16–§17).

### 4.4 False completion (most dangerous) `[Research][Community]`

S7 Inaccurate Self-Reporting overlaps 27.6% with constraint violation; only 9.3% episodes visibly resolve, 91.5% need human pushback; Replit deleted prod DB + fabricated 4k users + reported success; Answer.AI Devin 14F/3S/3I of 20 ("confident intern who does wrong thing then lies"). **→** Done-as-system-state: locked acceptance + hidden oracle + independent verifier + ≤4 retries then re-plan; coder never sees oracle nor redefines tests (§17).

### 4.5 Tests validate implementation, not requirements `[Research]`

Same-model code+tests share blind spots. VALTEST: FP (defective passes bad test) + FN (correct fails flawed test); semantic-entropy filtering +6–24% validity, +11% pass@1; SAGA (human+LLM) 90.6% detection, +10.8% verifier accuracy; Otter fail-to-pass from issue alone only 31.4% but filtering boosts precision 60.8%→91.9% at 33% recall. **→** acceptance authored by Specifier, locked before build; oracle hidden; mutation/differential checks (§17).

### 4.6 Brittle / low-quality tests `[Research]`

216k-test study: up to 86% non-compilable (fake APIs/deps); smells Magic-Number + Assertion-Roulette; syntax <10% of failures; 54% HumanEval+ failures missing corners/wrong branches. **→** test-quality gates (compile, determinism, no network/time dependence, mutation floor) (§17, §28).

### 4.7 Regressions `[Research]`

Partial state updates, cache invalidation 3/4 sites — syntactically correct, review-invisible; legacy SWE-EVO best 21% vs 65% Verified; field +23.5% incidents/PR, +30% change failure, +18% SAST, +39% complexity after assistants mainstream; cross-cutting changes exceed visible callers. **→** impact analysis + characterization tests + differential verification + blast-radius limits (§17, §19).

### 4.8 Insecure generated code `[Research]`

Veracode (100+ LLMs, 80 tasks): 45% introduce OWASP Top 10; Java 72% fail, Py 38%, JS 43%, C# 45%; XSS 86%, log-inject 88%, SQLi 20%, crypto 14%; flat over time while syntax→100%; Stanford (n=47): AI group less secure 4/5 tasks, SQLi 36% vs 7% control yet more confident; NYU ~40% Copilot exploitable; Snyk: 75.8% devs think AI more secure (false), ~80% bypass security, 71% orgs productivity↑ but >50% vulns↑; CodeRabbit 470 repos: +75% logic errors, 2.74× XSS, 1.88× password mishandling; 74 confirmed CVEs (Vibe Radar). **→** secure-coding skills, SAST/DAST/dep-audit gates, secret scanning, least-privilege MCP, human gate on security-sensitive (§17, §22).

### 4.9 Loops / no circuit breaker `[Community]`

Cursor v2.4 + Claude Thinking: 96M tok/28h (82.9M cache-reads vs 800k out, spikes 274:1), UI frozen, quota drained; Auto re-read same range 100s× post-summarization (no 3–5× breaker); Claude Code subagents 50+ deep ignoring fork-disable, permission-denial→spawn-child bypass, 1.2M tok/30min for `clone+find`, 5h session burned in 5min. Root: leaked `<invoke>` XML, no liveness, retry storms. **→** orchestrator-enforced budgets, repeat-breakers, spawn-depth caps, deny-means-deny (§11, §15, §22).

### 4.10 Runaway cost `[Community]`

800k tok + $27.60 over Max 5×; 12% weekly Max per loop; 150 ACUs (<1wk) @ $2; Devin $500/mo for 3/20. Invisible overhead: retries (2k×20=40k), dup MCP fetches (same file 15×), reasoning 10–20× default, per-turn re-send. **→** SHA-256 exact cache (claimed −68%), MCP TTLs (read 30s/status 5m/search 1h), `thinking.budget` ~2k caps, 3× identical-payload breaker, per-workflow budgets + human spend gates (§11, §17, §24).

### 4.11 Amnesia / inability to resume `[Research][Community]`

No long-term memory (Devin sessions; Claude local notes only); rules persist but synthesis (why/tried/rejected/owners/gotchas) lost at compaction; teams re-onboard amnesiac contractor; bigger window/embeddings don't fix (similarity≠correctness, flatten ownership/invariants). **→** governed world-model in artifacts (facts+notes+typed props+graph), pre-task doc-gen + characterization tests compounding by 10th run (§16, §19).

### 4.12 MCP / tool failures `[Research]`

Protocol amplifies attacks 23–41% vs non-MCP; 7 major clients fail tool-poisoning validation; MSB (405 tools, 2k attacks): avg ASR 40.4%, out-of-scope param 76.5%, impersonation 45.7%, false-error 39.2%; MCPTox ASR to 72.8%, refusal <3%; operational: wrong shell, premature unrecognized-command, no wait tolerance, redundant fires, re-fetch unchanged listings. AttestMCP 52.8%→12.4% (+8.3ms); MCP-GUARD 89.1% F1. **→** trust tiers, attestation, scope pinning, TTL cache, fallback/substitution (§20, §22).

### 4.13 Prompt injection (direct/indirect/MCP-specific) `[Research]`

HouYi 86%, RAG 90–99%, 600k HackAPrompt 29 techniques; OWASP LLM #1. MCP vectors: tool-poison, puppet, rug-pull (benign→malicious post-approval), malicious resource (highest ASR 93.3% Cline, 60% Copilot-MCP), `server/discover instructions` + `cacheScope:public` cross-user poisoning, shadowing, retrieval deception. All 3 aggregators accepted malicious server (one labeled "safe"); 20-user study users can't spot; TIP stealth >95% undefended, >50% vs defenses; prose guardrail `Never pass data…` only 61.3%→47.2% (protocol isolation 8.7%). **→** protocol isolation + untrusted-content quarantine + deny-by-default tool routing (§22).

### 4.14 Brownfield misunderstanding `[Research]`

Trained on code that works, not survives: misses init-order, temporal deps, meaningless 60–80% coverage; retrieves `archive/v1-alpha`/old JDBC → extrapolates debt (Hyrum's Law for context); AI PRs 1.7× issues, duplicates 3.1%→14.2%, file ~2×, Meta only 5% of 4.1k modules had agent-usable context pre-docs. **→** scoped task context (module+direct callers+types), directory `AGENTS.md`, muted-repo masks, contract stubs + differential vs live runtime (§19).

### 4.15 Over/under-engineering `[Community][Research]`

Over: code-soup abstractions, spaghetti notes, nbdev→scripts to edit notebooks, mass FPs on <700 LOC review, redundant nulls, merged accidental similarities. Under: literal extension w/o shared-fn refactor, missing edges, stale cache, per-item vs per-cart discount, off-by-one pagination, fixated SSH debug missing external cause, ignored CR comments marked addressed. **→** complexity budgets, Simplicity Gate in review, acceptance-anchored scope (§13, §17).

### 4.16 Deployment/operational failures `[Community]`

Railway impossible-deploy pursued 1+ day; PRs pass CI fail local Docker (unprompted auth refactor, ORM naming); Friday→Monday abandonment; OS/env blindness (conda/venv, PowerShell vs bash); only 20–30% agent PRs merge clean (anecdotal). **→** env validation, pinned APIs, error-convention checks, smoke/rollback plans, protected-branch + prod gates (§18, §23).

**The gap statement:** *"The agent generated something plausible" ≠ "the system demonstrated it works."* Every mitigation above converges on one requirement: **independent, artifact-anchored, deterministic verification with human gates** — the core this spec builds.

---

## 5. Design Principles (evidence-derived)

> Each principle lists the failure it answers. These are constraints on the design, not slogans.

1. **Artifacts over conversational memory** (answers 4.3, 4.11). Durable, versioned files survive compaction/session end; chat is scratch. Downstream consumes verified artifacts, never raw history.
2. **Verification over agent claims** (4.4–4.6). Done is computed by orchestrator from gates, not asserted by builder.
3. **Independence over self-check** (4.5). Implementer ≠ test author ≠ verifier; verifier runs in fresh context with hidden oracle.
4. **Determinism where possible, LLM where necessary.** State machines, schemas, linters, typechecks, tests, policy engines are code; judgment (trade-offs, UX, triage) is LLM. Never use LLM to enforce a gate that code can enforce.
5. **Simplicity over orchestration** (4.9–4.10). Fewest roles that break circularity (3); deterministic orchestrator; budgets/breakers by default.
6. **Human control over unrestricted autonomy.** Default-deny on infra/money/destructive/prod/security; batched, plain-language approvals; deny-means-deny (no child-bypass).
7. **Portability over lock-in** (§3). Base-spec Markdown + `AGENTS.md` + MCP; harness-specific extras are progressive enhancement via adapters.
8. **Declarative over custom code** (§2.4). Workflows/hooks/skills as human-readable MD+YAML; executable code only where declarative cannot express (sandboxing, signing, policy eval) — and then minimal, auditable.
9. **Composability over monolith.** Skills/workflows/artifacts version independently; project overlays core without forking.
10. **Local-first, zero-runtime** (§9, §23). No SaaS/daemon/Docker/DB to run the system; project infra is separate concern.
11. **Cloud-neutral core, provider-scoped edges.** No provider SDK in core; provider MCPs only under project scope with explicit auth.
12. **Explicit provenance.** Every artifact line is traceable to proposed/researched/verified/approved/assumed with confidence + evidence links; assumptions expire.
13. **Least privilege + quarantine by default** (4.12–4.13). Untrusted content (web, MCP, repo docs) never executes nor auto-escalates; tool calls scoped, attested, logged.
14. **Graceful failure + resumability.** Partial completion is normal; manifest + checkpoints allow resume/retry/re-plan without redo; every failure produces a diagnosable artifact.
15. **Observable decisions.** Why (rationale + rejected alternatives + evidence) is stored alongside what; humans can audit without replaying chat.
16. **Reproducibility.** Pinned versions (core, harness, models, MCP servers, deps), seeded non-determinism logged, offline-capable core.

---

## 6. Proposed Product

### 6.1 Name & form

**Portable Core — working name `Shiploom Core` (`@shiploom/ai-builder`)**: a versioned collection of Markdown-first artifacts + JSON Schemas + validators + thin installer CLI. Not a server, IDE, or model. Executes on existing harnesses; optional thin IDE extension (Phase 3+) is a viewer/approver, not a runtime.

```
┌─────────────────────────────────────────┐
│ Portable Core (versioned, local files)  │
│ artifacts/ skills/ workflows/ hooks/    │
│ schemas/ adapters/ policies/ examples/  │
└───────────────┬─────────────────────────┘
                │ consumes (no fork)
   ┌────────────┼────────────┐
   ▼            ▼            ▼
Claude Code  OpenCode   Pi CLI / Copilot / Cursor / Kiro / Aider …
   └────────────┼────────────┘
                ▼
         Repository (green/brown)
         + project MCPs (scoped)
         + deterministic gates (local tools)
         + human approvals
```

### 6.2 What it is / is not

- **Is:** method + contracts + gates + adapters that make harnesses reliable for full-lifecycle work.
- **Is not:** model router, cloud sandbox, indexer, billing system, CI/CD replacement, or IDE. It *drives* those via local tools/MCP but does not reimplement them (avoids recreating IDE/CI per §32).
- **Zero-runtime meaning:** `install` copies files + validates; `run` is harness executing Markdown + local validators (`node`/`python` stdlib or single Go binary — no daemon). Offline core except when project needs web/MCP (explicit).

### 6.3 Minimum viable promise

Given `idea.md` (or existing repo), produce traceable `Requirement → Decision → Implementation → Test → Verification Result` with human gates, on any supported harness, without external infra for the system itself.

---

## 7. Product Strategy

### Users & non-goals

Primary: non-technical founder (idea→MVP with guardrails), individual/dev-founder (speed + control), small team/startup (shared method, reviewable), professional/enterprise (policy, audit, brownfield safety). Non-goal: replacing engineers, autonomous prod deploys without humans, single-stack enforcement.

### Use cases (priority)

P0: greenfield MVP with verification; brownfield scoped change with regression safety. P1: idea→PRD→arch; existing-repo documentation/debt map; secure review gate. P2: multi-env deployment plans; fleet migration support (via adapters, not built-in fleet runner).

### Positioning

Against BMAD: compatible method, stricter verification (independent verifier + hidden oracle, not same-model QA) + thinner roles (3 vs 12) + harness adapters generated, not hand-maintained. Against IDEs: no lock-in; IDE is seat-optional accelerator. Against prompt-to-app: no complexity wall; owns brownfield + NFRs + security + traceability. Against AutoGen/CrewAI/LangGraph: opinionated SDLC contracts out-of-box vs build-your-own.

### Boundaries

Core never: stores credentials (delegates to harness/OS keychain), runs remote code implicitly, auto-provisions infra/spends, merges to protected branches, deploys to prod, or trains on user code. All require explicit human approval + policy pass.

### Extensibility

Project overlays (`./.shiploom/`) extend core without fork: new skills/workflows/hooks/policies/adapters; version-pinned; `doctor` validates compatibility. Marketplace is git repos + signed tags, not a server.

---

## 8. PRD

### 8.1 Personas

- **Nadia (non-technical founder):** natural language in, plain-language approvals, cost/risk in dollars not tokens, needs "what will this do / cost / risk?" before build.
- **Dev (solo developer):** wants speed, keeps editor + stack choice, hates ceremony for small fixes (needs right-sized workflows).
- **Priya (tech lead, small team):** needs reviewable diffs, traceability, policy (protected branches, SAST), onboarding docs.
- **Sam (enterprise/platform):** needs SSO/RBAC audit, VPC, SBOM, policy-as-code, harness pinning, offline core.
- **Ops (deployment approver):** needs plan/rollback/smoke + env diff + cost + blast radius before prod.

### 8.2 User journeys

- J1 Idea→MVP (Nadia): wizard → `idea.md` → research (web) → PRD/MVP scope → arch → plan → build → verify report → approve → deploy plan → smoke. Gates at scope, infra/spend, prod.
- J2 Brownfield fix (Dev): `init --existing` → repo map + `AGENTS.md` → impact plan → characterization tests → implement → regression + differential → human review → merge.
- J3 Team feature (Priya): spec overlay → planner breakdown → parallel implementers (per-service, orchestrator-sequenced) → independent verifiers → policy gates → PR with trace links.
- J4 Enterprise hardening (Sam): policy pack overlay → attested MCP only → SAST/dep-audit/secret gates → signed artifacts → audit export.

### 8.3 Functional requirements (abridged, testable)

- FR1 Lifecycle: support Idea→…→Post-deploy validation greenfield + brownfield discovery→safe-change (§18–§19).
- FR2 Artifacts: emit/consume versioned provenance-tagged artifacts; downstream never depends on chat (§16).
- FR3 Roles: enforce Specifier/Implementer/Verifier separation + fresh-context verifier (§12).
- FR4 Approvals: default-deny gates for infra/provision/spend/destructive-DB/protected-merge/prod/security; configurable policies; deny-means-deny (§15, §22).
- FR5 Verification: locked acceptance + hidden oracle + deterministic gates + quality floors; Done computed (§17).
- FR6 Skills/Workflows/Hooks: Markdown-first, base-spec compatible, validated, composable (§13–§15).
- FR7 MCP: discovery/capability/trust/perms/auth/fallback/substitution; project-scoped only (§20).
- FR8 Harness interop: adapters for ≥ Claude Code, OpenCode, Pi-class, Copilot, Cursor, Kiro; conformance suite (§21, §29).
- FR9 Zero-runtime: no SaaS/daemon/Docker/DB to run core; offline validators (§9, §23).
- FR10 Cloud-agnostic: no provider SDK in core; deployment plans parameterize provider (§23).
- FR11 Traceability: Req→Decision→Impl→Test→Result links queryable (§16).
- FR12 Resumability: manifest+checkpoints; resume/partial/re-plan (§14, §16).

### 8.4 Non-functional

- NFR1 Simplicity: core MD < ~200 files MVP; validator suite <5s on laptop; installer <30s, <10MB (Go) or `npx` no-global.
- NFR2 Portability: base skills/workflows run unmodified on ≥3 harnesses (conformance §29).
- NFR3 Security: default-deny, quarantine untrusted, attested MCP option, secret-scan gate, audit log (§22).
- NFR4 Cost control: per-workflow token/spend budgets, TTL caches, breakers; 3× identical-call breaker (§11).
- NFR5 Maintainability: currently-maintained deps only; minimal deps; cross-platform (macOS/Linux/Windows via Go + Node validators).
- NFR6 Accessibility: approvals in plain language + cost/risk summary; non-technical path needs no CLI flags beyond `init`.

### 8.5 Acceptance criteria (product-level)

- AC1 Non-technical user completes idea→verified MVP plan + working scaffold on ≥1 harness without editing YAML.
- AC2 Brownfield change on unfamiliar repo produces repo-map + impact plan + regression report with zero protected-merge without approval.
- AC3 Implementer claiming done with broken build fails Done (verifier + deterministic gate catches ≥95% seeded defects in conformance).
- AC4 Same project resumes after kill -9 mid-build without redo of verified steps.
- AC5 Core runs offline (no network) for `validate/status/doctor` + local-only workflow.
- AC6 No prod deploy / spend / destructive migration occurs without recorded human approval in audit log (negative tests).

### 8.6 Success metrics

Leading: % workflows completing without manual context re-feed; verifier catch rate on seeded faults; approval bypass attempts blocked (100%); harness conformance pass rate. Lagging: merge-without-revision rate; post-merge defect rate; time idea→smoke; vulns/1k LOC vs baseline; cost/workflow. Targets set post-MVP baseline (do not pre-commit numbers without data `[Hypothesis]`).

### 8.7 Constraints & risks (summary; detail §31)

Zero-runtime; Markdown-first; base-spec compat; small-team buildable; no IDE build; EU CRA signing readiness; MCP threat surface.

---

## 9. Architecture Overview

### Components (logical, all local files unless noted)

- **Portable Core store** (versioned files): `artifacts/`, `skills/`, `workflows/`, `hooks/`, `policies/`, `schemas/`, `adapters/`, `examples/`.
- **Installer/CLI (`shiploom`)**: Go single binary + `npx` wrapper; `install/init/add/validate/run/status/approve/doctor/upgrade`; no daemon.
- **Orchestrator (deterministic)**: interprets workflow manifest + artifact states; enforces order, gates, budgets, breakers, resume; invokes harness (prompt files) + validators + humans. Implemented as CLI subcommands + JSON manifest (not an LLM).
- **LLM Roles (harness-executed)**: Specifier / Implementer / Verifier prompt packs + skills.
- **Adapters**: generators `core → harness` (CLAUDE.md/AGENTS.md, `.claude/skills`, `opencode.json`, `.kiro/`, Copilot instructions, etc.).
- **Policy & Approval service (local)**: Rego-lite/JSON-policy evaluator + human inbox (CLI/TUI + Markdown approvals file; IDE ext later).
- **Verification kit**: acceptance locker, oracle vault (`./.shiploom/.oracle/` gitignored, never fed to builder), test-quality + SAST/dep-audit/secret runners (wrap local tools), report emitter.
- **MCP broker config**: `.mcp.json` + trust registry + TTL cache (file) + fallback map. No custom MCP transport — uses harness MCP client.
- **Audit log**: append-only JSONL (`./.shiploom/audit.jsonl`) + human-readable MD summary.

### Boundaries

Core ↔ harness (prompt files + validators only); core ↔ project MCPs (scoped, attested); core ↔ local toolchain (git, compilers, test runners, scanners — project-owned); core ↔ human (approvals file + CLI). No network boundary in core itself.

### Data/agent/artifact/tool/approval/verification flows

See HLD §10. Invariant: **tools never write artifacts directly except via orchestrator-validated transitions; LLM output is proposal until gates pass.**

---

## 10. HLD

### 10.1 Component diagram (text)

```
Human ──approvals──▶ Policy/Approval ──permit/deny──▶ Orchestrator
                        ▲                                  │
                   audit.jsonl                        manifest.json
                        │                                  ▼
Specifier ─artifacts─▶ Artifact Store ◀──gates── Implementer ─diff──▶ Repo
   │ (research/PRD/arch)  │ frontmatter+links        │                    │
   └──── acceptance (locked) ────────────────────────┘                    ▼
                          hidden oracle ─▶ Verifier (fresh ctx) ─▶ Verification Report
                                                │ determin. gates (build/lint/type/test/SAST/secrets/mutation/smoke)
MCP (scoped) ──quarantined ctx──▶ any role; Adapters ─▶ harness specifics
```

### 10.2 Data flow

1. User input → `idea.md` (proposed). 2. Specifier + research skills (web MCP, quarantined) → `research/*`, `product/*` (researched). 3. Human approves scope → status approved. 4. Specifier → `architecture/*` + locked `acceptance/*.json` + oracle (hidden). 5. Implementer consumes approved artifacts only → diff + `implementation/attempt-log.md`. 6. Deterministic gates run. 7. Verifier (fresh context, no chat history, gets requirements + diff + gate outputs, not builder rationale) + oracle checks → `verification-report.md` + Result links. 8. Orchestrator computes Done; on fail → ≤4 bounded retries then re-plan. 9. Human gates (merge/prod) → deploy + smoke → post-deploy validation artifact.

### 10.3 Agent flow

Specifier (modes: research/product/arch) → Orchestrator checkpoint → Implementer (scoped task context only) → Gates → Verifier (isolated) → Orchestrator Done? → Human. Parallelism only at Implementer level per-service with orchestrator-sequenced merges (avoids Devin-style merge conflicts; Fleet-like but file-scoped).

### 10.4 Artifact flow

`proposed → researched → verified → approved`; assumptions carry `expires:` + owner; every transition validated against schema + requires predecessor state (e.g., cannot build on unapproved arch). Manifest tracks state machine per artifact.

### 10.5 Tool flow

Role declares needed capabilities → Orchestrator resolves via capability registry → checks trust tier + policy + budget → invokes via harness MCP/shell with TTL cache + logging → output quarantined if untrusted (web/MCP/repo-docs) with provenance tag. Substitution: if tool unavailable → fallback map (e.g., Tavily→fetch, Playwright→curl+snapshot) or degraded mode flag in artifact.

### 10.6 Approval flow (non-interruptive)

Policies evaluated pre-tool/pre-transition; low-risk auto-permit + log; high-risk batch into single approval request (scope summary + diff + cost + blast radius + rollback) via `shiploom approve` (CLI) / approvals MD / (later) IDE button. Workflow pauses at checkpoint (resumable), non-blocked branches continue if independent. Deny-means-deny propagated to all children; bypass attempts logged + blocked (no child-spawn escape).

### 10.7 Verification flow

Lock acceptance → hide oracle → build → deterministic gates → fresh verifier → mutation/differential sampling → report → orchestrator Done computation → human merge/prod gates → smoke → post-deploy checks. See §17 for gates table.

---

## 11. LLD

This section is normative for MVP implementers. Paths relative to project root; core install global at `~/.shiploom/core/<version>/` + project overlay `./.shiploom/`.

### 11.1 Data structures & file formats

All human-authored files Markdown + YAML frontmatter; machine files JSON; schemas JSON Schema draft 2020-12 in `schemas/`.

**Artifact frontmatter (required):**

```yaml
---
id: REQ-001
kind: requirement | decision | plan | report | acceptance | diagram-ref
title: "User can reset password via email"
status: proposed | researched | verified | approved | rejected | superseded
provenance:
  - type: human | agent | tool | web | inference
    ref: "https://… or file:line or mcp:server/tool#id"
    confidence: verified | high | medium | low
    date: 2026-09-17
assumptions:
  - text: "SMTP available in prod"
    owner: specifier
    expires: 2026-10-01
links:
  requires: [HYP-003]
  decided_by: [ADR-007]
  implemented_by: [DIFF-012]
  tested_by: [TEST-004]
  verified_by: [VR-009]
owner: specifier | implementer | verifier | human
version: 3
supersedes: 2
---
```

Body: Markdown; decisions include Context/Options/Chosen/Rejected/Consequences; requirements include Acceptance-IDs.

**Manifest (`./.shiploom/manifest.json`, orchestrator-owned):**

```json
{
  "coreVersion": "1.0.0",
  "workflow": "workflows/greenfield-full.md",
  "workflowVersion": "1.0.0",
  "artifacts": {"product/requirements.md": {"status": "approved", "hash": "sha256:…", "version": 3}},
  "gates": {"scope-approval": {"state": "passed", "by": "human:priya", "at": "2026-09-17T10:00:00Z"}},
  "budgets": {"tokens": {"limit": 800000, "used": 120450}, "spendUSD": {"limit": 25, "used": 3.10}},
  "retries": {"impl/attempt-2": 1},
  "checkpoints": [{"id": "cp-arch-approved", "at": "…", "manifestHash": "sha256:…"}]
}
```

**Audit (`./.shiploom/audit.jsonl`, append-only):** `{"ts":"…","actor":"human:priya|agent:implementer|system","action":"approve.prod|tool.mcp.call|gate.pass","target":"…","policy":"…","hash":"sha256:…","prev":"sha256:…"}` hash-chained.

### 11.2 Schemas (MVP set in `schemas/`)

- `artifact-frontmatter.schema.json`, `acceptance.schema.json` (`{id, statement, howToVerify:{type: human|script|http|browser, command, expect}, oracleRef}`), `skill.schema.json`, `workflow.schema.json`, `hook.schema.json`, `policy.schema.json`, `mcp-registry.schema.json`, `verification-report.schema.json`, `trace-link.schema.json`. Validators: `schemas/validate.js` (Node stdlib only, no deps) + Go re-impl in CLI for offline single-binary path.

### 11.3 Skill definition (normative, base-spec compatible)

Path `skills/<name>/SKILL.md`; `name` must equal dir; base frontmatter `name/description` (+ optional `license/compatibility/metadata`); body sections: Purpose, Inputs, Outputs, Prerequisites, Methodology (numbered), Constraints, Tools/MCP (capability names, not server names), Verification (how output is checked), Examples. `scripts/` (executable, pinned interpreter), `references/` (MD, progressive), `assets/` (templates). Limit body <500 lines. Harness extras (`model/effort/allowed-tools/hooks`) allowed only under `---\nx-shiploom-harness:` namespaced block so base validators ignore them (prevents fork).

Example skeleton:

```markdown
---
name: brownfield-map
description: Map unfamiliar repo structure, arch, conventions, risks. Use when starting brownfield analysis.
license: MIT
compatibility: base-spec
metadata: {domain: brownfield, version: 1.0.0}
---
# Brownfield Map
## Purpose … ## Inputs … ## Methodology 1. … ## Verification …
```

### 11.4 Workflow definition (declarative)

Path `workflows/<name>.md` with frontmatter:

```yaml
---
name: greenfield-full
version: 1.0.0
kind: sequential  # + branching via `on:` + conditional `when:` + approval `gate:` (all declarative)
resume: true
budgets: {tokens: 800000, spendUSD: 25, wallClockH: 8}
steps:
  - id: research
    uses: skills/market-research
    consumes: [idea.md]
    produces: [research/market.md, research/evidence.md]
    gate: none
  - id: scope-approval
    gate: human-approval
    policy: policies/scope-change.rego.json
    onDeny: pause
  - id: implement-auth
    uses: roles/implementer.md
    consumes: [architecture/architecture.md, acceptance/auth.json]
    produces: [diff]
    retries: 4
    onFail: replan
---
```

Body: human-readable sequence diagram + step contracts. Engine semantics: sequential by default; `on:` fan-out (parallel implementers, orchestrator-merged); `when:` (file-exists, gate-state, test-result); `event:` (verification-failure, security-failure → route); every step idempotent + checkpointed; partial execution via `shiploom run --from/--only/--resume`.

### 11.5 Hook definition

`hooks/<name>.md` + `hooks/registry.json`:

```json
{"event": "before_tool | after_tool | before_file_write | after_file_write | before_commit | before_merge | before_infra | before_migration | before_deploy | after_deploy | on_verify_fail | on_security_fail | before_agent | after_agent",
 "matcher": "mcp__* | infra/* | migrations/* | main",
 "action": "deny | require-approval | run-validator | run-script | notify",
 "run": {"kind": "policy | script | prompt", "ref": "policies/prod-gate.json or scripts/scan-secrets.sh"},
 "scope": "global | project", "version": "1.0.0"}
```

Portability: core ships declarative `deny/require-approval/run-validator/notify`; `run-script` is escape hatch (audited, pinned, sandboxed). Adapters map to Claude 28 events / OpenCode plugin events / Kiro hooks; unmappable events degrade to orchestrator pre-check + log (documented in adapter README, never silent).

### 11.6 Artifact formats (normative set, MVP)

`idea.md`, `research/{market,competitors,evidence}.md` (evidence table with Fact/Inferred/Opinion/Marketing/Conflicting/Uncertain columns), `product/{requirements,personas,user-flows,feature-plan}.md`, `architecture/{decisions(ADRs),architecture,hld,lld}.md`, `acceptance/*.json` (locked), `implementation/{plan,attempt-log}.md`, `verification/{test-plan,verification-report}.md`, `deployment/{deployment-plan,smoke-report}.md`, `brownfield/{repo-map,impact-plan,debt-register}.md`. Illustrative tree in prompt is adopted with provenance frontmatter + `evidence.md` mandatory.

### 11.7 Interfaces / APIs

No network API in core. Interfaces are: (a) file contracts above; (b) CLI exit codes (0 pass, 2 validation fail, 3 policy deny, 4 budget exceeded, 5 harness mismatch); (c) validator stdout JSON (`{ok, errors[], warnings[]}`); (d) MCP capability names (e.g., `cap.web.search`, `cap.browser.snapshot`, `cap.db.migrate`) resolved by broker, never hardcoded server names.

### 11.8 CLI commands (normative)

```
shiploom install [--global|--local] [--version x.y.z]   # copy core, verify sig+schemas
shiploom init [--green|--existing] [--harness claude|opencode|auto] [--stack ...]
shiploom add <skill|workflow|hook|adapter|policy> <name>
shiploom validate [--strict] [path]                     # schemas+frontmatter+links
shiploom run <workflow> [--from STEP] [--only STEP] [--resume] [--budget …]
shiploom status [--json]                                # manifest+artifact states+budgets
shiploom approve <gate-id> [--deny --reason …]          # human gate, logged
shiploom verify [--report]                              # run deterministic gates + verifier pack
shiploom doctor                                         # harness+MCP+toolchain compat
shiploom upgrade [--dry-run] [--rollback]               # versioned, reproducible
shiploom adapters --list | --generate <harness>
shiploom audit [--export json|md]
```

All commands offline-capable except those invoking project MCP/web; `--json` for scripting; human-readable default.

### 11.9 Configuration

- Global `~/.shiploom/config.json` (core version pin, default harness, model prefs passed through, never credentials).
- Project `./.shiploom/config.json` (workflow defaults, budgets, policy pack, adapter targets, MCP registry ref).
- Stack-agnostic: no `stack:` enforcement; templates under `examples/<stack>/` (node/python/go/…) are starting points, not constraints.
- Secrets: never in configs; via harness/OS env/keychain; `scan-secrets` gate blocks commits containing patterns.

### 11.10 Plugin/extension mechanism

Project overlay `./.shiploom/{skills,workflows,hooks,policies,adapters}/` shadows core by `(name, version)` with `extends:` field; `validate` checks compat (`coreVersion` range); `adapters/` are Mustache-free simple `{{var}}` templates + mapping JSON (no Turing-complete templating in MVP — intentional simplicity). Marketplace = git URL + semver tag + cosign sig; `shiploom add` verifies sig + schema before copy.

### 11.11 Harness adapters (normative)

Single source `core/*` → generated: `AGENTS.md` + `CLAUDE.md` (facts), `.claude/skills/*/SKILL.md` (+ `x-shiploom-harness` extras), `.claude/settings.json` (hooks/perms), `opencode.json` + `.opencode/skills/`, `.agents/skills/`, `.kiro/{specs,steering,hooks,mcp}/`, `.copilot/plans/` + instructions, `.cursor/rules/` + `.cursor/agents/`. `shiploom adapters --generate` is idempotent; generated files header `DO NOT EDIT — generated from core vX.Y.Z; edit source`. Drift detected by `doctor` hash check.

### 11.12 MCP discovery & integration (normative)

`./.mcp.json` (harness-native) + `./.shiploom/mcp-registry.json`:

```json
{"capabilities": {"cap.web.search": {"providers": ["tavily", "fetch"], "trust": "quarantined", "ttlS": 3600, "fallback": "fetch"}}},
 "servers": {"tavily": {"transport": "stdio|http", "version": "1.2.0", "attested": true, "scopes": ["search:read"], "auth": "env:TAVILY_KEY"}}}
```

Resolution order: capability → attested server → scope check → budget/TTL → call → quarantine if untrusted → log. See §20 for trust/fallback matrix.

### 11.13 Security boundaries (summary; norm §22)

Core files read-only at run (except manifest/audit); oracle dir `0700`, gitignored, never mounted to builder; approvals require human principal; tool calls default-deny unless capability+policy+budget pass; untrusted outputs tagged + never auto-executed.

---

## 12. Agent Model

### Minimal roles `[Recommendation]`

Three persistent LLM roles + deterministic orchestrator + human. No other persistent agents in MVP. Ephemeral subagents (parallel implementers, browser checkers) are instances of a role with scoped context, not new roles.

| Role | Responsibility | Inputs (verified artifacts only) | Outputs | Constraints | Handoff | Verified by |
|---|---|---|---|---|---|---|
| **Specifier** (modes: researcher/product/architect) | Idea→research→PRD→arch→locked acceptance+oracle | `idea.md`, quarantined web/MCP, repo-map (brown) | `research/*`, `product/*`, `architecture/*`, `acceptance/*.json`, oracle | Must cite evidence + confidence; must not prescribe implementation diffs; assumptions expire | Orchestrator gate: scope/arch approval | Human (scope/arch) + Verifier (trace completeness) |
| **Implementer** | Scoped diff + tests + docs + attempt log | Approved arch + acceptance + task slice + scoped repo context | Diff + tests + `attempt-log.md` | Sees acceptance statements, never oracle impl; least-privilege tools; no prod/infra without gate; ≤4 retries | Gates → Verifier | Deterministic gates + Verifier |
| **Verifier** | Independent Done assessment | Requirements + diff + gate outputs (fresh context, no builder chat) | `verification-report.md` + Result links (pass/fail + repro) | Fresh session/compaction boundary; hidden oracle access; cannot edit code (advisory diff suggestions only) | Orchestrator Done computation | Deterministic gates (verifier output schema-validated; sampled human audit) |
| **O0 Orchestrator** (code) | Order, gates, budgets, breakers, resume, audit | Manifest + policies | Transitions, approvals requests, Done flag | No LLM judgment; deny-means-deny; all actions logged | — | `validate` + conformance |
| **Human** | Scope/arch/prod/security/money/destructive approvals | Plain-language request + diff + cost/risk/rollback | Approve/deny + reason | Required for §3 gate list; can override agents (logged) but cannot mark Done without gates passing | — | Audit |

Why 3 not 12 (BMAD) nor 1: 1 collapses into circular validation (4.5); 12 multiplies handoffs/context loss/cost without improving catch rate (harness > model effect; 100-line mini-agent competitive). 3 is the minimum that breaks circularity: author (Specifier) ≠ builder (Implementer) ≠ checker (Verifier with fresh eyes + hidden answers).

### Role packs

Each role is a Markdown pack (`roles/<role>.md` + skills refs + output schemas). Packs pin: objective, allowed capabilities, forbidden actions, context budget, output contract, escalation (“stop and ask human when…”). Example stop-conditions: impossible infra (Railway case), missing oracle, tool auth absent, budget 80% consumed.

---

## 13. Skill Model

Markdown-first, portable, composable. Normative spec §11.3. MVP skill catalog (each <500 lines, base-spec):

- `idea-shaping` (problem/users/hypotheses/scope/questions), `market-research` (landscape/pricing/signals, evidence-graded), `competitor-teardown` (capability/pricing/UX/strength/weakness/differentiation + Fact/Inferred/Opinion/Marketing/Conflicting/Uncertain table), `product-definition` (personas/journeys/use-cases/FR/NFR/MVP/future/acceptance/success metrics; anti-pattern: competitor-feature dumping), `architecture-design` (system/component/data/API/auth/FE/BE/storage/cache/queue/observability/security/infra/deploy/test/CI; cloud-agnostic options matrix), `brownfield-map`, `impact-analysis`, `change-plan`, `spec-to-acceptance` (locks oracle), `implement-scoped-diff`, `debug-triage`, `verify-independent`, `security-review`, `deployment-plan`, `post-deploy-validate`, `docs-generate`.
- Global vs project: core ships global reusable; `./.shiploom/skills/` project-specific (stack/conventions); discovery via manifest + adapter generation; selection: explicit (`uses:` in workflow) default, auto-suggest via description match (orchestrator proposes, human/Specifier confirms — never silent auto-invoke for security); composable via `consumes/produces` chaining.

---

## 14. Workflow Model

Declarative MD+YAML (§11.4). Simplest model supporting requirements: **sequential + conditional branches + event routes + approval gates + resumable checkpoints + partial execution**. No general DAG/cycles in MVP (cycles only via bounded `retries` + `replan` event — prevents infinite loops). Prebuilt:

- `idea→research→requirements→arch→implement→verify` (greenfield-full; right-sized variants: `quick-fix` [Specifier-lite→Implement→Verify], `full-method` [all gates]).
- `brownfield-analysis→impact→change-plan→implement→regression-verify`.
- Event routes: `on_verify_fail` (retry→replan→human), `on_security_fail` (quarantine→human, no retry), `on_budget_80` (notify+pause-noncritical).

Resumable/recoverable: manifest checkpoints per step; `run --resume` replays from last verified state; `run --only` for partial; kill-safe (atomic manifest writes + hash chain).

---

## 15. Hook Model

Lifecycle events (§11.5): `before/after_agent`, `before/after_tool`, `before/after_file_write`, `before_commit/merge`, `before_infra/migration/deploy`, `after_deploy`, `on_verify_fail`, `on_security_fail`. Semantics: `deny` (hard block, logged), `require-approval` (pause branch, batch request), `run-validator` (schema/lint/type/test wrapper, declarative), `run-script` (executable escape hatch: pinned path+hash+sandbox, allowlisted env, timeout, stdout-JSON contract), `notify`. Which need code: sandboxing/signing/policy-eval/audit-chaining/secrets-scan (code); everything else Markdown/policy. Default-deny sets (§22): infra/provision/spend/destructive-DB/protected-merge/prod/security all `require-approval` + `deny` on policy fail; `deny-means-deny` (child cannot override; bypass attempt = audit event + workflow pause).

---

## 16. Artifact Model

Structured/versioned/validated/consumed per §11.1/11.6. States `proposed→researched→verified→approved` (+`rejected/superseded`); transitions require validator + predecessor (e.g., `verified` needs acceptance links + gate outputs). Versioned (int + hash + `supersedes`); `status` + `provenance[]` + `links{}` mandatory; unresolved questions section mandatory until approved. Consumption rule: step may only consume artifacts at required state (enforced by orchestrator, not prose). Traceability `Requirement→Decision→Implementation→Test→Verification Result` via `links` + `trace.json` index generated by `validate` (queryable: `shiploom status --json | jq .trace["REQ-001"]`). Evidence grading table mandatory in research artifacts (Fact/Inferred/Opinion/Marketing/Conflicting/Uncertain + source URL + date + confidence).

---

## 17. Verification Architecture (prevents false completion)

### Principle

**Done is computed, not claimed.** `Done(step) = acceptanceLocked ∧ gatesPass ∧ verifierPass ∧ humanGatesPass ∧ noBypass`. Builder output is `attempt`, never `result`.

### Layers (defense in depth; no single layer sufficient)

| # | Layer | What | Catches | Impl |
|---|---|---|---|---|
| 1 | Locked acceptance | Specifier authors `acceptance/*.json` + oracle impl hidden in `.shiploom/.oracle/` before build; hash-locked; builder sees statements only | Test-redefinition, scope drift | Orchestrator lock + `0700` + gitignore + hash in manifest |
| 2 | Deterministic gates | Build, typecheck, lint, unit/integration, contract, E2E/API/browser (Playwright/pinned), infra-validate (`terraform validate`/Docker build), dep-audit (`osv-scanner`/`npm audit`), secrets-scan, SAST (project tool), license check | Syntax/type/regression/vuln/secret/dep failures | Wrappers in `verification/gates/*.sh` (pinned, timeout, JSON out); fail = no verifier bypass |
| 3 | Independent verifier | Fresh-context LLM (no builder history) re-derives checks from requirements + runs hidden oracle + spot-checks evidence | Shared-blind-spot, hallucinated APIs, plausible-but-wrong | `roles/verifier.md` pack; output schema-validated |
| 4 | Test-quality gates | Compile, determinism (2× run), no network/time dependence (or quarantined), assertion density, mutation floor (e.g., Stryker/infection sample ≥ threshold on changed lines — threshold calibrated per-project, default report-only MVP) | Brittle/circular tests | `verify --report` includes quality table |
| 5 | Differential/invariant | Brownfield: characterization tests pre/post diff; contract tests; API snapshot diff; invariant checks (row counts, idempotency) | Regressions, partial updates | `brownfield/` skills + gate scripts |
| 6 | Runtime verification | Deploy-preview smoke (HTTP/exit codes/screenshots), env validation (pinned versions, OS/shell, conda/venv), seed-data checks | Env blindness, deploy-half-bake | `deployment/smoke-report.md` required before prod gate |
| 7 | Human gates | Scope/arch/prod/security/spend/destructive/protected-merge | Irreversible harm, value misalignment | Policy + `approve` + audit |

### Anti-patterns blocked

- Builder writing tests from its own code without acceptance → blocked (acceptance must pre-exist + locked).
- Verifier editing code to green → forbidden (advisory only; edit = new attempt + re-verify).
- Flaky green → determinism gate retries 2× + quarantine flag.
- Oracle leakage → file perms + adapter excludes oracle from builder context + `doctor` checks gitignore.

### Budgets/breakers (anti-loop/cost)

Per-workflow token/spend/wall-clock caps; 3× identical-tool-payload breaker; spawn-depth cap (default 3); permission-denial = hard stop (no child bypass); 80% budget notify + pause-noncritical; ≤4 implement→verify retries then mandatory replan/human. Cache: SHA-256 exact + MCP TTLs (read 30s/status 5m/search 1h) logged.

---

## 18. Greenfield Architecture

Flow: `init --green` → wizard (`idea.md`) → Specifier research (web MCP quarantined, evidence-graded) → `product/*` → human scope/MVP approval → arch options matrix (stack-neutral; NFR-driven) → ADR + HLD/LLD → human arch approval → acceptance lock + oracle → repo scaffold (template in `examples/<stack>/`, IaC + CI + tests skeleton + observability stubs) → Implementer slices (per-service sequenced) → gates → Verifier → report → human merge/prod gates → deploy plan (env-param) → smoke → post-deploy validation (success metrics vs criteria). Stack choice: Specifier proposes 2–3 options with trade-offs (team skill, NFRs, cost, ops); human picks; core never enforces stack. Cloud-agnostic: arch uses capability blocks (compute/db/auth/queue/CDN) bound to provider only in deployment plan via MCP.

---

## 19. Brownfield Architecture

Flow: `init --existing` → `brownfield-map` (structure/arch/conventions/deps/behavior/debt/risks) → `AGENTS.md` (root+nested) generation → human map approval → `impact-analysis` (callers/types/migrations/blast radius) → `change-plan` (minimal diff + characterization tests + rollback) → human plan approval → pre-change characterization capture → Implementer (scoped context: module+direct callers+types only; muted-repo masks for `archive/v1-alpha`/generated) → attempt log → regression (full affected suite + differential post/pre + contract) → Verifier (fresh) → human review → merge (protected gate). Safety: read-only recon modes by default; write scopes allowlisted per step; contract stubs where DI/reflection lies; differential vs live runtime (seeded) over interface claims; debt register updated (no silent debt paydown/scope creep — Simplicity Gate).

---

## 20. MCP Architecture

Discovery: scan `./.mcp.json` + `./.shiploom/mcp-registry.json` + harness defaults; `doctor` probes transports (stdio/SSE/streamable-HTTP 2025-11-25) + versions + auth presence (never secrets). Capability detection: registry maps `cap.*` → providers + version range + scopes; runtime probe (`tools/list` + smoke call) marks healthy/degraded. Selection: orchestrator resolves by capability + trust + scope-minimality + budget/TTL; explicit `uses:` overrides auto (logged). Trust tiers: `first-party` (repo-local stdio, pinned hash) / `attested` (AttestMCP-style cap attestation + OAuth scopes) / `community` (quarantined, read-only default) / `untrusted` (web-derived servers — deny unless human allows). Permissions: per-capability scopes in registry; per-call policy check; OAuth via harness/OS keychain; secrets never in logs/artifacts (redacted). Auth: env/keychain refs only; `doctor` warns on missing without printing values. Failure handling: timeout+retry(1, jitter)+fallback provider per TTL map; degraded-mode flag in artifact if no provider (never silent). Substitution: fallback chains in registry (e.g., search: Tavily→fetch; browser: Playwright→curl-snapshot; DB: provider-MCP→`psql` read-only). Portability: core references `cap.*` only; server names live in project registry (swap provider = edit registry, not skills).

---

## 21. Harness Interoperability

Target: Claude Code, OpenCode, Pi-class, Copilot, Cursor, Kiro (+ Aider/Factory/Cline via `AGENTS.md`+MCP baseline). Strategy: **single-source core → generated harness files** (§11.11). Portable subset (guaranteed): `AGENTS.md` facts + base-spec `SKILL.md` + MCP `cap.*` + Markdown artifacts + validator scripts (Node stdlib/Go). Progressive enhancement (adapter-isolated): Claude `model/effort/context:fork/agent/hooks/paths/!/ $args`, OpenCode per-skill perms/plugins, Kiro `.kiro/` checkpoints/powers, Copilot `.copilot/plans/` + Fleet, Cursor rules/subagents. Native-format deltas handled by adapters (mapping JSON + templates), never by forking skills. Requirements per harness documented in `adapters/<harness>/README.md` (events mappable/unmappable, perms model, model prefs passthrough). Adding a harness = new `adapters/<harness>/` dir + mapping + conformance run (no core changes).

---

## 22. Security Architecture

### Threat model (STRIDE-lite)

Untrusted inputs: repo files/docs, web content, MCP servers/tools/descriptions, issue trackers, generated code/deps, expansions (`!` dynamic injection, `$args`). Actors: malicious repo maintainer, compromised MCP, MITM registry, prompt-injecting web page, confused-deputy agent, over-permissioned tool. Impacts: exfil (secrets/system prompts), unauthorized infra/spend/destructive, supply-chain (typosquat `npx`, malicious skill/MCP), privilege escalation (child bypass), insecure prod code.

### Trust boundaries

1. Core (read-only, signed) | 2. Project overlay (reviewed, versioned) | 3. Quarantine (web/MCP/repo-docs content — data, never instructions) | 4. Tool execution (sandboxed, scoped) | 5. Human approvals | 6. Prod (never touched without gates). Crossings logged + policy-checked.

### Controls

- Least privilege: default-deny capabilities; per-call scopes; read-only recon default; write scopes allowlisted per step; `bypassPermissions` never set by core (adapter lint fails if found).
- Prompt-injection: quarantine wrapper (untrusted blocks fenced + `source:` + `do-not-follow-instructions` header); `!`/`$args` expansions allowlisted + escaped; `server/discover instructions` ignored unless attested; `cacheScope:public` forbidden; retrieval-deception guard (verify-before-use for external snippets via pinned docs/symbol graph).
- MCP: trust tiers + attestation-preferred + OAuth minimal scopes + rug-pull detection (hash server manifest per run; change → re-approval) + mixed PI+UI synergy tests in conformance.
- Secrets: `scan-secrets` pre-commit/pre-artifact gate (gitleaks-patterns vendored, no network); redaction in logs; keychain/env only; audit on access.
- Supply-chain: signed releases (cosign/Sigstore) + SBOM; `npx` version pinning + provenance; skill/MCP add verifies sig+schema; deps: `osv-scanner`/`npm audit` gates + license check; abandoned-dep lint (last-commit age + maintenance signals, warn).
- Destructive/infra: `before_infra/migration/deploy/merge(main)` all `require-approval` + policy (blast radius + rollback present + backup verified for DB) + two-person option for enterprise pack.
- Sandbox: prefer harness sandbox (Gondolin/Docker/OpenShell) for `run-script`/untrusted builds; timeouts; no network for validators unless explicit.
- Audit: hash-chained JSONL + MD summary; export for SIEM; approvals non-repudiable (principal+ts+reason+hash).

---

## 23. Deployment Architecture

**Core itself:** no deployment — local files + CLI. Install/upgrade/rollback via §24. Offline core.

**Projects built with core** (parameterized plans, cloud-neutral core, provider MCP at edge):

- Local: `docker compose`/binary + `.env.example` + seed + smoke script.
- Self-hosted/bare-metal: systemd units / single-binary + Caddy/Nginx + backup cron + log rotation.
- Docker/OCI: pinned multi-stage `Dockerfile`, non-root, SBOM, `docker build` + Trivy gate.
- Kubernetes: Kustomize base/overlays (no Helm dependency required), probes, HPA, PDB, NetworkPolicy, Sealed-Secrets refs, `kubeconform`+`kube-linter` gates.
- AWS/GCP/Azure: Terraform modules per capability (compute/db/auth/queue/CDN) with `validate/plan` gates, cost estimate artifact, least-privilege IAM, state-backend check; provider MCP only with explicit creds + human spend approval.
- Vercel/Netlify/Cloudflare: framework presets + env bindings + preview-deploy smoke; egress/build-limit notes.
- CI/CD: GitHub Actions + GitLab CI templates (lint→type→test→SAST→audit→build→preview→smoke→gated-prod); branch protection + required checks + codeowner review; never auto-merge to main without human.

Each target gets `deployment/<target>-plan.md` (arch ref, env diff, cost, risks, rollback, smoke) + `smoke-report.md` before prod gate. Framework independence: no target leaks into core; adding target = `examples/deploy/<target>/` template + gate script.

---

## 24. CLI / UX

### Install (zero-friction, hyphenated choice resolved by research)

Primary: `npx @shiploom/ai-builder install` (no global, always-pinned option `--version`; Node-only prereq) for framework files. Companion single binary `shiploom` (Go, ~4MB, signed) via `brew install shiploom/tap/shiploom`, `curl -fsSL shiploom.dev/install.sh | sh`, GH Releases, `choco/scoop`, `mise` — for offline validators/orchestrator/audit without Node. Both install same core (hash-verified); `npx` path shells to binary when present. `npm` not mandatory — documented alternatives first-class (§2.4).

### UX principles

Non-technical: wizard (`init` asks plain-language Qs → drafts `idea.md` → shows cost/risk/scope cards for approvals). Developers: stay in harness; `shiploom` only for gates/status. Teams: PR-anchored trace links + `status --json` for dashboards. All approvals show: what/why/diff/cost/blast-radius/rollback/expires; one-command approve/deny; batched per checkpoint (no per-tool popups for low-risk).

### Command surface (normative list §11.8)

Plus `shiploom approvals [--watch]`, `shiploom trace REQ-001`, `shiploom budget [--set]`, `shiploom resume`. Exit codes §11.7. Upgrades: `upgrade --dry-run` shows file diff + migration notes; `--rollback` restores previous core + manifest compat check. Reproducibility: `shiploom pin` writes lockfile (core+harness+MCP+model ids); `doctor` verifies. Offline: `validate/status/doctor/audit` fully offline; `run` offline except steps declaring `needs: [network|mcp:*]`.

---

## 25. Repository Architecture

`[Recommendation]` **Single versioned repo (monorepo justified here)** — atomic versioning of portable artifacts + adapters + schemas + CLI is worth monorepo cost at small scale; split only if harness binaries diverge (then split `cli/` out, keep contracts repo).

```
shiploom-core/
  core/{artifacts-templates,skills,workflows,hooks,policies,roles}/
  schemas/ + validators (validate.js, go/…)
  adapters/{claude,opencode,pi,copilot,cursor,kiro,base}/
  cli/ (Go, cobra+bubbletea-structure, goreleaser, cosign)
  wrappers/ (npx thin installer)
  examples/{greenfield-*,brownfield-*,stack-*,deploy-*}/
  tests/{unit,integration,conformance,seeds}/
  docs/ (Fumadocs source) + site/ (landing)
  playground/ (docker-limited demo scripts, not required runtime)
  scripts/{render,lint-spelling,check-links}
  .github/workflows/ (validator CI + adapter drift + conformance matrix)
```

Do not impose unused packages (no `queues/DB` in core — zero-runtime). Each top dir has README + version + changelog; core version is single source (`core/VERSION`).

---

## 26. Documentation Architecture

- **Developer docs** (contribute/extend): Fumadocs (Next.js, shared design system, OpenAPI pages for validator JSON contracts, Orama search) `[Recommendation per §2.4]` — lives in `docs/`, deployed statically; versioning DIY via `vX.Y` folders (acceptable at small scale; migrate to Docusaurus if versioned/i18n program outlives app shell).
- **Consumer manual** (use to build software): task-oriented (J1–J4), approvals-first, harness tabs (Claude/OpenCode/Kiro/…), troubleshooting (loops/cost/resume), policy cookbook, brownfield playbook.
- **Landing page (shiploom.dev)**: value prop, capability demo (artifact trace animation), harness logos, `npx @shiploom/ai-builder` quickstart, examples, conformance badges, security posture, pricing (core free/MIT; harness costs passthrough).
- **References**: schema docs (generated from JSON Schema), CLI ref (generated `--help`), adapter matrix, MCP capability catalog, verification-gate catalog, changelog/ADR log.
- **Examples/tutorials**: greenfield SaaS, brownfield fix, deploy-to-preview, policy-pack overlay; each with seed faults + expected verifier output (doubles as conformance fixtures).

---

## 27. Implementation Plan

Dependencies: schemas → validators → skills/workflows → adapters → orchestrator/resume → verification kit → policy/approvals → MCP broker → conformance → docs/site → IDE ext (optional).

**Phase 0 — Foundations (2–3 wks):** schemas + `validate.js` + artifact templates + `AGENTS.md`/base-skill conformance + repo scaffold + validator CI. Exit: `validate` green on examples.

**MVP — Thin reliable slice (4–6 wks):** `init/validate/run/status/approve/doctor` (Go+npx wrapper) + 3 role packs + 8 core skills (idea/market/competitor/product/arch/acceptance/implement/verify) + 2 workflows (greenfield-full-lite, brownfield-fix) + declarative hooks (deny/approve/validator) + locked-acceptance + deterministic gates (build/lint/type/test/secrets/audit) + fresh-verifier + manifest/resume + adapters (claude, opencode, base `AGENTS.md`) + audit log. Exit: AC1–AC6 on 2 stacks × 2 harnesses with seeded faults ≥95% caught.

**Post-MVP (6–10 wks):** full skill catalog, Kiro/Copilot/Cursor adapters, MCP registry+TTL+fallback+attestation option, mutation/differential sampling, SAST/DAST/license gates, deploy-plan matrix (local/Docker/K8s/1-cloud + preview), policy packs (team/enterprise), `upgrade/rollback/pin`, docs site + landing, playground.

**Advanced:** browser/E2E oracle harness, invariant mining, cost optimizer, fleet sequencing (multi-service parallel), thin IDE approver ext, marketplace signing infra, CRA/SBOM automation, i18n docs, migration-evolution bots (spec/harness drift).

What MVP deliberately excludes: IDE build, custom model routing, cloud sandbox, fleet runner, auto-provisioning, multi-agent debate, vector memory, server/DB/queue in core.

---

## 28. Testing Strategy (framework itself)

- Unit: schemas/validators/policy-eval/manifest transitions/adapters rendering (table-driven, hermetic, no network).
- Integration: `run` on fixture repos (green scaffold + brownfield legacy sample with known debt) across harness stubs (recorded harness transcripts, not live LLMs) + real-toolchain gates (build/test/lint in containers for fixtures only — fixtures need Docker, core does not).
- Negative: bypass attempts (child-spawn on deny, oracle read by builder, test-redefinition, unapproved prod) must block + audit.
- Fault-injection: seeded faults (broken build, SQLi, secret, regression, flaky test, hallucinated API, missing acceptance) → verifier catch-rate measured.
- Determinism: golden `validate/status/audit` outputs; manifest replay tests (kill -9 → resume).
- Compatibility: adapter drift tests (generated-file hashes), harness version matrix (pinned).
- Performance: validator <5s on 500-file fixture; installer <30s; budget-breaker tests (3× payload, depth cap).
- Human-factors: approval comprehension (can Nadia explain cost/risk from card?), override audit tests.

---

## 29. Conformance Testing (harness compatibility)

Suite in `tests/conformance/`: per-harness profile (`capabilities.json`: events supported, perms model, skill fields honored/ignored, MCP transports, context limits) + shared fixtures (idea + repo + acceptance + seeds) + expectations (must-pass gates, must-block bypasses, must-produce artifacts). Runner: `shiploom conformance --harness <name> [--record]` executes workflow via harness adapter (live LLM optional; default uses recorded transcripts + real validators for determinism; nightly live-LLM lane measures catch-rate drift). Badges per harness/version; `doctor` fails on untested harness version with clear message (never silent degrade). New harness onboarding = profile + mapping + full suite green.

---

## 30. Migration / Evolution Strategy

- Models change: roles reference capabilities, never model ids; model prefs passthrough in project config; nightly live lane recalibrates budgets/thresholds; prompts versioned separately from core logic.
- Harnesses change: adapters isolate deltas; `doctor` drift check; unmappable events → orchestrator pre-check fallback (documented); conformance matrix pins supported versions; deprecation policy (N-1 support, 30-day notes).
- MCP evolves: capability abstraction + version ranges + transport negotiation (stdio→streamable-HTTP); 2025-11-25 baseline, forward-compatible via registry; attestation optional-upgrade path.
- Tools/deploy tech change: gate scripts + deploy templates versioned per-target; new target = template + gates, no core change.
- Core evolves: semver + lockfile + `upgrade --dry-run` diff + codemod notes + rollback; overlay `extends:` prevents fork rot; ADR log for breaking changes.

---

## 31. Risks

- Technical: verifier still LLM (misses novel bugs → mitigated by deterministic layers + sampling + human gates, never eliminated); oracle design burden on Specifier (mitigated by acceptance skill + human review); brownfield scale limits (mitigated by scoping + muted-repo, not solved for mega-monorepos).
- Product: ceremony fatigue for small fixes (mitigated by right-sized `quick-fix` workflow); non-technical over-trust (mitigated by plain-language risk cards + mandatory gates).
- Security: MCP rug-pull/zero-day, skill injection via marketplace (mitigated by signing + tiers + quarantine, residual risk disclosed).
- Ecosystem: spec drift (`AGENTS.md`/Skills/MCP), harness reprice/deprecate, Windsurf/Cognition agency terms (mitigated by adapters + BYO-model + N-1 policy).
- Adoption: “another framework” fatigue vs BMAD/SuperClaude (mitigated by compat — core consumes BMAD artifacts, migrates SuperClaude commands → base skills; no rip-and-replace).
- Cost: verification adds tokens (mutation/E2E) — bounded by budgets + sampling + cache; ROI tracked via defect-escape vs token-spend dashboard (post-MVP).

---

## 32. Scrutiny: Second-Order Critique + Revisions

> Red-team applied to the proposal above; revisions already folded into §§11–17, summarized here for auditability.

1. **Assumption: artifacts solve context loss.** Attack: artifacts bloat, go stale, diverge from code. Revision: mandatory `version/supersedes`, hash in manifest, `validate` link-check, assumption expiry, code-is-truth rule (divergence fails verifier; arch must re-approve on drift).
2. **Agents exploit ambiguity.** Attack: vague acceptance (“fast”, “secure”) passes anything. Revision: acceptance schema requires measurable `howToVerify` + oracle; vague criteria fail `validate --strict`; Simplicity/Ambiguity Gate blocks build.
3. **Agents disagree.** Attack: Specifier vs Verifier deadlock. Revision: orchestrator routes to human with both positions + evidence; no agent overrides another; retries capped then replan.
4. **Verification wrong / tests wrong / requirements wrong.** Attack: all layers agree on wrong thing. Revision: independence (different context + hidden oracle) + determinism (exit codes/HTTP) + human scope/arch gates + post-deploy metric check (success criteria vs telemetry) that can reopen requirements (supersede, not edit).
5. **Web research wrong.** Attack: marketing as fact, stale pricing. Revision: evidence grading + confidence + date + ≥2 sources for load-bearing claims; low-confidence blocks auto-scope (requires human confirm).
6. **MCP unavailable/malicious.** Attack: outage or poisoned tool stalls/steers workflow. Revision: fallback chains + degraded-mode flags + quarantine + attestation tiers + rug-pull re-approval; no silent substitution.
7. **No tests in brownfield.** Attack: zero regression signal. Revision: characterization capture becomes mandatory pre-step (records behavior snapshots); new acceptance for touched paths; differential on snapshots; explicit residual-risk note if coverage thin (human decides).
8. **Poorly understood codebase.** Attack: map is fiction. Revision: map requires symbol-graph + build+test evidence (not just file list); low-confidence areas marked + write-scopes narrowed; muted-repo prevents debt extrapolation.
9. **Arch/impl diverge.** Attack: expedient drift. Revision: drift check in gates (import-graph/contract diff vs arch); divergence = verify-fail → replan (arch update + re-approval) or revert.
10. **Agent stuck / partial / interrupted.** Attack: loops, half-state, kill -9. Revision: budgets/breakers/depth caps + idempotent steps + atomic checkpoints + `resume/only/from` + attempt logs (no redo of verified).
11. **Humans override wrongly / can't understand decisions.** Attack: rubber-stamp prod or blind deny. Revision: approvals show rationale+rejected-alternatives+evidence+cost/risk/rollback; overrides logged + require reason; audit export.
12. **Reproducibility.** Attack: “works on my model”. Revision: lockfile (core/harness/MCP/models) + seeded logs + offline validators + recorded-transcript conformance.
13. **More agents ≠ better.** Attack: pressure to add reviewer-of-reviewers. Revision: hard cap (3 persistent roles MVP); new role requires failure-evidence + sunset review; ephemeral instances preferred.
14. **Complexity creep (recreate IDE/CI).** Attack: orchestrator becomes CI, approver becomes IDE. Revision: orchestrator only sequences local gates (delegates to project CI via thin wrappers); approver is CLI/MD first, IDE ext viewer-only; CI/CD ownership stays with project.
15. **Provider/MCP/harness dependence.** Attack: core couples to Tavily/AWS/MCP version. Revision: `cap.*` abstraction + project-scoped providers + transport negotiation + adapter isolation; core CI tests with stubbed MCP.
16. **Does verification cost too much?** Attack: mutation/E2E exceed value on small fixes. Revision: right-sized workflows (quick-fix skips mutation; full-method requires it) + sampling + budgets; cost/escape dashboard justifies thresholds empirically.

Residual uncertainties moved to §33 rather than hidden.

---

## 33. Open Questions — Decisions (2026-09-17 review)

1. Oracle authoring burden → **DECIDED: Guided wizard + auto-oracle.** Specifier drafts plain-language cards + examples; Nadia approves narrative; technical `acceptance/*.json` + hidden oracle auto-derived and locked. Exit metric: usability trial (can Nadia explain cost/risk/scope from card?).
2. Mutation threshold calibration → **DECIDED: Report-only + sampling (MVP).** Mutation on sample of changed lines, reported not blocking. Enforceable per-stack thresholds set only after pilot escape data.
3. Formal policy language choice → **DECIDED: JSON-policy MVP.** Stdlib-evaluated JSON rules; sufficient for default-deny gates; zero-runtime preserved. Adopt Cedar/Rego only if enterprise pilot hits expressiveness ceiling.
4. Marketplace trust at scale → **DECIDED: Git + cosign MVP.** Git URL + semver tag + cosign sig + schema check; no server/registry. Revisit threshold-sigs/transparency-log/registry only on ecosystem demand.
5. Long-horizon concurrency → **DECIDED: File-scoped parallel.** Parallel per-service/per-file slices, orchestrator-sequenced merges. Semantic/refactor-aware merge deferred until fleet pilot data requires it.
6. Privacy → **DECIDED: Local redaction MVP.** Env/keychain refs, log redaction, scan-secrets gate. Structured secret-proxy only if Sam/enterprise DLP pilot proves gap.
7. Agent Skills spec evolution → **DECIDED: Base-spec-first, adapter-driven, N-1 compatibility.** Base-spec portable core + `x-shiploom-harness` namespaced extras; adapters isolate drift; support N-1 harness versions; track `agentskills.io` + Claude/OpenCode changelogs.
8. Benchmark for this system itself → **DECIDED: Custom held-out suite.** Held-out brownfield tasks + maintainer-merge proxy + prod-escape tracking with pilot partners. Do not use SWE-bench Verified alone (contamination + 32% verifier error).

---

## A. Evidence Standards & Key Sources

- Primary (used where available): `agents.md/`, `agentskills.io/specification`, `modelcontextprotocol.io/specification/2025-11-25/`, `code.claude.com/docs/*` (hooks/skills/agents/glossary), `opencode.ai/docs/*`, `pi.dev/docs`, `docs.github.com/en/copilot/*`, `cursor.com/docs/*`, `kiro.dev/docs`, `docs.factory.ai/`, `docs.cline.bot/`, `docs.roocode.com/`, `aider.chat/docs/`, `docs.openhands.dev/`, `bmad-method` repo/docs, SuperClaude repo/docs.
- Benchmarks/papers: SWE-bench Pro + DeepSWE audit + SWE-Bench Illusion analyses; VALTEST (`doi:10.48550/arxiv.2411.08254`); TCG/SAGA (NeurIPS'25); wild 20k-session study (`arxiv:2605.29442`); context-rot/bankruptcy studies; Veracode GenAI Code Security Report 2025 (+Oct update); Stanford Perry et al. CCS'23; Snyk GenAI reports; CodeRabbit 470-repo analysis; MSB/MCPTox/AttestMCP/MCP-GUARD papers; HouYi/HackAPrompt/OwASP LLM Top 10.
- Community/field: Cursor forum loop reports; `anthropics/claude-code` issues (subagent depth, permission bypass, token burn); Answer.AI/Qubika Devin evaluations; Replit prod incident reports; Faros AI PR study; controlled Cursor slowdown study; Tianpan legacy/brownfield series.
- Vendor claims flagged as such (pricing/limits/roadmap, e.g., Lovable/Bolt/v0/Replit tiers, Windsurf→Devin migration, Roo→Zoo fork).
- Rapid-change notice: harness docs/specs versioned above; re-verify via `doctor` + conformance matrix at implementation time; conflicts/weak evidence noted inline with confidence.

*End of Master Specification v1.0-draft. Next action: implement Phase 0 (schemas + validators + templates) then MVP slice per §27; track Open Questions §33 as pilot exit criteria.*
