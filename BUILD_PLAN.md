# Shiploom Core — Build Plan

**Source:** `MASTER_SPEC.md v1.0-draft` (2026-09-17)
**Status:** Approved for implementation
**Date documented:** 2026-09-17

## 1. What we're building

**`Shiploom Core` (`@shiploom/ai-builder`)** — a zero-runtime, Markdown-first portable layer, not an IDE / model / server. Per MASTER_SPEC §6 / §9:

- **Core triad:** `Specifier → Implementer → Verifier` (fresh context + hidden oracle) + deterministic Orchestrator (code, not LLM) + Human Approver.
- **Done = system state:** `acceptanceLocked ∧ gatesPass ∧ verifierPass ∧ humanGatesPass ∧ noBypass` — never an agent claim.
- **Invariants:** artifacts over chat; local-first; cloud-neutral core; `cap.*` MCP abstraction;/human gates on infra/spend/destructive/prod/security.

**Minimum viable promise (§6.3):** given `idea.md` (or existing repo), produce traceable `Requirement → Decision → Implementation → Test → Verification Result` with human gates, on any supported harness, without external infra for the system itself.

## 2. Locked decisions

Decided 2026-09-17 (supersede spec defaults where noted):

| Decision | Choice | Spec reference / note |
|---|---|---|
| CLI stack | **Python + uv for MVP** | Overrides §2.4 Go-default. Keep §11.8 CLI surface + §11.1–11.2 schemas stable so a Go single-binary can replace Python later without contract break. Validators stdlib-only. **[Superseded 2026-09-17: Go migration P1–P6 complete (PR23–PR37); Go single binary is now the default distribution, Python is the fallback pending removal in v1.3.0. See `GO_MIGRATION_PLAN.md`.]** |
| First harness targets | **Claude Code + OpenCode only** | Spec MVP scope (§27). Cursor/Kiro/Copilot deferred to post-MVP. |
| MVP slice order | **Greenfield-first** | `greenfield-full-lite` fully working; `brownfield-fix` skeleton for AC2 smoke, hardened post-MVP. |
| Docs site timing | **Defer site** | Keep `docs/` as plain Markdown in MVP. Fumadocs site post-MVP per §26. |

Still-decided per §33 (carry as pilot exit criteria, do not revisit without data):

1. Oracle authoring: guided wizard + auto-oracle; exit = Nadia comprehension trial.
2. Mutation: report-only + sampling in MVP; blocking thresholds only after pilot escape data.
3. Policy language: JSON-policy MVP (stdlib-evaluated); Cedar/Rego only if enterprise pilot hits ceiling.
4. Marketplace trust: git URL + semver tag + cosign sig + schema check; no server/registry.
5. Concurrency: file-scoped parallel, orchestrator-sequenced merges.
6. Privacy: local redaction MVP (env/keychain refs, log redaction, scan-secrets gate).
7. Skills spec: base-spec-first + `x-shiploom-harness` extras, N-1 harness support.
8. Benchmark: custom held-out suite + merge proxy + escape tracking; never SWE-bench Verified alone.

## 3. Repository architecture (§25, adapted for Python)

Single versioned monorepo. Atomic versioning of artifacts + adapters + schemas + CLI outweighs monorepo cost at this scale. Split `cli/` out only if harness binaries diverge.

```
shiploom-core/
  core/{artifacts-templates/,skills/,workflows/,hooks/,policies/,roles/}
  schemas/                          # 9 JSON Schemas, draft 2020-12
  validators/                       # Python stdlib only
  cli/                              # Python+uv, argparse stdlib (no Typer/click in MVP)
  wrappers/                         # pipx/uvx entry (+ optional thin npx installer later)
  adapters/{base,claude,opencode}/
  examples/{greenfield-starter,brownfield-sample,stack-python,stack-node}/
  tests/{unit,integration,conformance,seeds}/
  docs/                             # plain Markdown only for MVP
  scripts/{render,lint-spelling,check-links}
  .github/workflows/               # validator CI + adapter drift
  core/VERSION  pyproject.toml  AGENTS.md
```

Rules:

- Single source of truth: `core/VERSION`.
- No `queues/`, DB, server, daemon in core (zero-runtime).
- Each top dir: README + version + changelog.
- Install: `pipx install shiploom` / `uvx shiploom` primary. `brew/curl|sh/GH Releases` + Go binary deferred with Go migration.

## 4. Phase 0 — Foundations (2–3 wks)

Goal: contracts exist and are machine-enforceable.
Exit: `validate` green on examples.

