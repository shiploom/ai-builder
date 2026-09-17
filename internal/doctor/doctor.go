// Package doctor ports cli/doctor.py (stdlib only): offline toolchain,
// harness, and project compatibility checks. Never touches the network.
// Statuses: pass / warn / fail. ok is false when anything fails.
package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shiploom/ai-builder/internal/auditlog"
	"github.com/shiploom/ai-builder/internal/jsoncanon"
	"github.com/shiploom/ai-builder/internal/mcp"
	"github.com/shiploom/ai-builder/internal/validate"
)

// SchemaFiles mirrors SCHEMA_FILES (normative set, fixed order).
var SchemaFiles = []string{
	"artifact-frontmatter.schema.json",
	"acceptance.schema.json",
	"skill.schema.json",
	"workflow.schema.json",
	"hook.schema.json",
	"policy.schema.json",
	"mcp-registry.schema.json",
	"verification-report.schema.json",
	"trace-link.schema.json",
}

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

// Report mirrors run_checks() output.
type Report struct {
	Ok       bool
	Checks   []Check
	Failures int
	Warnings int
}

// ToMap renders {"ok","checks","failures","warnings"}.
func (r Report) ToMap() map[string]any {
	list := make([]any, 0, len(r.Checks))
	for _, c := range r.Checks {
		list = append(list, c.ToMap())
	}
	return map[string]any{
		"ok": r.Ok, "checks": list,
		"failures": int64(r.Failures), "warnings": int64(r.Warnings),
	}
}

// FormatHuman mirrors format_human().
func (r Report) FormatHuman() string {
	verdict := "OK"
	if !r.Ok {
		verdict = "PROBLEMS"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "shiploom doctor: %s (%d fail, %d warn)\n", verdict, r.Failures, r.Warnings)
	for _, c := range r.Checks {
		fmt.Fprintf(&b, "  [%-4s] %-20s %s\n",
			strings.ToUpper(c.Status), c.Name, c.Detail)
	}
	return b.String()
}

