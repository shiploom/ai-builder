// Package conformance ports cli/conformance.py (stdlib only):
// deterministic harness-conformance checks. Recorded-transcript mode is
// the default (no LLM calls). Generates into a scratch dir, then asserts
// must-produce files, skill validity, frontmatter identity, and
// harness-specific contracts.
package conformance

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/shiploom/ai-builder/internal/adapters"
	"github.com/shiploom/ai-builder/internal/jsoncanon"
	"github.com/shiploom/ai-builder/internal/validate"
)

// AllHarnesses mirrors ALL_HARNESSES (fixed order).
var AllHarnesses = []string{"base", "claude", "opencode"}

// Check mirrors one {"name","status","detail"} entry.
type Check struct {
	Name   string
	Status string
	Detail string
}

// ToMap renders the check mapping.
func (c Check) ToMap() map[string]any {
	return map[string]any{"name": c.Name, "status": c.Status, "detail": c.Detail}
}

func pass(name, detail string) Check { return Check{name, "pass", detail} }
func fail(name, detail string) Check { return Check{name, "fail", detail} }

// skillNames mirrors _skill_names().
func skillNames(toolRoot string) []string {
	items, err := os.ReadDir(filepath.Join(toolRoot, "core", "skills"))
	if err != nil {
		return nil
	}
	var out []string
	for _, item := range items {
		if item.IsDir() {
			out = append(out, item.Name())
		}
	}
	sort.Strings(out)
	return out
}

