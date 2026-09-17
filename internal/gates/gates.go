// Package gates ports cli/gates.py (stdlib only): deterministic
// verification gates plus the verify-step checker.
package gates

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shiploom/ai-builder/internal/characterize"
	"github.com/shiploom/ai-builder/internal/execrun"
	"github.com/shiploom/ai-builder/internal/jsoncanon"
	"github.com/shiploom/ai-builder/internal/manifest"
	"github.com/shiploom/ai-builder/internal/oracle"
	"github.com/shiploom/ai-builder/internal/validate"
)

// GateOrder mirrors run_gates() fixed order.
var GateOrder = []string{"build", "typecheck", "lint", "sast", "dast", "test",
	"contract", "secrets", "depAudit", "license", "mutation", "compile"}

// ConfiguredGates are resolved via execrun (command or skip).
var ConfiguredGates = map[string]bool{
	"build": true, "typecheck": true, "lint": true, "sast": true,
	"dast": true, "test": true, "contract": true,
}

// SecretRule is one compiled scanner rule.
type SecretRule struct {
	Name string
	Re   *regexp.Regexp
}

// SecretRules mirrors SECRET_RULES (verified against CPython corpus).
func SecretRules() []SecretRule {
	return []SecretRule{
		{"private-key", regexp.MustCompile(`-----BEGIN (?:RSA |OPENSSH |EC |DSA )?PRIVATE KEY-----`)},
		{"aws-key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
		{"token-prefix", regexp.MustCompile(`\b(?:ghp_|gho_|gsk_|sk-ant-|sk-proj-|xox[bpas]-)[A-Za-z0-9_-]{8,}`)},
		{"secret-assign", regexp.MustCompile(`(?i)\b(?:secret|token|password|passwd|api[_-]?key|auth[_-]?token)\b\s*[:=]\s*['"][^'"\s]{4,}['"]`)},
		{"conn-string", regexp.MustCompile(`(?i)\b(?:postgres|mysql|mongodb|redis)://[^/\s:]+:[^/\s@]+@[^\s]+`)},
	}
}

// Finding mirrors secret findings (values never included).
type Finding struct {
	Path string
	Line int
	Rule string
}

// SkipDirs mirrors gates.SKIP_DIRS.
var SkipDirs = map[string]bool{
	".git": true, ".venv": true, ".conda": true, "__pycache__": true,
	"node_modules": true, "dist": true, "build": true, ".oracle": true,
	".validator-cache": true, "shiploom_core.egg-info": true,
}

func isTextFile(path string) bool {
	fh, err := os.Open(path)
	if err != nil {
		return false
	}
	defer fh.Close()
	buf := make([]byte, 8192)
	n, err := fh.Read(buf)
	if err != nil && n == 0 {
		return n > 0
	}
	for _, b := range buf[:n] {
		if b == 0 {
			return false
		}
	}
	return true
}

// splitLines mirrors str.splitlines for \n texts.
func splitLines(text string) []string {
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	return lines
}

// Inventory mirrors dep_inventory() ({deps count, unpinned list}).
type Inventory struct {
	Deps     int
	Unpinned []string
}

func depInventory(projectDir string) Inventory {
	var deps, unpinned []string
	addReq := func(pattern string) {
		matches, _ := filepath.Glob(filepath.Join(projectDir, pattern))
		sort.Strings(matches)
		for _, req := range matches {
			raw, err := os.ReadFile(req)
			if err != nil {
				continue
			}
			for _, line := range splitLines(string(raw)) {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
					continue
				}
				name := strings.TrimSpace(regexp.MustCompile(`[<>=!~\s\[]`).Split(line, 2)[0])
				if name == "" {
					continue
				}
				deps = append(deps, "pip:"+name)
				if !strings.Contains(line, "==") {
					unpinned = append(unpinned, "pip:"+name)
				}
			}
		}
	}
	addReq("requirements*.txt")
	addReq(filepath.Join("requirements", "*.txt"))
	if raw, err := os.ReadFile(filepath.Join(projectDir, "package.json")); err == nil {
		if doc, err := jsoncanon.Decode(raw); err == nil {
			if obj, ok := doc.(map[string]any); ok {
				for _, section := range []string{"dependencies", "devDependencies"} {
					if depsMap, ok := obj[section].(map[string]any); ok {
						for name, spec := range depsMap {
							deps = append(deps, "npm:"+name)
							specStr := ""
							if s, ok := spec.(string); ok {
								specStr = strings.TrimSpace(s)
							} else {
								specStr = "?"
							}
							matched, _ := regexp.MatchString(`^[0-9]+\.[0-9]+\.[0-9]+$`, specStr)
							if !matched {
								unpinned = append(unpinned, "npm:"+name)
							}
						}
					}
				}
			}
		}
	}
	if raw, err := os.ReadFile(filepath.Join(projectDir, "go.mod")); err == nil {
		for _, line := range splitLines(string(raw)) {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				skip := map[string]bool{"module": true, "go": true, "require": true, ")": true, "//": true}
				if !skip[parts[0]] {
					deps = append(deps, "go:"+parts[0])
				}
			}
		}
	}
	return Inventory{Deps: len(uniqueStrings(deps)), Unpinned: uniqueSorted(unpinned)}
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func uniqueSorted(in []string) []string {
	out := uniqueStrings(in)
	sort.Strings(out)
	return out
}

