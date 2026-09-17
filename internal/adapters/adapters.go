// Package adapters ports cli/adapters.py (stdlib only): single-source
// core -> generated harness files. Generated files carry DO NOT EDIT
// headers and are never hand-edited.
package adapters

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shiploom/ai-builder/internal/jsoncanon"
	"github.com/shiploom/ai-builder/internal/validate"
)

// TemplateVars mirrors TEMPLATE_VARS (substitution order).
var TemplateVars = []string{"projectName", "coreVersion", "workflow"}

// coreVersion reads the live tool core version.
func coreVersion(toolRoot string) string {
	raw, err := os.ReadFile(filepath.Join(toolRoot, "core", "VERSION"))
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(raw))
}

// defaultWorkflow mirrors _default_workflow().
func defaultWorkflow(projectDir string) string {
	raw, err := os.ReadFile(filepath.Join(projectDir, ".shiploom", "config.json"))
	if err != nil {
		return "greenfield-full-lite"
	}
	doc, err := jsoncanon.Decode(raw)
	if err != nil {
		return "greenfield-full-lite"
	}
	obj, ok := doc.(map[string]any)
	if !ok {
		return "greenfield-full-lite"
	}
	workflow, _ := obj["workflow"].(string)
	if workflow == "" {
		return "greenfield-full-lite"
	}
	if i := strings.LastIndex(workflow, "/"); i >= 0 {
		return workflow[i+1:]
	}
	return workflow
}

// context mirrors _context().
func context(toolRoot, projectDir string) map[string]string {
	name := projectDir
	if resolved, err := filepath.EvalSymlinks(projectDir); err == nil {
		name = resolved
	}
	return map[string]string{
		"projectName": filepath.Base(filepath.Clean(name)),
		"coreVersion": coreVersion(toolRoot),
		"workflow":    defaultWorkflow(projectDir),
	}
}

// substitute mirrors _substitute().
func substitute(template string, ctx map[string]string) string {
	for _, key := range TemplateVars {
		template = strings.ReplaceAll(template, "{{"+key+"}}", ctx[key])
	}
	return template
}

// skillWithHeader mirrors _skill_with_header().
func skillWithHeader(toolRoot, source string) (string, error) {
	raw, err := os.ReadFile(source)
	if err != nil {
		return "", err
	}
	text := string(raw)
	header := fmt.Sprintf("<!-- DO NOT EDIT — generated from Shiploom core %s; "+
		"edit core source, then re-run `shiploom adapters --generate`. -->\n",
		coreVersion(toolRoot))
	lines := strings.SplitAfter(text, "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if t := strings.TrimSpace(lines[i]); t == "---" || t == "..." {
				return strings.Join(lines[:i+1], "") + header + strings.Join(lines[i+1:], ""), nil
			}
		}
	}
	return header + text, nil
}

// Adapter mirrors one list_adapters() entry.
type Adapter struct {
	Name        string
	Version     string
	Description string
	Outputs     []string
}

// ToMap renders the adapter mapping.
func (a Adapter) ToMap() map[string]any {
	outputs := make([]any, 0, len(a.Outputs))
	for _, o := range a.Outputs {
		outputs = append(outputs, o)
	}
	return map[string]any{
		"adapter": a.Name, "version": a.Version,
		"description": a.Description, "outputs": outputs,
	}
}

// ListAdapters mirrors list_adapters().
func ListAdapters(toolRoot string) []Adapter {
	dir := filepath.Join(toolRoot, "adapters")
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, item := range items {
		if item.IsDir() {
			names = append(names, item.Name())
		}
	}
	sort.Strings(names)
	var out []Adapter
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(dir, name, "mapping.json"))
		if err != nil {
			continue
		}
		doc, err := jsoncanon.Decode(raw)
		if err != nil {
			continue
		}
		obj, ok := doc.(map[string]any)
		if !ok {
			continue
		}
		adapter, _ := obj["adapter"].(string)
		if adapter == "" {
			adapter = name
		}
		version, _ := obj["version"].(string)
		if version == "" {
			version = "?"
		}
		description, _ := obj["description"].(string)
		var outputs []string
		if list, ok := obj["outputs"].([]any); ok {
			for _, item := range list {
				if m, ok := item.(map[string]any); ok {
					target, _ := m["target"].(string)
					if target == "" {
						target = "?"
					}
					outputs = append(outputs, target)
				}
			}
		}
		if isTruthy(obj["skills"]) {
			outputs = append(outputs, "<skillsTarget>/*/SKILL.md")
		}
		out = append(out, Adapter{adapter, version, description, outputs})
	}
	if out == nil {
		out = []Adapter{}
	}
	return out
}

// Report mirrors generate()'s report mapping.
type Report struct {
	Adapter   string
	Created   []string
	Updated   []string
	Unchanged []string
}