// checkTree mirrors _check_tree().
func checkTree(toolRoot, harness, root string, schemas *validate.Schemas) []Check {
	profilePath := filepath.Join(toolRoot, "tests", "conformance", harness, "capabilities.json")
	raw, err := os.ReadFile(profilePath)
	if err != nil {
		return []Check{fail("profile-loads", err.Error())}
	}
	profileDoc, err := jsoncanon.Decode(raw)
	if err != nil {
		return []Check{fail("profile-loads", err.Error())}
	}
	profile, _ := profileDoc.(map[string]any)
	if profile == nil {
		return []Check{fail("profile-loads", "not an object")}
	}
	checks := []Check{pass("profile-loads", profilePath)}

	var mapping *adapters.Adapter
	for _, a := range adapters.ListAdapters(toolRoot) {
		if a.Name == harness {
			m := a
			mapping = &m
			break
		}
	}
	if mapping == nil {
		return append(checks, fail("adapter-registered", harness))
	}
	checks = append(checks, pass("adapter-registered", "v"+mapping.Version))
	profileVersion, _ := profile["version"].(string)
	if mapping.Version != profileVersion {
		checks = append(checks, fail("adapter-profile-version",
			fmt.Sprintf("adapter v%s != profile v%s", mapping.Version, profileVersion)))
	} else {
		checks = append(checks, pass("adapter-profile-version", "v"+mapping.Version))
	}

	skillsDirs := map[string]string{"claude": ".claude/skills", "opencode": ".opencode/skills"}
	if dir, ok := skillsDirs[harness]; ok {
		var generated []string
		abs := filepath.Join(root, dir)
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			items, _ := os.ReadDir(abs)
			for _, item := range items {
				if item.IsDir() {
					if _, err := os.Stat(filepath.Join(abs, item.Name(), "SKILL.md")); err == nil {
						generated = append(generated, item.Name())
					}
				}
			}
			sort.Strings(generated)
		}
		want := skillNames(toolRoot)
		match := reflect.DeepEqual(generated, want)
		// Mirror `names == _skill_names()`: both sorted lists; nil vs []
		// compare equal only when both empty.
		if len(generated) == 0 && len(want) == 0 {
			match = true
		}
		checks = append(checks, Check{"skills-mirror-core",
			map[bool]string{true: "pass", false: "fail"}[match],
			fmt.Sprintf("%d skills", len(generated))})
		bad := 0
		for _, name := range generated {
			path := filepath.Join(abs, name, "SKILL.md")
			verr, warnings, _ := validate.ValidatePath(schemas, path, true)
			genRaw, genReadErr := os.ReadFile(path)
			genFm, _, genPerr := validate.ParseFrontmatter(string(genRaw))
			srcRaw, srcReadErr := os.ReadFile(filepath.Join(toolRoot, "core", "skills", name, "SKILL.md"))
			srcFm, _, srcPerr := validate.ParseFrontmatter(string(srcRaw))
			if len(verr) > 0 || len(warnings) > 0 || genPerr != "" || srcPerr != "" ||
				genReadErr != nil || srcReadErr != nil || !reflect.DeepEqual(genFm, srcFm) {
				bad++
			}
		}
		if bad == 0 {
			checks = append(checks, pass("skills-validate-clean", "all strict-clean"))
		} else {
			checks = append(checks, fail("skills-validate-clean", fmt.Sprintf("%d bad", bad)))
		}
	}

	if harness == "base" {
		agents := filepath.Join(root, "AGENTS.md")
		raw, err := os.ReadFile(agents)
		if err != nil {
			checks = append(checks, fail("agents-md", "missing AGENTS.md"))
		} else {
			fm, _, _ := validate.ParseFrontmatter(string(raw))
			ok := fm == nil && strings.Contains(string(raw), "DO NOT EDIT")
			checks = append(checks, Check{"agents-md",
				map[bool]string{true: "pass", false: "fail"}[ok],
				"plain Markdown with header"})
		}
	}

	if harness == "claude" {
		settingsPath := filepath.Join(root, ".claude", "settings.json")
		raw, err := os.ReadFile(settingsPath)
		if err != nil {
			checks = append(checks, fail("settings-shape", err.Error()))
		} else if doc, err := jsoncanon.Decode(raw); err != nil {
			checks = append(checks, fail("settings-shape", err.Error()))
		} else {
			obj, _ := doc.(map[string]any)
			var groups []any
			if obj != nil {
				if hooks, _ := obj["hooks"].(map[string]any); hooks != nil {
					groups, _ = hooks["PreToolUse"].([]any)
				}
			}
			hasGuard := false
			if groups != nil {
				for _, g := range groups {
					gm, _ := g.(map[string]any)
					if gm == nil {
						continue
					}
					hooks, _ := gm["hooks"].([]any)
					for _, h := range hooks {
						hm, _ := h.(map[string]any)
						if hm == nil {
							continue
						}
						if cmd, _ := hm["command"].(string); strings.Contains(cmd, "shiploom-guard.py") {
							hasGuard = true
						}
					}
				}
			}
			ok := groups != nil && hasGuard
			checks = append(checks, Check{"settings-shape",
				map[bool]string{true: "pass", false: "fail"}[ok],
				"PreToolUse guard wired"})
		}
		guard := filepath.Join(root, ".claude", "hooks", "shiploom-guard.py")
		if info, err := os.Stat(guard); err != nil || info.IsDir() {
			checks = append(checks, fail("guard-behavior", "missing guard script"))
		} else if guardDenies(guard, true) && guardDenies(guard, false) {
			checks = append(checks, pass("guard-behavior", "denies oracle writes, silent otherwise"))
		} else {
			checks = append(checks, fail("guard-behavior", "denies oracle writes, silent otherwise"))
		}
	}

	if harness == "opencode" {
		configPath := filepath.Join(root, "opencode.json")
		raw, err := os.ReadFile(configPath)
		if err != nil {
			checks = append(checks, fail("config-shape", err.Error()))
		} else if doc, err := jsoncanon.Decode(raw); err != nil {
			checks = append(checks, fail("config-shape", err.Error()))
		} else {
			obj, _ := doc.(map[string]any)
			ok := false
			if obj != nil {
				schema, _ := obj["$schema"].(string)
				// Mirror `"AGENTS.md" in (instructions or [])`: substring
				// when the value is a string, membership when a list.
				hasAgents := false
				switch ins := obj["instructions"].(type) {
				case string:
					hasAgents = strings.Contains(ins, "AGENTS.md")
				case []any:
					for _, item := range ins {
						if s, _ := item.(string); strings.Contains(s, "AGENTS.md") {
							hasAgents = true
						}
					}
				}
				perm, _ := obj["permission"].(map[string]any)
				edit := ""
				if perm != nil {
					edit, _ = perm["edit"].(string)
				}
				ok = schema == "https://opencode.ai/config.json" && hasAgents && edit == "ask"
			}
			checks = append(checks, Check{"config-shape",
				map[bool]string{true: "pass", false: "fail"}[ok],
				"instructions + permission defaults"})
		}
	}

	return checks
}