// LicenseAllow mirrors LICENSE_ALLOW.
var LicenseAllow = map[string]bool{
	"MIT": true, "Apache-2.0": true, "ISC": true,
	"BSD-2-Clause": true, "BSD-3-Clause": true, "BSD-4-Clause": true,
	"PSF-2.0": true, "Python-2.0": true, "CC0-1.0": true, "Unlicense": true,
	"MPL-2.0": true, "LGPL-2.1-only": true, "LGPL-2.1-or-later": true,
	"LGPL-3.0-only": true, "LGPL-3.0-or-later": true, "EPL-2.0": true,
}

// LicenseAliases mirrors LICENSE_ALIASES.
var LicenseAliases = map[string]string{
	"MIT License": "MIT", "Apache License 2.0": "Apache-2.0",
	"Apache License, Version 2.0": "Apache-2.0", "Apache 2.0": "Apache-2.0",
	"BSD 3-Clause": "BSD-3-Clause", "BSD 2-Clause": "BSD-2-Clause",
	"BSD License": "BSD-3-Clause", "ISC License": "ISC",
	"Python Software Foundation License": "PSF-2.0",
}

func normalizeLicense(raw any) string {
	// Mirror str(raw or ""): falsy values become "".
	if raw == nil {
		return ""
	}
	text := validate.PyStr(raw)
	if text == "False" || text == "0" || text == "0.0" || text == "[]" || text == "{}" || text == "" {
		// Python falsy values (False, 0, [], {}) hit `or ""`.
		// ("False"/"0"/etc. can never be real SPDX ids.)
		return ""
	}
	text = strings.Trim(strings.TrimSpace(text), "()")
	if text == "" {
		return ""
	}
	if alias, ok := LicenseAliases[text]; ok {
		return alias
	}
	return text
}

// LicenseInventory mirrors license_inventory().
func LicenseInventory(projectDir string) (declared map[string]string, unknown, nonAllowlisted []string) {
	declared = map[string]string{}
	lockRaw, _ := os.ReadFile(filepath.Join(projectDir, "package-lock.json"))
	if lockRaw != nil {
		if doc, err := jsoncanon.Decode(lockRaw); err == nil {
			if obj, ok := doc.(map[string]any); ok {
				if packages, ok := obj["packages"].(map[string]any); ok {
					for path, meta := range packages {
						if path == "" {
							continue
						}
						metaObj, ok := meta.(map[string]any)
						if !ok {
							continue
						}
						segs := strings.Split(path, "node_modules/")
						name := strings.Split(segs[len(segs)-1], "/")[0]
						if lic := normalizeLicense(metaObj["license"]); lic != "" {
							declared["npm:"+name] = lic
						}
					}
				}
			}
		}
	}
	for _, dep := range sortedDepNames(projectDir) {
		eco, name := splitDep(dep)
		if eco == "npm" {
			if _, ok := declared["npm:"+name]; ok {
				continue
			}
		}
		unknown = append(unknown, dep)
	}
	for name, lic := range declared {
		if !LicenseAllow[lic] {
			nonAllowlisted = append(nonAllowlisted, name)
		}
	}
	sort.Strings(unknown)
	unknown = uniqueStrings(unknown)
	sort.Strings(nonAllowlisted)
	return declared, unknown, nonAllowlisted
}

