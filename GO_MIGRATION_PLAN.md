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
- P2 (shipped): leaf libs in dependency order (manifest → mcp → auditlog →
  policy → oracle → characterize → difflib → gates-verify + `validate`
  wiring), jsoncanon/pyRepr/fnmatch foundations, 7/7 parity.
- P3 (shipped): orchestrator + read surfaces — `internal/workflow`
  (find/load/resolve_uses/glob), `internal/run` stepper, `internal/trace`,
  `internal/status`, `internal/approvals` + `run`/`status`/`approvals` CLI
  wiring. Parity 32/32 (CPython 3.9 + 3.13 legs).
- P4 (shipped): full CLI surface — flags, exit codes 0/2/3/4/5, human-readable output
  byte-identical (`ac-demo.sh` parses them). Shipped slices: `lock`
  (`--check`/`--actor`/`--json`, vault 0700) + `verify`
  (`--report`/`--gates`/`--json`, deterministic quality table) +
  `trace` (`--json`, sorted relations/incoming) + `approve`
  (`--deny`/`--reason`/`--actor`/`--json`) + `budget`
  (`--set`/`--actor`/`--json`/`[path]`) + `resume`
  (`--budget`/`--actor`/`--json`, position + stepper) + `audit`
  (`--export json|md`) + `characterize`
  (`--capture`/`--diff`/`--list`, `--command`/`--timeout`/`--actor`/`--json`) +
  `doctor` (`--json`, offline toolchain/harness/project checks, exit 5 on
  failure) + `pin` (`--check`/`--json`, reproducibility pin) + `upgrade`
  (`--dry-run`/`--rollback`/`--actor`/`--json`, backup + validation
  gate) + `adapters` (`--list`/`--generate`/`--json`, idempotent harness
  files) + `add` (`--from`/`--tag`/`--force`/`--actor`/`--json`, overlay
  packs with strict validation + rollback) + `conformance`
  (`--harness`/`--record`/`--json`, scratch-generate + profile checks) +
  `install` (`--global`/`--local`/`--version`, offline core copy) + `init`
  (`--green`/`--existing`/`--harness`/`--stack`/`--force`, project
  scaffold). All 20 commands wired. Parity 77/77.
- P5: distribution — extend `release.yml` (go build matrix + existing SBOM
  pattern), brew tap activation, formula from template, npx wrapper.
  Shipped first: AC demo under Go (`SHIPLOOM_GO_BIN` mode in
  `scripts/ac-demo.sh`, 54/54). Measured: 5.6MB binary (<10MB),
  ~5ms cold start (<50ms). Shipped second: `go-verify` + 5-platform
  `go-build` matrix jobs in `release.yml` (checksums + Go module
  manifest attached), Go source-build brew formula, `curl|sh`
  installer (`scripts/install.sh`), thin npx launcher
  (`wrappers/npx`, `@shiploom/cli`), and the `docs/install.md`
  rewrite (Go binary primary, Python fallback).
- P6 (in progress): transition — dual-ship with version-parity check,
  Python fallback deprecated one minor after Go parity, then removed.
  - Slice 1 (this): `scripts/version-check.sh` asserts tag ==
    `core/VERSION` == `pyproject.toml` == `wrappers/npx/package.json` ==
    the `core X` field of both `--version` outputs; wired into
    `release.yml`; static part unit-tested.
  - Slice 2 (at 1.2.0): Python deprecation notice, version-gated
    (`core/VERSION >= 1.2.0`) AND TTY-gated (`sys.stderr.isatty()`).
    Both gates are load-bearing: an always-on Python-only line would
    break all 77 parity fixtures, and stdout must stay machine-
    parseable for `ac-demo.sh` greps — so the notice goes to stderr.
  - Slice 3: docs mark the Python fallback deprecated (removal v1.3.0);
    removal itself is out of scope for P6 (parity harness + Python CLI
    deleted alongside, a minor later). N-1 support per MASTER_SPEC
    deprecation policy.
  - Out of scope: `doctor` changes, npm publish automation, Windows CI.

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

## Port notes (P2) — fidelity contract and known divergences

- Canonical JSON: Python `json.dump(sort_keys, indent=2, ensure_ascii)` and
  compact-separator hash inputs reproduced byte-exact (`internal/jsoncanon`),
  proven by embedded CPython goldens. Audit chains are mutually verifiable
  across implementations in both directions (interop test).