1. **Schemas first (blocks everything, §11.2):** `artifact-frontmatter, acceptance, skill, workflow, hook, policy, mcp-registry, verification-report, trace-link`.
2. **Validators (Python stdlib only):** `validate.py` → stdout `{ok, errors[], warnings[]}`, exit codes `0/2/3/4/5` per §11.7. Frontmatter check, link check + `trace.json` generation, `--strict` vague-acceptance rejection ("fast"/"secure" without measurable `howToVerify` fails).
3. **Artifact templates (§11.6 + §11.1 frontmatter):** `idea.md`, `research/*` (evidence-grading table: Fact/Inferred/Opinion/Marketing/Conflicting/Uncertain mandatory), `product/*`, `architecture/*`, `acceptance/*.json`, `brownfield/*`.
4. **Base portability fixture:** root + nested `AGENTS.md`, one base-spec `SKILL.md` (name == dir, description 1–1024 chars, body <500 lines) passing on Claude + OpenCode.
5. **Repo + CI:** `pyproject.toml` (uv), hermetic table-driven unit tests, adapter hash/drift skeleton, link/spelling checks.

## 5. MVP — Thin reliable slice (4–6 wks)

Exit: AC1–AC6 on 2 stacks (python + node) × 2 harnesses (Claude, OpenCode), seeded faults ≥95% caught.

### 5.1 CLI

Normative surface §11.8 (keep names/flags stable for future Go port):

```
shiploom install [--global|--local] [--version x.y.z]
shiploom init [--green|--existing] [--harness claude|opencode|auto] [--stack ...]
shiploom add <skill|workflow|hook|adapter|policy> <name>
shiploom validate [--strict] [path]
shiploom run <workflow> [--from STEP] [--only STEP] [--resume] [--budget ...]
shiploom status [--json]
shiploom approve <gate-id> [--deny --reason ...]
shiploom verify [--report]
shiploom doctor
shiploom upgrade [--dry-run] [--rollback]
shiploom adapters --list | --generate <harness>
shiploom audit [--export json|md]
# + trace REQ-001, budget, resume, approvals --watch
```

State files: global `~/.shiploom/config.json`, project `./.shiploom/config.json`, `./.shiploom/manifest.json` (atomic writes, hash-chained, kill -9 safe), `./.shiploom/audit.jsonl` (append-only, hash-chained). `pin` lockfile: core+harness+MCP+model ids. Build `install/init/validate/run/status/approve/doctor` first; `verify/adapters/audit/trace/budget` after. All offline except steps declaring `needs: [network|mcp:*]`.

### 5.2 Roles (3 persistent only, §12)

- `roles/specifier.md` (researcher/product/architect modes): evidence + confidence required; no implementation diffs; assumptions expire.
- `roles/implementer.md`: consumes approved arch + acceptance + task slice only; sees statements, never oracle impl; ≤4 retries.
- `roles/verifier.md`: fresh context, no builder history; hidden oracle access; advisory diffs only (edit = new attempt + re-verify).
- Orchestrator = code: order, gates, budgets, breakers, resume, audit. No LLM judgment.

### 5.3 Skills (8 for greenfield path, §13)

Build: `idea-shaping, market-research, competitor-teardown, product-definition, architecture-design, spec-to-acceptance, implement-scoped-diff, verify-independent`.
Stub: `brownfield-map, deployment-plan`.
Rules: base-spec frontmatter (`name/description` + optional `license/compatibility/metadata`); body sections Purpose/Inputs/Outputs/Prerequisites/Methodology/Constraints/Tools(capability names, not server names)/Verification/Examples; `scripts/` pinned interpreter; harness extras only under `x-shiploom-harness:` block.

### 5.4 Workflows (§14)

- `greenfield-full-lite` (full): idea→research→requirements→arch→implement→verify. Sequential + `on:` fan-out + `when:` conditionals + `gate:` approvals + `retries:4→replan`. Idempotent + checkpointed.
- `brownfield-fix` (skeleton for MVP): map→impact→plan→implement→regression-verify.
- Event routes: `on_verify_fail` (retry→replan→human), `on_security_fail` (quarantine→human, no retry), `on_budget_80` (notify + pause-noncritical).

### 5.5 Hooks (§15)

Declarative `deny / require-approval / run-validator / notify`; `run-script` as audited, pinned, sandboxed escape hatch. Default-deny: infra/provision/spend/destructive-DB/protected-merge/prod/security. `deny-means-deny` propagated to children; bypass = audit event + pause. Map to Claude 28 events / OpenCode plugin events; unmappable → orchestrator pre-check + log, never silent.