func sortedDepNames(projectDir string) []string {
	names := map[string]bool{}
	addReq := func(pattern string) {
		matches, _ := filepath.Glob(filepath.Join(projectDir, pattern))
		sort.Strings(matches)
		for _, req := range matches {
			raw, err := os.ReadFile(req)
			if err != nil {
				continue
			}
			for _, line := range splitLines(string(raw)) {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
					continue
				}
				name := strings.TrimSpace(regexp.MustCompile(`[<>=!~\s\[]`).Split(line, 2)[0])
				if name != "" {
					names["pip:"+name] = true
				}
			}
		}
	}
	addReq("requirements*.txt")
	addReq(filepath.Join("requirements", "*.txt"))
	if raw, err := os.ReadFile(filepath.Join(projectDir, "package.json")); err == nil {
		if doc, err := jsoncanon.Decode(raw); err == nil {
			if obj, ok := doc.(map[string]any); ok {
				for _, section := range []string{"dependencies", "devDependencies"} {
					if depsMap, ok := obj[section].(map[string]any); ok {
						for name := range depsMap {
							names["npm:"+name] = true
						}
					}
				}
			}
		}
	}
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

var osvLockfiles = []string{"package-lock.json", "yarn.lock", "pnpm-lock.yaml",
	"Cargo.lock", "Gemfile.lock", "go.mod", "requirements.txt", "poetry.lock"}

// osvScan mirrors _osv_scan(): (vulns or nil, detail). Nil vulns with a
// reason means "fall back to inventory" (scanner absent or no lockfiles).
func osvScan(projectDir string, timeoutS float64) ([]string, string) {
	scanner, err := exec.LookPath("osv-scanner")
	if err != nil {
		return nil, "osv-scanner not on PATH"
	}
	found := false
	for _, name := range osvLockfiles {
		if info, err := os.Stat(filepath.Join(projectDir, name)); err == nil && !info.IsDir() {
			found = true
			break
		}
	}
	if !found {
		return nil, "no supported lockfiles"
	}
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(timeoutS*float64(time.Second)))
	defer cancel()
	cmd := exec.CommandContext(ctx, scanner, "--format", "json", "--recursive", projectDir)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return nil, fmt.Sprintf("scanner error: %s", err)
		}
		// Nonzero exit with output: parse anyway (exit 1 = findings).
	}
	doc, err := jsoncanon.Decode(stdout.Bytes())
	if err != nil {
		// Mirror Python: json error inside the try becomes "scanner error".
		// An empty-results run still parses (exit 0, "{}").
		return nil, fmt.Sprintf("scanner error: %s", err)
	}
	obj, _ := doc.(map[string]any)
	var vulns []string
	if results, ok := obj["results"].([]any); ok {
		for _, r := range results {
			packages, _ := r.(map[string]any)["packages"].([]any)
			for _, p := range packages {
				pkg, _ := p.(map[string]any)
				info, _ := pkg["package"].(map[string]any)
				name, _ := info["name"].(string)
				version, _ := info["version"].(string)
				if vulnList, ok := pkg["vulnerabilities"].([]any); ok {
					for _, v := range vulnList {
						vm, _ := v.(map[string]any)
						id, _ := vm["id"].(string)
						if id == "" {
							id, _ = vm["summary"].(string)
						}
						if id == "" {
							id = "?"
						}
						vulns = append(vulns, fmt.Sprintf("%s@%s %s", name, version, id))
					}
				}
			}
		}
	}
	sort.Strings(vulns)
	seen := map[string]bool{}
	uniq := vulns[:0]
	for _, v := range vulns {
		if !seen[v] {
			seen[v] = true
			uniq = append(uniq, v)
		}
	}
	if len(uniq) > 0 {
		return uniq, fmt.Sprintf("%d vuln(s)", len(uniq))
	}
	return uniq, "clean"
}

// GateResult mirrors one gate entry.
type GateResult struct {
	Status    string
	Command   string
	HasCmd    bool
	Exit      int
	HasExit   bool
	DurationS float64
	Detail    string
	Tail      string
	HasTail   bool
	Extra     map[string]any
}