// guardDenies mirrors _guard_denies().
func guardDenies(guard string, oracleCase bool) bool {
	target := "src/app.py"
	if oracleCase {
		target = ".shiploom/.oracle/x"
	}
	event := fmt.Sprintf(`{"tool_name": "Edit", "tool_input": {"file_path": %q}}`, target)
	py, err := exec.LookPath("python3")
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, py, guard)
	cmd.Stdin = strings.NewReader(event)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	runErr := cmd.Run()
	code := 0
	if runErr != nil {
		exitErr, ok := runErr.(*exec.ExitError)
		if !ok {
			// Start/timeout faults mirror the (OSError,
			// SubprocessError) branch: no decision.
			return false
		}
		code = exitErr.ExitCode()
	}
	if oracleCase {
		// Mirror `json.loads(proc.stdout or "{}")`: empty stdout parses
		// to {} (no deny); malformed JSON fails the check.
		text := stdout.String()
		if strings.TrimSpace(text) == "" {
			text = "{}"
		}
		doc, err := jsoncanon.Decode([]byte(text))
		if err != nil {
			return false
		}
		obj, _ := doc.(map[string]any)
		if obj == nil {
			return false
		}
		hookOut, _ := obj["hookSpecificOutput"].(map[string]any)
		if hookOut == nil {
			return false
		}
		decision, _ := hookOut["permissionDecision"].(string)
		return code == 0 && decision == "deny"
	}
	return code == 0 && strings.TrimSpace(stdout.String()) == ""
}

// Report mirrors check_harness()'s report.
type Report struct {
	Harness  string
	Ok       bool
	Checks   []Check
	Failures int
	Errors   []string
}

// ToMap renders the report mapping (errors only when present, like Python:
// single-harness unknown carries errors; check runs carry checks).
func (r Report) ToMap() map[string]any {
	doc := map[string]any{"harness": r.Harness, "ok": r.Ok}
	if r.Errors != nil {
		items := make([]any, 0, len(r.Errors))
		for _, e := range r.Errors {
			items = append(items, e)
		}
		doc["errors"] = items
	} else {
		list := make([]any, 0, len(r.Checks))
		for _, c := range r.Checks {
			list = append(list, c.ToMap())
		}
		doc["checks"] = list
		doc["failures"] = int64(r.Failures)
	}
	return doc
}

// CheckHarness mirrors check_harness().
func CheckHarness(toolRoot, harness string, schemas *validate.Schemas) (bool, Report) {
	known := false
	for _, h := range AllHarnesses {
		if h == harness {
			known = true
		}
	}
	if !known {
		return false, Report{Harness: harness, Ok: false,
			Errors: []string{fmt.Sprintf("unknown harness %s", validate.PyRepr(harness))}}
	}
	tmp, err := os.MkdirTemp("", "shiploom-conf-")
	if err != nil {
		return false, Report{Harness: harness, Ok: false, Errors: []string{err.Error()}}
	}
	defer os.RemoveAll(tmp)
	report, genErrs := adapters.Generate(toolRoot, harness, tmp)
	_ = report
	if len(genErrs) > 0 {
		return false, Report{Harness: harness, Ok: false, Errors: genErrs}
	}
	checks := checkTree(toolRoot, harness, tmp, schemas)
	failures := 0
	for _, c := range checks {
		if c.Status == "fail" {
			failures++
		}
	}
	return failures == 0, Report{Harness: harness, Ok: failures == 0,
		Checks: checks, Failures: failures}
}

// RunAll mirrors run_all().
func RunAll(toolRoot string, schemas *validate.Schemas, record bool) (bool, map[string]Report) {
	results := map[string]Report{}
	for _, harness := range AllHarnesses {
		ok, report := CheckHarness(toolRoot, harness, schemas)
		_ = ok
		results[harness] = report
		if record {
			dir := filepath.Join(toolRoot, "tests", "conformance", "_records")
			_ = os.MkdirAll(dir, 0o755)
			raw, err := jsoncanon.Marshal(report.ToMap())
			if err == nil {
				_ = os.WriteFile(filepath.Join(dir, harness+".json"), append(raw, '\n'), 0o644)
			}
		}
	}
	allOk := true
	for _, r := range results {
		if !r.Ok {
			allOk = false
		}
	}
	return allOk, results
}