### 5.6 Verification kit (§17 — the core bet)

`Done(step) = acceptanceLocked ∧ gatesPass ∧ verifierPass ∧ humanGatesPass ∧ noBypass`.

| # | Layer | MVP scope |
|---|---|---|
| 1 | Locked acceptance | Specifier authors `acceptance/*.json` + oracle in `./.shiploom/.oracle/` (0700, gitignored, excluded from builder ctx) before build; hash-locked in manifest |
| 2 | Deterministic gates | `build, typecheck, lint, unit/integration, contract, dep-audit (osv-scanner/npm audit), secrets-scan` wrappers in `verification/gates/*.sh`, pinned + timeout + JSON out |
| 3 | Independent verifier | Fresh-context pack, schema-validated output |
| 4 | Test-quality | Compile, 2× determinism, no-net/time dependence, assertion density; mutation sampling report-only |
| 5 | Differential | Characterization pre/post for brownfield skeleton |
| 6 | Runtime smoke | Preview HTTP/exit-code checks + `smoke-report.md` required before prod gate |
| 7 | Human gates | Scope/arch/prod/security/spend/destructive/protected-merge via `approve` + audit |

Budgets/breakers: per-workflow token/spend/wall-clock caps; 3× identical-payload breaker; spawn-depth cap 3; denial = hard stop; 80% notify.

### 5.7 Adapters (§21, §11.11)

Single-source `core/*` → generated files. MVP dirs: `base` (`AGENTS.md`), `claude` (`CLAUDE.md`, `.claude/skills/`), `opencode` (`.opencode/skills/`). As-built PR10: `settings.json` hook mapping + `opencode.json` deferred post-MVP with documented reasons (adapter READMEs); `doctor` hash/drift check likewise deferred to the conformance runner. `adapters --generate` idempotent, `DO NOT EDIT` header. Each adapter README documents mappable/unmappable events + perms model.

### 5.8 MCP (§20, §11.12)

`./.mcp.json` (harness-native) + `./.shiploom/mcp-registry.json`: `cap.* → providers + version range + scopes + trust + ttlS + fallback`. Resolution: capability → attested → scope-minimal → budget/TTL → call → quarantine-if-untrusted → log. Trust tiers: first-party / attested / community (quarantined, read-only) / untrusted (deny unless human allows). Secrets via env/keychain refs only, redacted in logs. Fallbacks e.g. search Tavily→fetch, browser Playwright→curl-snapshot. No silent substitution — degraded-mode flag in artifact.

### 5.9 Policy / approvals / audit (§22)

JSON-policy MVP (stdlib-evaluated). Batched plain-language cards (what/why/diff/cost/blast-radius/rollback/expires), one-command approve/deny, `approvals --watch`. Least-privilege default-deny; `bypassPermissions` never set by core (adapter lint fails if found). Quarantine wrapper for web/MCP/repo-docs content. `scan-secrets` pre-commit/pre-artifact gate. Signed releases (cosign/Sigstore) + SBOM when publishing.

MVP deliberately excludes: IDE build/extension, model routing, cloud sandbox, fleet runner, auto-provisioning, multi-agent debate, vector memory, server/DB/queue in core, DAST/full SAST, Kiro/Copilot/Cursor adapters, marketplace server, docs site.

## 6. Post-MVP (6–10 wks) / Advanced

- Post-MVP: full skill catalog, Kiro/Copilot/Cursor adapters, MCP attestation + rug-pull re-approval, mutation/differential sampling hardening, SAST/DAST/license gates, deploy matrix (local/Docker/K8s/1-cloud + preview), team/enterprise policy packs, `upgrade/rollback/pin`, Fumadocs site + landing, playground, cost/escape dashboard.
- Advanced: browser/E2E oracle harness, invariant mining, cost optimizer, fleet sequencing, thin IDE approver (viewer-only), marketplace signing infra, CRA/SBOM automation, i18n, drift bots.

## 7. Testing strategy (§28) + conformance (§29)