// toMap renders with Python key shapes (command/exit/tail only when set).
func (g GateResult) toMap() map[string]any {
	m := map[string]any{"status": g.Status, "durationS": g.DurationS, "detail": g.Detail}
	if g.HasCmd {
		m["command"] = g.Command
	}
	if g.HasExit {
		m["exit"] = int64(g.Exit)
	}
	if g.HasTail {
		m["tail"] = g.Tail
	}
	for k, v := range g.Extra {
		m[k] = v
	}
	return m
}

// Report mirrors run_gates() output.
type Report struct {
	Ok           bool
	Gates        map[string]GateResult
	Order        []string
	Quality      map[string]string
	QualityOrder []string
	Warnings     map[string]string
	Verdict      string
	Errors       []string
}

func toReportMap(r Report) map[string]any {
	gates := map[string]any{}
	for name, g := range r.Gates {
		gates[name] = g.toMap()
	}
	warnings := map[string]any{}
	for k, v := range r.Warnings {
		warnings[k] = v
	}
	quality := map[string]any{}
	for k, v := range r.Quality {
		quality[k] = v
	}
	errs := make([]any, 0, len(r.Errors))
	for _, e := range r.Errors {
		errs = append(errs, e)
	}
	return map[string]any{
		"ok": r.Ok, "gates": gates, "quality": quality,
		"warnings": warnings, "verdict": r.Verdict, "errors": errs,
	}
}

// ToMap renders the report exactly like run_gates() JSON (sorted keys
// handled by the writer). Public for the verify CLI.
func (r Report) ToMap() map[string]any {
	return toReportMap(r)
}

