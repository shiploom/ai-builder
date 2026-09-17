# Go Migration Plan (revisited parked decision, 2026-09-17)

**Decision:** migrate the CLI + validators from Python+uv to Go,
stdlib-only. Rationale: spec already nominates Go as the default public-CLI
language (§2.4: 3–6MB, <10ms, `GOOS=` cross-compile) and designs distribution
(brew tap, curl|sh, GH releases, npx wrapper) around a binary. The Python
surface (~4,556 lines, 17 modules, test-enforced stdlib-only) is ideal port
substrate, and JSON contracts + exit codes + 251 hermetic tests give a
ready-made parity harness.

**Framework decision:** stdlib-only (`flag` + hand-rolled subcommands) —
preserves the zero-dependency supply-chain posture. No cobra/viper.

## Strategy: strangler with golden parity (not big-bang)

- P1 (shipped): toolchain bootstrap + `go.mod` (`github.com/shiploom/ai-builder`)
  + `cmd/shiploom/` + `internal/` layout + parity-harness design + `--version`
  parity proof.
- P2: leaf libs in dependency order (manifest → mcp → auditlog → policy →
  approvals → add → adapters → conformance → doctor → oracle → status →
  trace → characterize), each with ported unit tests + golden parity.
- P3: orchestrator + gates (`run`, `gates`, `verify_step_check`) — highest
  risk; replay recorded `run` transcripts, not just unit tests.
- P4: full CLI surface — flags, exit codes 0/2/3/4/5, human-readable output
  byte-identical (`ac-demo.sh` parses them).
- P5: distribution — extend `release.yml` (go build matrix + existing SBOM
  pattern), brew tap activation, formula from template, npx wrapper.
- P6: transition — dual-ship with version-parity check, Python fallback
  deprecated one minor after Go parity, then removed.

## Hard parts (release blockers)

- YAML-subset frontmatter parser: replicate quirks exactly (tab rejection,
  unclosed-flow-list errors, one-level nesting). Fuzz both parsers on the
  template + seed corpus; any divergence blocks release.
- JSON Schema subset evaluator: golden-file every schema + valid/invalid fixture.
- Error message strings: downstream scripts grep them — keep identical, test them.
- Windows semantics (0700 dirs, exec bits, paths) need Windows CI legs
  (current matrix is ubuntu-only).
- Parity harness: Python CLI over fixtures/seeds → capture stdout JSON +
  exit codes (timestamps/hashes normalized) → Go must reproduce byte-identical.

## Success criteria

Parity suite green on darwin/linux/windows; binary <10MB; cold start <50ms;
full AC demo passing under the Go binary; `docs/install.md` rewritten
("No Go single binary is planned" is false post-migration).