- Unit: schemas/validators/policy/manifest/adapters (table-driven, hermetic, no network).
- Integration: `run` on fixture repos (green scaffold + brownfield legacy with known debt) via recorded harness transcripts (default) + real toolchain gates in containers.
- Negative: child-spawn-on-deny, oracle-read-by-builder, test-redefinition, unapproved prod must block + audit.
- Fault-injection seeds: broken build, SQLi, secret, regression, flaky test, hallucinated API, missing acceptance → verifier catch-rate metric.
- Determinism: golden `validate/status/audit` outputs; kill -9 → resume replay.
- Compat: adapter drift hashes; pinned harness version matrix.
- Perf: validator <5s on 500-file fixture; installer <30s; breaker/depth-cap tests.
- Human-factors: Nadia comprehension of cost/risk card; override audit.
- Conformance suite `tests/conformance/<harness>/`: `capabilities.json` + shared fixtures + expectations. `shiploom conformance --harness [--record]`. Badges per harness/version. Nightly live-LLM lane measures drift only.

## 8. Acceptance mapping (product-level §8.5)

- AC1 idea→verified MVP without YAML edit → `greenfield-full-lite` + wizard + approvals cards.
- AC2 brownfield change + regression report, zero protected-merge w/o approval → `brownfield-fix` skeleton + gates.
- AC3 broken-build claim fails Done ≥95% seeded → §5.6 layers 1–3.
- AC4 kill -9 resume without redo → atomic manifest + checkpoints + `run --resume/--only/--from`.
- AC5 offline `validate/status/doctor` → stdlib validators, no network in core path.
- AC6 no prod/spend/destructive without logged approval (negative tests) → default-deny + audit.

## 9. Execution order (first 10 PRs)

1. PR1: scaffold + `core/VERSION` + 9 schemas + `pyproject.toml` (uv).
2. PR2: `validators/validate.py` + unit tests + validator CI.
3. PR3: artifact templates + `trace.json` + `status --json`.
4. PR4: CLI `init/validate/status/doctor` + manifest + hash-chained audit.
5. PR5: 3 role packs + `idea-shaping/product-definition/architecture-design`.
6. PR6: `spec-to-acceptance` + oracle vault + lock + `--strict` gate.
7. PR7: `run` orchestrator + `greenfield-full-lite` + resume/budgets/breakers.
8. PR8: deterministic gates + `verify` + verifier pack + quality table.
9. PR9: base+claude+opencode adapters + `adapters --generate` + conformance fixtures.
10. PR10: hooks/policy/approvals + MCP registry + negative tests + AC1–AC6 demo (2 stacks × 2 harnesses).

Dependencies: schemas → validators → skills/workflows → adapters → orchestrator/resume → verification kit → policy/approvals → MCP broker → conformance → docs/site.

## 10. Risks (summary, detail §31)

- Verifier still LLM → mitigated by deterministic layers + sampling + human gates, never eliminated.
- Oracle design burden → wizard + auto-oracle + human review.
- Brownfield scale → scoping + muted-repo; mega-monorepos not solved in MVP.
- Ceremony fatigue → right-sized `quick-fix` variant.
- Non-technical over-trust → plain-language risk cards + mandatory gates.
- MCP rug-pull / skill injection → tiers + quarantine + signing; residual risk disclosed.
- Spec/harness drift → adapters + `doctor` + N-1 policy + BYO-model.
- "Another framework" fatigue → BMAD-artifact compat, SuperClaude command migration, no rip-and-replace.
- Verification token cost → budgets + sampling + cache; post-MVP cost/escape dashboard.

## 11. Second-order scrutiny (folded in, §32)

Artifacts bloat → version/supersedes + manifest hash + link-check + assumption expiry + code-is-truth (drift fails verifier). Vague acceptance → measurable `howToVerify` + oracle required. Agent deadlock → orchestrator routes both positions + evidence to human. Wrong requirements/tests → independence + determinism + human scope/arch gates + post-deploy reopen via supersede. Stale web research → evidence grading + ≥2 sources for load-bearing claims. MCP outage/malice → fallback + degraded flag + quarantine. No brownfield tests → mandatory characterization capture + residual-risk note. Fictional map → symbol-graph + build/test evidence required. Arch/impl drift → import-graph/contract diff gate. Loops/partial → budgets/breakers + idempotent steps + atomic checkpoints. Rubber-stamp → rationale + rejected-alternatives + cost/risk/rollback on every card; overrides need reason. "Works on my model" → lockfile + seeded logs + recorded-transcript conformance. Role creep → hard cap 3 persistent roles. Complexity creep (rebuild IDE/CI) → orchestrator sequences local gates only; approver CLI/MD first. Provider coupling → `cap.*` abstraction.

*Next action: implement Phase 0 PR1 (scaffold + schemas). Track §33 items as pilot exit criteria.*