// RunGates mirrors run_gates(). selected=nil runs the fixed order;
// unknown names fail like Python.
func RunGates(projectDir string, selected []string) Report {
	configured := execrun.LoadGateConfig(projectDir)
	order := append([]string{}, GateOrder...)
	if selected != nil {
		var unknown []string
		keep := map[string]bool{}
		for _, g := range selected {
			keep[g] = true
		}
		for _, g := range selected {
			known := false
			for _, o := range GateOrder {
				if o == g {
					known = true
				}
			}
			if !known {
				unknown = append(unknown, g)
			}
		}
		if len(unknown) > 0 {
			return Report{Ok: false, Gates: map[string]GateResult{},
				Quality: map[string]string{}, Warnings: map[string]string{},
				Verdict: "fail",
				Errors:  []string{fmt.Sprintf("unknown gates: %s", strings.Join(unknown, ", "))}}
		}
		var filtered []string
		for _, g := range order {
			if keep[g] {
				filtered = append(filtered, g)
			}
		}
		order = filtered
	}
	report := Report{Gates: map[string]GateResult{}, Quality: map[string]string{},
		Warnings: map[string]string{}, Errors: []string{}}
	var testCmd string
	var testTimeout float64 = execrun.DefaultTimeoutS
	testCmdSet := false

	configTimeout := func(gateID string) float64 {
		if entry, ok := configured[gateID].(map[string]any); ok {
			if f, ok := execrun.ToFloat(entry["timeoutS"]); ok {
				return f
			}
		}
		return execrun.DefaultTimeoutS
	}

	for _, gateID := range order {
		switch gateID {
		case "build", "typecheck", "lint", "sast", "dast", "test", "contract":
			command, reason, ok := resolveGateCommand(projectDir, configured, gateID)
			if !ok {
				report.Gates[gateID] = GateResult{Status: "skip", DurationS: 0.0, Detail: reason}
				continue
			}
			timeoutS := configTimeout(gateID)
			res := execrun.RunCommand(projectDir, command, timeoutS)
			report.Gates[gateID] = GateResult{Status: res.Status, Command: command,
				HasCmd: true, Exit: res.Exit, HasExit: true, DurationS: res.DurationS,
				Detail: res.Detail, Tail: res.Tail, HasTail: true}
			if gateID == "test" {
				testCmd, testTimeout, testCmdSet = command, timeoutS, true
			}
		case "secrets":
			started := nowMono()
			findings := ScanSecrets(projectDir)
			detail := "clean"
			status := "pass"
			if len(findings) > 0 {
				status = "fail"
				detail = fmt.Sprintf("%d finding(s)", len(findings))
			}
			items := make([]any, 0, len(findings))
			for _, f := range findings {
				items = append(items, map[string]any{
					"path": f.Path, "line": int64(f.Line), "rule": f.Rule})
			}
			report.Gates[gateID] = GateResult{Status: status,
				DurationS: elapsed(started), Detail: detail,
				Extra: map[string]any{"findings": items}}
		case "depAudit":
			started := nowMono()
			inv := depInventory(projectDir)
			if len(inv.Unpinned) > 0 {
				top := inv.Unpinned
				if len(top) > 10 {
					top = top[:10]
				}
				report.Warnings["depAudit"] = "unpinned: " + strings.Join(top, ", ")
			}
			timeoutS := configTimeout(gateID)
			vulns, vulnDetail := osvScan(projectDir, timeoutS)
			invMap := map[string]any{"deps": int64(inv.Deps), "unpinned": inv.Unpinned}
			if invMap["unpinned"] == nil {
				invMap["unpinned"] = []any{}
			} else {
				list := make([]any, 0, len(inv.Unpinned))
				for _, u := range inv.Unpinned {
					list = append(list, u)
				}
				invMap["unpinned"] = list
			}
			if vulns == nil {
				report.Gates[gateID] = GateResult{Status: "pass",
					DurationS: elapsed(started),
					Detail: fmt.Sprintf("%d deps, %d unpinned (%s)",
						inv.Deps, len(inv.Unpinned), vulnDetail),
					Extra: map[string]any{"inventory": invMap}}
			} else {
				status := "pass"
				if len(vulns) > 0 {
					status = "fail"
				}
				items := make([]any, 0, len(vulns))
				for _, v := range vulns {
					items = append(items, v)
				}
				report.Gates[gateID] = GateResult{Status: status,
					DurationS: elapsed(started),
					Detail:    "osv-scanner: " + vulnDetail,
					Extra: map[string]any{"inventory": invMap,
						"vulnerabilities": items}}
			}
		case "license":
			started := nowMono()
			declared, unknown, nonAllowlisted := LicenseInventory(projectDir)
			if len(nonAllowlisted) > 0 {
				top := nonAllowlisted
				if len(top) > 10 {
					top = top[:10]
				}
				report.Warnings["license"] = "non-allowlisted: " + strings.Join(top, ", ")
			}
			declaredAny := map[string]any{}
			for k, v := range declared {
				declaredAny[k] = v
			}
			unknownAny := make([]any, 0, len(unknown))
			for _, u := range unknown {
				unknownAny = append(unknownAny, u)
			}
			nonAny := make([]any, 0, len(nonAllowlisted))
			for _, u := range nonAllowlisted {
				nonAny = append(nonAny, u)
			}
			report.Gates[gateID] = GateResult{Status: "pass",
				DurationS: elapsed(started),
				Detail: fmt.Sprintf("%d declared, %d unknown, %d non-allowlisted (report-only)",
					len(declared), len(unknown), len(nonAllowlisted)),
				Extra: map[string]any{"inventory": map[string]any{
					"declared": declaredAny, "unknown": unknownAny, "nonAllowlisted": nonAny}}}
		case "mutation":
			started := nowMono()
			candidates, sample, err := MutationSample(projectDir, MutationLimit)
			if err != nil {
				// Mirror Python: sampler faults would crash _run_command's
				// caller chain... actually mutation_sample has no runner
				// faults (pure walk); an error here means python3 missing.
				// Report skip with reason (documented: needs python3).
				report.Gates[gateID] = GateResult{Status: "skip",
					DurationS: elapsed(started),
					Detail:    fmt.Sprintf("no python3 on PATH (%s)", err)}
				continue
			}
			items := make([]any, 0, len(sample))
			for _, s := range sample {
				items = append(items, map[string]any{
					"path": s["path"], "line": toInt64(s["line"]), "kind": s["kind"]})
			}
			report.Gates[gateID] = GateResult{Status: "pass",
				DurationS: elapsed(started),
				Detail: fmt.Sprintf("%d candidates sampled (report-only; thresholds post-pilot)",
					candidates),
				Extra: map[string]any{"sample": map[string]any{
					"candidates": int64(candidates), "sample": items}}}
		case "compile":
			// Divergence note: CPython always has sys.executable; Go
			// skips when no python exists (documented degradation).
			if cmd, ok := compileCommand(); ok {
				res := execrun.RunCommand(projectDir, cmd, execrun.DefaultTimeoutS)
				detail := res.Detail
				if res.Status == "pass" {
					detail = "all Python files compile"
				}
				report.Gates[gateID] = GateResult{Status: res.Status, Command: res.Command,
					HasCmd: true, Exit: res.Exit, HasExit: true, DurationS: res.DurationS,
					Detail: detail}
			} else {
				report.Gates[gateID] = GateResult{Status: "skip", DurationS: 0.0,
					Detail: "no python3 on PATH"}
			}
		}
	}

	report.Quality["compile"] = gateStatus(report, "compile")
	report.Quality["secrets"] = gateStatus(report, "secrets")
	report.Quality["tests"] = gateStatus(report, "test")
	if lic, ok := report.Gates["license"]; ok {
		report.Quality["license"] = lic.Status
	} else {
		report.Quality["license"] = "skip"
	}
	if mut, ok := report.Gates["mutation"]; ok {
		report.Quality["mutation"] = mut.Detail
	} else {
		report.Quality["mutation"] = "not-run (report-only sampling per 33-D2)"
	}
	report.QualityOrder = []string{"compile", "secrets", "tests", "license", "mutation", "determinism"}
	if g, ok := report.Gates["test"]; ok && g.Status == "pass" && testCmdSet {
		second := execrun.RunCommand(projectDir, testCmd, testTimeout)
		if second.Status == "pass" {
			report.Quality["determinism"] = "pass (2 identical runs)"
		} else {
			report.Quality["determinism"] = "fail (flaky: second run " + second.Detail + ")"
			report.Gates["test"] = GateResult{Status: "fail", Command: testCmd,
				HasCmd: true, DurationS: second.DurationS,
				Detail: "flaky: " + second.Detail, Tail: second.Tail, HasTail: true}
		}
	} else {
		report.Quality["determinism"] = "not-run (no passing test gate)"
	}

	verdict := "pass"
	for _, g := range report.Gates {
		if g.Status == "fail" {
			verdict = "fail"
		}
	}
	report.Verdict = verdict
	report.Ok = verdict == "pass"
	return report
}