- Numbers keep literals (`json.Number`): `800000` stays integral while
  `"25.0"` stays float, so integer checks, `%r` output, and manifest bytes
  match. Manifest floats like `25.0` round-trip byte-identical.
- `pyRepr`/`pyEqual` mirror CPython `repr()`/`==` for JSON scalars
  (single-quote preference, `\x7f` escaping, `True == 1`).
- fnmatch ported from the CPython algorithm (including `*` crossing `/`,
  unlike Go's path.Match); truth table generated from CPython itself.
- Lengths count runes (Python `len(str)`); `\n`-only splitlines parity.
- Deterministic where Python is: sorted patternProperties, sorted link
  pass, sorted error output.
- Deliberate divergences (all fail-closed, never silent wrong): engine
  suffixes on regexp/JSON error text; exotic numerics (hex floats,
  misplaced underscores); non-UTF8 `.md` (Python tracebacks, Go reports
  clean); unhashable workflow ids/links (Python crashes, Go skips);
  malformed packs (Python may crash, Go non-matches); dict-in-enum key
  order. No committed fixture exercises any of these.

## Port notes (P3) — orchestrator fidelity contract

- `find_workflow` overlay semantics kept: `<project>/.shiploom/workflows/`
  shadows tool core; `name.md` suffix and legacy `workflows/` prefix
  accepted. Tool root derives from the schemas dir (SHIPLOOM_SCHEMAS or
  CWD/exe probing); `SHIPLOOM_CORE_DIR` defaults to it when unset so the
  default policy pack and hooks resolve from scratch dirs.
- `resolve_uses`: overlay path then tool core; skill dirs resolve to
  `SKILL.md`; always a path (hint prints its basename).
- `_glob_hits` follows pathlib: literal-exists fast path (absolute
  patterns stay absolute), single `*` never crosses `/`, `**` recurses,
  dangling links filtered, hits sorted as strings.
- `load_workflow`: parse-error first, then the no-frontmatter message;
  `.md`-only (no JSON branch, no extra steps-shape check — schema owns it).
- Stepper: binding adoption/guard, wallClockH adoption + overrides,
  `--from` reset (steps + retries only, gates kept), `--only`
  predecessor guard, human/policy/verification gates, retry counting with
  `onFail: abort|replan`, checkpoints with post-save sha256, audit appends
  (`run.start/step.done/complete/paused/retry/replan/reopened/policy.deny`).
  Exit 3 only from policy deny; exit 4 only from wall-clock breach.
- New deliberate divergences: audit/save failures crash CPython but return
  exit 2 with a report error in Go; only the missing-manifest
  FileNotFoundError text is synthesized (`[Errno 2] ...`), other OSError
  texts differ; `status.invalid` order follows directory walk order
  (fixtures keep at most one invalid file); argparse-only error paths
  (`--help` text, usage wrapping, `prog:`-prefixed errors) are not yet
  byte-identical — all covered flag behaviors are.

## Port notes (P4) — lock/verify fidelity contract

- `lock`: reuses the P2 `internal/oracle` library (discovery, vault 0700,
  git-leak check, hash compare). Human lines (`locked N criteria...`,
  `acceptance lock: ok/BROKEN`, `lock failed:`, `fail:`/`warn:`) and JSON
  (`ok/errors/warnings/summary`) match; empty-failure summary stays `{}`.
  `--actor` accepts space and `=` forms.
- `verify`: reuses `internal/gates` (fixed `GateOrder`, quality insertion
  order `compile/secrets/tests/license/mutation/determinism`). Human lines
  (`verify: verdict`, `[STAT] gate detail`, `quality key value`,
  `fail:`, `wrote verification/gate-report.json`) match; `--gates`
  accepts space/`=` forms, empty selection means full run, unknown names
  fail like Python. `--report` writes `verification/gate-report.json`
  with canonical JSON + trailing newline.
- Fixed `gates.toReportMap` to emit jsoncanon-compatible types only
  (`quality`/`warnings` as `map[string]any`, `errors` as `[]any`); the
  writer supports no other map/slice shapes.
- Parity avoids timing nondeterminism: `verify` fixtures assert human
  output only (never `--json`, whose `durationS` varies); `lock` JSON is
  duration-free and safe.

## Port notes (P4) — trace/approve/budget/resume fidelity contract

- `trace`: reuses P3 `internal/trace` (`BuildTrace` with `strict=false`).
  Error lines (`  fail: path: msg` to stdout) and unknown-id
  (`unknown id %r`, exit 2) match; human relations and incoming refs are
  alphabetically sorted; JSON (`ok/id/links/referencedBy/warnings`) matches.
- `approve`: reuses `run.LoadManifest` for the exact `no manifest: ...`
  shape, `workflow.FindWorkflow`/`LoadWorkflow` for resolution, and
  `manifest.Utcnow`/`auditlog.Append` for writes. Gate lookup, approvable
  check (`human-approval|policy`), deny-requires-reason, and human/JSON
  outputs match. `--actor`/`--reason` accept space and `=` forms. JSON
  carries a live `at` timestamp so parity fixtures use human output only.
- `budget`: `[path]` positional defaults to `.`; `--set` validates keys
  (`tokens|spendUSD|wallClockH`, `%r` on unknown) and mirrors setdefault
  semantics (new slots get `used: 0`, existing slots keep theirs);
  numbers print via `PyStr` so `0`/`25.0` literals survive. Audit
  `budget.set` targets the sorted key list.
- `resume`: bound-workflow guard, budget parsing, workflow order, pending
  gates, and position (`done/total/next/pendingGates`, `next: complete`
  when done) match; then the stepper report reuses run's human lines.

## Port notes (P4) — audit/characterize fidelity contract

- `audit`: `Verify` runs first in every mode (exit from chain health);
  `--export` accepts space and `=` forms with `json|md` choices enforced.
  JSON (`ok/errors/entries`) and md table match; human
  (`audit: N entries, chain ok/BROKEN`) matches. Fixtures pre-build the
  log via `budget --set` in setup so both runtimes replay identical
  timestamps/hashes (pristine seed copies).
- `characterize`: required-exclusive `--capture|--diff|--list` enforced;
  `--command`/`--timeout`/`--actor` accept space and `=`; `--timeout`
  parses like `type=int` (trims space, tolerates `_`), negatives fail.
  List (`snapshots: a, b` or `none`), capture
  (`captured NAME: exit N sha LAST12`), and diff (`unchanged:` /
  `changed: NAME (exitChanged=X, outputChanged=Y)` + `  ` lines) match;
  diff-missing (`no snapshot %r...`, exit 2) matches.
- Fixed a latent P2 divergence in `internal/characterize`: snapshot-name
  quoting now uses `validate.PyRepr` (single-quote preference, not `%q`),
  and the missing-file error synthesizes the `[Errno 2] ...` text like
  `run.LoadManifest` does. Capture `--json` still exposes an int/float
  `timeoutS` difference (Python `600` vs Go `600.0`), so parity fixtures
  use human output only.

## Port notes (P4) — doctor fidelity contract

- New `internal/doctor` library mirrors `run_checks()`: Python version
  (via the operator `python3`, same binary the harness uses), `core/VERSION`,
  9 normative schemas (`$schema`/`$id` shape check), harness `claude`/
  `opencode` via `LookPath`, project config/manifest/audit/oracle-mode/
  mcp-registry (via the ported `mcp.AttestationStatus`), and `git`.
- Deliberate shims (documented): the `validators` check always passes
  (the Go binary embeds the port); missing-`python3` fails with a
  `requires >=3.9` message that has no Python-side counterpart (fixtures
  always have `python3`). Runtime `normalize.sed` already masks the
  `Python X.Y.Z` version string.
- Exit `5` (harness mismatch) on any failure; human
  (`shiploom doctor: OK/PROBLEMS (N fail, M warn)`, `[STAT] name detail`)
  and JSON (`ok/checks/failures/warnings`) match. Fixtures pre-build
  config/manifest/audit/oracle/registry in setup for determinism.

## Port notes (P4) — pin/upgrade fidelity contract

- `pin`: live pin reads the tool `core/VERSION` file (like Python
  `core_version()`), `python3 --version`, `runtime.GOOS` (matches
  `sys.platform` on darwin/linux; Windows `windows` vs `win32` is a
  documented divergence), and `--version` probes for harness CLIs with
  the same 10s-cap/first-line rules. `mcp` versions come from the
  ported registry loader (failures keep `{}`). Drift lines use `%r`
  via `PyRepr`; human outputs (`pinned ...`, `pin clean: ...`) are
  deterministic while `--json` carries a live `pinnedAt`, so fixtures
  use human lines (plus one JSON run verified manually).
- `upgrade`: manifest/config/backup round-trips reuse canonical JSON;
  dry-run payload, incompatible/stale/current branches, rollback
  restore + unlink, auto-rollback on failed strict validation (first 5
  non-`.shiploom` errors), and audit entries match. Missing-key order
  follows `REQUIRED_MANIFEST_KEYS`.
- Extended the `[Errno 2]` missing-file synthesis (previously manifest
  and snapshots only) to audit-log reads, doctor config/manifest reads,
  and upgrade config/backup/pin reads — all verified byte-identical
  against FileNotFoundError text.

## Port notes (P4) — adapters/add fidelity contract

- New `internal/adapters` library mirrors template substitution
  (`{{projectName, coreVersion, workflow}}`, unknown vars verbatim),
  skill-header insertion after frontmatter, sorted outputs/skills, and
  the `created/updated/unchanged` write journal (hooks get `+x`).
  `all` folds per-adapter reports. `unknown adapter %r` / `bad
  mapping.json for %r` match; JSON parse details stay engine-suffixed
  (documented). Outputs list relative paths only, so the
  `projectName`-in-content difference between `py/` and `go/` copies
  never reaches parity comparisons.
- New `internal/add` library mirrors kind/name/source validation,
  local-vs-git fetch (shallow tag-pinned clone, 120s cap), dir/file
  payload location, mode-preserving copy (`__pycache__` skipped),
  strict validation with the `links.dangling` carve-out, rollback, and
  unsigned provenance (`tag: null` when absent).
- Fixed a latent P2 doc-vs-code mismatch in `internal/auditlog`: the
  package doc always promised spaced `json.dump`-default file lines but
  the code wrote compact lines. New `jsoncanon.MarshalLine` (sorted
  keys, `", "`/`": "` separators, single line) is now used for log
  lines; hashes are unchanged (still compact-canonical) and the
  cross-implementation interop test still passes. Hash equality across
  implementations was re-verified on a shared chain.

## Port notes (P4) — conformance fidelity contract

- New `internal/conformance` library reuses the ported
  `adapters.Generate` (scratch dir), `validate.ValidatePath` (strict
  skill checks), and `validate.ParseFrontmatter` (generated-vs-core
  identity via deep equality). Guard behavior replays the fixture event
  through the operator `python3` (30s cap) with the same deny/silence
  rules; empty stdout parses as `{}` exactly like `json.loads(... or
  "{}")`. Opencode `instructions` honors both list-membership and
  string-substring shapes, like Python's `in`.
- Record writes (`--record`, under the tool's `tests/conformance/
  _records/`) share byte shapes with the library writer; fixtures never
  use `--record` (it would dirty the repo). Reports carry no durations
  or timestamps, so human and JSON outputs are both parity-safe.

## Port notes (P4) — install/init fidelity contract (P4 complete)

- `install`: version gate, strict pre-copy validation of `core/` +
  `examples/` (failures to stderr, first 10), `dirs_exist_ok` copy of
  `core/schemas/validators` (`__pycache__` skipped, modes preserved),
  receipt, and the `--global/--local` defaulting (`--global` when
  neither; conflict fails) match. `--global` fixtures are avoided (they
  write `$HOME`); `--local` is stateful with PROJ-masked paths.
- `init`: exclusive `--green/--existing`, harness allowlist (exit 5),
  0700 vault (chmod faults ignored), `.gitignore`, config (with
  `stack: null` default), registry, genesis manifest, audit
  genesis + `project.init`, and byte-copy seed files match. Dot-exists
  guard and seed kept/seeded notes match.
- Both resolve CWD symlinks (`EvalSymlinks`, mirroring `Path.cwd()`)
  so macOS `/tmp -> /private/tmp` reports match CPython byte-for-byte.
  The parity harness now also canonicalizes its scratch dir (`pwd -P`)
  so PROJ/TMP masking hits on symlinked TMPDIRs.