// RunChecks mirrors run_checks(). toolRoot locates core/VERSION and
// schemas/ (parent of the schemas dir); schemas carries the loaded
// normative schemas for registry validation.
func RunChecks(schemas *validate.Schemas, schemasDir, toolRoot, projectDir string) Report {
	var checks []Check
	add := func(name, status, detail string) {
		checks = append(checks, Check{name, status, detail})
	}

	// --- toolchain / runtime ---
	if version, ok := pythonVersion(); ok {
		parts := strings.Split(version, ".")
		major, minor := 0, 0
		if len(parts) > 0 {
			fmt.Sscanf(parts[0], "%d", &major)
		}
		if len(parts) > 1 {
			fmt.Sscanf(parts[1], "%d", &minor)
		}
		if major > 3 || (major == 3 && minor >= 9) {
			add("python", "pass", "Python "+version)
		} else {
			add("python", "fail", "requires >=3.9, found "+version)
		}
	} else {
		add("python", "fail", "requires >=3.9, found none (python3 not on PATH)")
	}

	versionFile := filepath.Join(toolRoot, "core", "VERSION")
	coreRaw, err := os.ReadFile(versionFile)
	coreVersion := ""
	if err != nil {
		add("core-version", "fail", fmt.Sprintf("unreadable %s", versionFile))
	} else {
		coreVersion = strings.TrimSpace(string(coreRaw))
		add("core-version", "pass", coreVersion)
	}

	var bad []string
	for _, fname := range SchemaFiles {
		raw, err := os.ReadFile(filepath.Join(schemasDir, fname))
		if err != nil {
			bad = append(bad, fname)
			continue
		}
		doc, err := jsoncanon.Decode(raw)
		if err != nil {
			bad = append(bad, fname)
			continue
		}
		obj, _ := doc.(map[string]any)
		if obj == nil {
			bad = append(bad, fname)
			continue
		}
		schema, _ := obj["$schema"].(string)
		id, _ := obj["$id"].(string)
		if schema != "https://json-schema.org/draft/2020-12/schema" ||
			!strings.HasPrefix(id, "https://shiploom.dev/schemas/") {
			bad = append(bad, fname)
		}
	}
	// Mirror Python (missing is always empty there; only bad matters).
	if len(bad) > 0 {
		add("schemas", "fail", fmt.Sprintf("bad: %s", strings.Join(bad, ", ")))
	} else {
		add("schemas", "pass", fmt.Sprintf("%d normative schemas", len(SchemaFiles)))
	}

	// The Go binary embeds the ported validators; the importable check
	// always passes wherever the Python one does (documented shim).
	add("validators", "pass", "importable (stdlib-only)")

	// --- harnesses (warn-only) ---
	for _, harness := range []string{"claude", "opencode"} {
		if _, err := exec.LookPath(harness); err == nil {
			add("harness-"+harness, "pass", "CLI on PATH")
		} else {
			add("harness-"+harness, "warn", "CLI not on PATH")
		}
	}

	// --- project (only when initialized) ---
	shiploomDir := filepath.Join(projectDir, ".shiploom")
	if info, err := os.Stat(filepath.Join(shiploomDir, "config.json")); err == nil && !info.IsDir() {
		configPath := filepath.Join(shiploomDir, "config.json")
		raw, err := os.ReadFile(configPath)
		if err != nil {
			add("project-config", "fail", fmt.Sprintf("unreadable: %s", osErrText(configPath, err)))
		} else if doc, err := jsoncanon.Decode(raw); err != nil {
			add("project-config", "fail", fmt.Sprintf("unreadable: %s", err))
		} else if obj, ok := doc.(map[string]any); !ok {
			add("project-config", "fail", fmt.Sprintf("unreadable: %s", "not an object"))
		} else {
			_, hasCore := obj["coreVersion"]
			_, hasHarness := obj["harness"]
			if hasCore && hasHarness {
				add("project-config", "pass", fmt.Sprintf("harness=%s", validate.PyStr(obj["harness"])))
			} else {
				add("project-config", "fail", "missing coreVersion/harness keys")
			}
		}

		manifestPath := filepath.Join(shiploomDir, "manifest.json")
		raw2, err2 := os.ReadFile(manifestPath)
		if err2 != nil {
			add("project-manifest", "fail", fmt.Sprintf("unreadable: %s", osErrText(manifestPath, err2)))
		} else if doc, err := jsoncanon.Decode(raw2); err != nil {
			add("project-manifest", "fail", fmt.Sprintf("unreadable: %s", err))
		} else if obj, ok := doc.(map[string]any); !ok {
			add("project-manifest", "fail", "unreadable: not an object")
		} else if coreVersion != "" {
			if mv, _ := obj["coreVersion"].(string); mv != coreVersion {
				add("project-manifest", "fail", fmt.Sprintf(
					"core %s != tool core %s (run upgrade)", validate.PyStr(obj["coreVersion"]), coreVersion))
			} else if hasKeys(obj, "workflow", "budgets") {
				add("project-manifest", "pass", validate.PyStr(obj["workflow"]))
			} else {
				add("project-manifest", "fail", "missing workflow/budgets keys")
			}
		} else if hasKeys(obj, "workflow", "budgets") {
			add("project-manifest", "pass", validate.PyStr(obj["workflow"]))
		} else {
			add("project-manifest", "fail", "missing workflow/budgets keys")
		}

		ok, errs := auditlog.Verify(projectDir)
		if ok {
			add("project-audit", "pass", "chain ok")
		} else {
			add("project-audit", "fail", strings.Join(errs, "; "))
		}

		oracle := filepath.Join(shiploomDir, ".oracle")
		if info, err := os.Stat(oracle); err == nil && info.IsDir() {
			mode := info.Mode().Perm() & 0o777
			if mode == 0o700 {
				add("project-oracle", "pass", fmt.Sprintf("mode %o", mode))
			} else {
				add("project-oracle", "warn", fmt.Sprintf("mode %o (want 700)", mode))
			}
		} else {
			add("project-oracle", "warn", "no .oracle/ yet (created by init)")
		}

		registry := filepath.Join(shiploomDir, "mcp-registry.json")
		if info, err := os.Stat(registry); err == nil && !info.IsDir() {
			raw, err := os.ReadFile(registry)
			if err != nil {
				add("project-mcp-registry", "fail", fmt.Sprintf("unreadable: %s", err))
			} else if doc, err := jsoncanon.Decode(raw); err != nil {
				add("project-mcp-registry", "fail", fmt.Sprintf("unreadable: %s", err))
			} else {
				schema, serr := schemas.Load("mcp-registry")
				var schemaErrs []string
				if serr != nil {
					schemaErrs = []string{serr.Error()}
				} else {
					schemaErrs = validate.ValidateAgainstSchema(doc, schema, "$")
				}
				if len(schemaErrs) > 0 {
					top := schemaErrs
					if len(top) > 3 {
						top = top[:3]
					}
					add("project-mcp-registry", "fail", strings.Join(top, "; "))
				} else {
					obj, _ := doc.(map[string]any)
					caps := 0
					if capsObj, ok := obj["capabilities"].(map[string]any); ok {
						caps = len(capsObj)
					}
					summary := mcp.AttestationStatus(obj)
					detail := fmt.Sprintf("%d capabilities, attestation: %d attested / %d unverified / %d unattested",
						caps, summary.Counts["attested"],
						summary.Counts["unverified"], summary.Counts["unattested"])
					var risky []string
					for name, status := range summary.Servers {
						if status == "unverified" || status == "unattested" {
							risky = append(risky, name)
						}
					}
					sort.Strings(risky)
					if len(risky) > 0 {
						top := risky
						if len(top) > 5 {
							top = top[:5]
						}
						detail += fmt.Sprintf(" (unprovenanced: %s)", strings.Join(top, ", "))
						add("project-mcp-registry", "warn", detail)
					} else {
						add("project-mcp-registry", "pass", detail)
					}
				}
			}
		} else {
			add("project-mcp-registry", "warn", "none configured")
		}
	} else {
		if info, err := os.Stat(shiploomDir); err == nil && info.IsDir() {
			add("project", "warn", ".shiploom/ holds an install, not a project (run shiploom init)")
		} else {
			add("project", "warn", "no ./.shiploom/ (run shiploom init)")
		}
	}

	// --- toolchain (informational) ---
	if _, err := exec.LookPath("git"); err == nil {
		add("toolchain-git", "pass", "on PATH")
	} else {
		add("toolchain-git", "warn", "not on PATH")
	}

	failures, warnings := 0, 0
	for _, c := range checks {
		switch c.Status {
		case "fail":
			failures++
		case "warn":
			warnings++
		}
	}
	return Report{Ok: failures == 0, Checks: checks, Failures: failures, Warnings: warnings}
}

func hasKeys(obj map[string]any, keys ...string) bool {
	for _, k := range keys {
		if _, ok := obj[k]; !ok {
			return false
		}
	}
	return true
}

// osErrText mirrors FileNotFoundError str() for missing files (same
// synthesis as run.LoadManifest); other OSError texts differ.
func osErrText(path string, err error) string {
	if os.IsNotExist(err) {
		return fmt.Sprintf("[Errno 2] No such file or directory: '%s'", path)
	}
	return err.Error()
}

// pythonVersion reports the operator python3's "X.Y.Z" (same binary the
// parity harness uses for the reference CLI).
func pythonVersion() (string, bool) {
	for _, binary := range []string{"python3", "python"} {
		path, err := exec.LookPath(binary)
		if err != nil {
			continue
		}
		out, err := exec.Command(path, "--version").Output()
		if err != nil {
			continue
		}
		text := strings.TrimSpace(string(out))
		if rest, ok := strings.CutPrefix(text, "Python "); ok {
			return strings.TrimSpace(rest), true
		}
		return text, true
	}
	return "", false
}