// MutationLimit mirrors MUTATION_LIMIT.
const MutationLimit = 20

// mutationSamplerPy is byte-identical in behavior to mutation_sample():
// same walk, same node kinds, same sort, same cap. It runs under the
// operator's python3 because porting CPython's AST walker would be a
// second parser to keep in lockstep — the correct tool for Python ASTs
// is Python's own parser. Output JSON matches {"candidates", "sample"}.
const mutationSamplerPy = `
import ast, json, os, sys
SKIP = {".git", ".venv", ".conda", "__pycache__", "node_modules",
        "dist", "build", ".oracle", ".validator-cache", "shiploom_core.egg-info"}
LIMIT = 20
def _kind(node):
    if isinstance(node, ast.Compare):
        return "comparison"
    if isinstance(node, ast.BoolOp):
        return "boolean-op"
    if isinstance(node, ast.UnaryOp) and isinstance(node.op, ast.Not):
        return "negation"
    if (isinstance(node, ast.Return) and isinstance(node.value, ast.Constant)
            and isinstance(node.value.value, bool)):
        return "boolean-return"
    if isinstance(node, ast.BinOp):
        return "arithmetic"
    return None
try:
    limit = int(sys.argv[2]) if len(sys.argv) > 2 else LIMIT
except ValueError:
    limit = LIMIT
root = sys.argv[1]
found = []
for dirpath, dirnames, filenames in os.walk(root):
    dirnames[:] = [d for d in dirnames if d not in SKIP and d != "tests"]
    for fn in sorted(filenames):
        if not fn.endswith(".py"):
            continue
        path = os.path.join(dirpath, fn)
        try:
            with open(path, "r", encoding="utf-8") as fh:
                tree = ast.parse(fh.read())
        except (OSError, ValueError, SyntaxError):
            continue
        for node in ast.walk(tree):
            k = _kind(node)
            if k:
                found.append({"path": os.path.relpath(path, root),
                              "line": getattr(node, "lineno", 0), "kind": k})
found.sort(key=lambda c: (c["path"], c["line"], c["kind"]))
print(json.dumps({"candidates": len(found), "sample": found[:limit]}, sort_keys=True))
`