// ToMap renders the report mapping.
func (r Report) ToMap() map[string]any {
	return map[string]any{
		"adapter": r.Adapter,
		"created": stringsToAny(r.Created), "updated": stringsToAny(r.Updated),
		"unchanged": stringsToAny(r.Unchanged),
	}
}

func stringsToAny(in []string) []any {
	out := make([]any, 0, len(in))
	for _, s := range in {
		out = append(out, s)
	}
	return out
}

// Generate mirrors generate(). Returns (report, errors); report is nil
// only for an unknown adapter.
func Generate(toolRoot, harness, projectDir string) (*Report, []string) {
	if harness == "all" {
		full := &Report{Adapter: "all"}
		var errors []string
		for _, adapter := range ListAdapters(toolRoot) {
			report, errs := Generate(toolRoot, adapter.Name, projectDir)
			if report != nil {
				full.Created = append(full.Created, report.Created...)
				full.Updated = append(full.Updated, report.Updated...)
				full.Unchanged = append(full.Unchanged, report.Unchanged...)
			}
			errors = append(errors, errs...)
		}
		return full, errors
	}
	src := filepath.Join(toolRoot, "adapters", harness)
	raw, err := os.ReadFile(filepath.Join(src, "mapping.json"))
	if err != nil {
		return nil, []string{fmt.Sprintf("unknown adapter %s", validate.PyRepr(harness))}
	}
	doc, err := jsoncanon.Decode(raw)
	if err != nil {
		return nil, []string{fmt.Sprintf("bad mapping.json for %s: %s",
			validate.PyRepr(harness), err)}
	}
	mapping, _ := doc.(map[string]any)
	if mapping == nil {
		return nil, []string{fmt.Sprintf("bad mapping.json for %s: %s",
			validate.PyRepr(harness), "not an object")}
	}
	ctx := context(toolRoot, projectDir)
	report := &Report{Adapter: harness}
	var errors []string
	write := func(rel, content string) {
		dest := filepath.Join(projectDir, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			errors = append(errors, fmt.Sprintf("cannot write %s: %s", rel, err))
			return
		}
		if prev, err := os.ReadFile(dest); err == nil && string(prev) == content {
			report.Unchanged = append(report.Unchanged, rel)
		} else {
			_, statErr := os.Stat(dest)
			created := os.IsNotExist(statErr)
			if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
				errors = append(errors, fmt.Sprintf("cannot write %s: %s", rel, err))
				return
			}
			if created {
				report.Created = append(report.Created, rel)
			} else {
				report.Updated = append(report.Updated, rel)
			}
		}
		if strings.Contains(rel, "/hooks/") &&
			(strings.HasSuffix(rel, ".py") || strings.HasSuffix(rel, ".sh")) {
			if info, err := os.Stat(dest); err == nil {
				_ = os.Chmod(dest, info.Mode()|0o111)
			}
		}
	}
	if outputs, ok := mapping["outputs"].([]any); ok {
		for _, item := range outputs {
			output, _ := item.(map[string]any)
			if output == nil {
				errors = append(errors, fmt.Sprintf("bad output mapping in %s adapter", harness))
				continue
			}
			template, _ := output["template"].(string)
			target, _ := output["target"].(string)
			tmplRaw, err := os.ReadFile(filepath.Join(src, template))
			if err != nil || target == "" {
				errors = append(errors, fmt.Sprintf("bad output mapping in %s adapter", harness))
				continue
			}
			write(target, substitute(string(tmplRaw), ctx))
		}
	}
	if isTruthy(mapping["skills"]) {
		skillsTarget, _ := mapping["skillsTarget"].(string)
		coreSkills := filepath.Join(toolRoot, "core", "skills")
		switch {
		case skillsTarget == "":
			errors = append(errors, fmt.Sprintf("skills adapter %s misses skillsTarget",
				validate.PyRepr(harness)))
		case !isDir(coreSkills):
			errors = append(errors, fmt.Sprintf("core skills missing: %s", coreSkills))
		default:
			items, _ := os.ReadDir(coreSkills)
			var names []string
			for _, item := range items {
				if item.IsDir() {
					names = append(names, item.Name())
				}
			}
			sort.Strings(names)
			for _, name := range names {
				source := filepath.Join(coreSkills, name, "SKILL.md")
				if info, err := os.Stat(source); err != nil || info.IsDir() {
					continue
				}
				content, err := skillWithHeader(toolRoot, source)
				if err != nil {
					errors = append(errors, fmt.Sprintf("cannot write %s skill: %s", name, err))
					continue
				}
				write(skillsTarget+"/"+name+"/SKILL.md", content)
			}
		}
	}
	sort.Strings(report.Created)
	sort.Strings(report.Updated)
	sort.Strings(report.Unchanged)
	return report, errors
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// isTruthy mirrors Python truthiness for the skills flag.
func isTruthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case string:
		return t != ""
	case []any:
		return len(t) > 0
	case map[string]any:
		return len(t) > 0
	case int64:
		return t != 0
	case float64:
		return t != 0
	default:
		return true
	}
}