// MutationSample mirrors mutation_sample() via the embedded sampler.
// Returns (candidates, sample, error). Runner faults surface as errors;
// callers decide fail vs skip (run_gates treats them as tool errors).
func MutationSample(projectDir string, limit int) (int, []map[string]any, error) {
	py, err := exec.LookPath("python3")
	if err != nil {
		return 0, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(execrun.DefaultTimeoutS)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, py, "-c", mutationSamplerPy, projectDir, strconv.Itoa(limit))
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Dir = projectDir
	if err := cmd.Run(); err != nil {
		return 0, nil, err
	}
	doc, err := jsoncanon.Decode(stdout.Bytes())
	if err != nil {
		return 0, nil, err
	}
	obj, ok := doc.(map[string]any)
	if !ok {
		return 0, nil, fmt.Errorf("sampler returned non-object")
	}
	candidates := 0
	if n, ok := obj["candidates"].(json.Number); ok {
		candidates, _ = strconv.Atoi(string(n))
	} else if n, ok := obj["candidates"].(float64); ok {
		candidates = int(n)
	}
	var sample []map[string]any
	if list, ok := obj["sample"].([]any); ok {
		for _, item := range list {
			if m, ok := item.(map[string]any); ok {
				sample = append(sample, m)
			}
		}
	}
	if sample == nil {
		sample = []map[string]any{}
	}
	return candidates, sample, nil
}

// ScanSecrets mirrors scan_secrets().
func ScanSecrets(projectDir string) []Finding {
	rules := SecretRules()
	var findings []Finding
	_ = filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if path != projectDir && SkipDirs[base] {
				return filepath.SkipDir
			}
			return nil
		}
		if !isTextFile(path) {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		// Mirror errors="strict": skip undecodable files.
		if !utf8.Valid(raw) {
			return nil
		}
		content := string(raw)
		lineno := 0
		for _, line := range splitLines(content) {
			lineno++
			for _, rule := range rules {
				if rule.Re.MatchString(line) {
					rel, err := filepath.Rel(projectDir, path)
					if err != nil {
						rel = path
					}
					findings = append(findings, Finding{Path: rel, Line: lineno, Rule: rule.Name})
					break
				}
			}
		}
		return nil
	})
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		return findings[i].Line < findings[j].Line
	})
	if findings == nil {
		findings = []Finding{}
	}
	return findings
}

func splitDep(dep string) (string, string) {
	if i := strings.Index(dep, ":"); i >= 0 {
		return dep[:i], dep[i+1:]
	}
	return "", dep
}

func gateStatus(report Report, gateID string) string {
	if g, ok := report.Gates[gateID]; ok {
		return g.Status
	}
	return "skip"
}

// resolveGateCommand mirrors the command-resolution half of the
// configurable-gate branch (configured entry or test auto-detect).
// Returns (command, reason, ok); timeouts resolve separately.
func resolveGateCommand(projectDir string, configured map[string]any, gateID string) (string, string, bool) {
	if entry, present := configured[gateID]; present {
		if obj, isMap := entry.(map[string]any); isMap {
			if cmd, _ := obj["command"].(string); cmd != "" {
				return cmd, "configured", true
			}
		} else if cmd, isStr := entry.(string); isStr && cmd != "" {
			return cmd, "configured", true
		}
	}
	if gateID == "test" {
		if cmd, _, reason, ok := execrun.ResolveTestCommand(projectDir); ok {
			return cmd, reason, true
		}
		return "", "no tests/ directory", false
	}
	return "", "not configured", false
}

// compileCommand mirrors the compile gate's interpreter lookup: CPython
// always has sys.executable; Go degrades to skip when no python exists.
func compileCommand() (string, bool) {
	for _, binary := range []string{"python3", "python"} {
		if _, err := exec.LookPath(binary); err == nil {
			return binary + " -m compileall -q .", true
		}
	}
	return "", false
}

func nowMono() time.Time {
	return time.Now()
}

func elapsed(started time.Time) float64 {
	secs := time.Since(started).Seconds()
	return float64(int(secs*100+0.5)) / 100
}

func toInt64(v any) int64 {
	if f, ok := execrun.ToFloat(v); ok {
		return int64(f)
	}
	return 0
}

// VerifyStepCheck mirrors verify_step_check(): full default gate set,
// report twin (schema, verdict, locked criteria), then snapshot diffs.
// Returns (ok, errors, warnings).
func VerifyStepCheck(projectDir string, schemas *validate.Schemas) (bool, []validate.Entry, []validate.Entry) {
	report := RunGates(projectDir, nil)
	var failed []string
	for _, name := range GateOrder {
		if g, ok := report.Gates[name]; ok && g.Status == "fail" {
			failed = append(failed, name)
		}
	}
	// Mirror sorted(...) (code-point order, like Python).
	sort.Strings(failed)
	if len(failed) > 0 {
		return false, []validate.Entry{{Path: projectDir,
			Message: "gate failed: " + strings.Join(failed, ", ")}}, nil
	}
	twinPath := filepath.Join(projectDir, "verification", "verification-report.json")
	twinRaw, err := os.ReadFile(twinPath)
	if err != nil {
		return false, []validate.Entry{{Path: "verification/verification-report.json",
			Message: "verifier report twin missing"}}, nil
	}
	twin, err := validate.DecodeJSON(twinRaw)
	if err != nil {
		return false, []validate.Entry{{Path: twinPath,
			Message: fmt.Sprintf("invalid JSON: %s", err)}}, nil
	}
	schema, err := schemas.Load("verification-report")
	if err != nil {
		return false, []validate.Entry{{Path: twinPath, Message: err.Error()}}, nil
	}
	if schemaErrors := validate.ValidateAgainstSchema(twin, schema, "$"); len(schemaErrors) > 0 {
		return false, []validate.Entry{{Path: twinPath, Message: schemaErrors[0]}}, nil
	}
	twinObj, _ := twin.(map[string]any)
	if verdict, _ := twinObj["verdict"].(string); verdict != "pass" {
		return false, []validate.Entry{{Path: twinPath,
			Message: fmt.Sprintf("verdict is %s, not pass", validate.PyRepr(twinObj["verdict"]))}}, nil
	}
	locked := map[string]any{}
	if data, err := manifest.Load(projectDir); err == nil {
		if acceptance, ok := data["acceptance"].(map[string]any); ok {
			if criteria, ok := acceptance["criteria"].(map[string]any); ok {
				locked = criteria
			}
		}
	}
	if len(locked) > 0 {
		var unlocked []string
		if results, ok := twinObj["results"].([]any); ok {
			for _, r := range results {
				if obj, ok := r.(map[string]any); ok {
					if id, _ := obj["acceptanceId"].(string); id != "" {
						if _, ok := locked[id]; !ok {
							unlocked = append(unlocked, id)
						}
					}
				}
			}
		}
		// Mirror sorted(set(...)) with dedup.
		seen := map[string]bool{}
		var uniq []string
		for _, id := range unlocked {
			if !seen[id] {
				seen[id] = true
				uniq = append(uniq, id)
			}
		}
		sort.Strings(uniq)
		if len(uniq) > 0 {
			return false, []validate.Entry{{Path: twinPath,
				Message: "unlocked criteria in report: " + strings.Join(uniq, ", ")}}, nil
		}
	} else if len(oracle.DiscoverAcceptance(projectDir)) > 0 {
		return false, []validate.Entry{{Path: projectDir,
			Message: "acceptance present but not locked"}}, nil
	}
	var snapWarnings []validate.Entry
	for _, name := range characterize.ListSnapshots(projectDir) {
		result := characterize.Diff(projectDir, name, 0)
		if result.HasError {
			return false, []validate.Entry{{
				Path:    ".shiploom/characterization/" + name + ".json",
				Message: "snapshot diff error: " + result.Err}}, nil
		}
		if result.Changed {
			snapWarnings = append(snapWarnings, validate.Entry{
				Path: ".shiploom/characterization/" + name + ".json",
				Message: fmt.Sprintf("behavior changed since capture (exitChanged=%s, outputChanged=%s); review diff at merge-approval",
					pyBool(result.ExitChanged), pyBool(result.OutputChanged))})
		}
	}
	if snapWarnings == nil {
		snapWarnings = []validate.Entry{}
	}
	return true, nil, snapWarnings
}

func pyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}
