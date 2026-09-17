// Package policy ports cli/policy.py (stdlib only): JSON-policy packs and
// hook matching with deny > require-approval > allow precedence.
package policy

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/shiploom/ai-builder/internal/jsoncanon"
)

// Decisions mirrors Python DECISIONS.
var Decisions = []string{"allow", "deny", "require-approval"}

// DefaultPackPath resolves the tool's default pack relative to CWD
// (repo-root runs). Installed layouts resolve via SHIPLOOM_CORE_DIR.
func DefaultPackPath() string {
	if dir := os.Getenv("SHIPLOOM_CORE_DIR"); dir != "" {
		return filepath.Join(dir, "core", "policies", "default.json")
	}
	return filepath.Join("core", "policies", "default.json")
}

// DefaultHooksPath resolves the tool's hooks registry.
func DefaultHooksPath() string {
	if dir := os.Getenv("SHIPLOOM_CORE_DIR"); dir != "" {
		return filepath.Join(dir, "core", "hooks", "registry.json")
	}
	return filepath.Join("core", "hooks", "registry.json")
}

// LoadPack mirrors load_pack().
func LoadPack(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unreadable policy pack %s: %s", path, fsErrText(err))
	}
	doc, err := jsoncanon.Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("unreadable policy pack %s: %s", path, err)
	}
	obj, ok := doc.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("policy pack %s misses rules[]", path)
	}
	if _, ok := obj["rules"].([]any); !ok {
		return nil, fmt.Errorf("policy pack %s misses rules[]", path)
	}
	return obj, nil
}

// fnmatchMatch mirrors fnmatch.fnmatchcase (full-string match, case
// sensitive). Non-string patterns never match; Python would crash on
// malformed packs instead (robustness divergence, documented).
func fnmatchMatch(name, pattern string) bool {
	rx, err := fnmatchRegexp(pattern)
	if err != nil {
		return false
	}
	return rx.MatchString(name)
}

func fnmatchTranslate(pattern string) string {
	var b strings.Builder
	runes := []rune(pattern)
	n := len(runes)
	i := 0
	for i < n {
		c := runes[i]
		i++
		switch c {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '[':
			j := i
			if j < n && runes[j] == '!' {
				j++
			}
			if j < n && runes[j] == ']' {
				j++
			}
			for j < n && runes[j] != ']' {
				j++
			}
			if j >= n {
				b.WriteString("\\[")
			} else {
				stuff := string(runes[i:j])
				stuff = strings.ReplaceAll(stuff, "\\", "\\\\")
				i = j + 1
				if strings.HasPrefix(stuff, "!") {
					stuff = "^" + stuff[1:]
				} else if strings.HasPrefix(stuff, "^") {
					stuff = "\\" + stuff
				}
				b.WriteString("[")
				b.WriteString(stuff)
				b.WriteString("]")
			}
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	return "(?s:" + b.String() + ")\\z"
}

func fnmatchRegexp(pattern string) (*regexp.Regexp, error) {
	return regexp.Compile(fnmatchTranslate(pattern))
}

func ruleMatches(rule map[string]any, action, resource string, context map[string]string) bool {
	actionPats, _ := rule["actions"].([]any)
	resPats, _ := rule["resources"].([]any)
	// Mirror Python: a *string* actions value iterates characters.
	// (Malformed packs only; schemas require arrays.)
	actionStrs := strList(actionPats, rule["actions"])
	resStrs := strList(resPats, rule["resources"])
	hit := false
	for _, pattern := range actionStrs {
		if fnmatchMatch(action, pattern) {
			hit = true
			break
		}
	}
	if !hit {
		return false
	}
	hit = false
	for _, pattern := range resStrs {
		if fnmatchMatch(resource, pattern) {
			hit = true
			break
		}
	}
	if !hit {
		return false
	}
	cond, _ := rule["condition"].(map[string]any)
	if cond == nil {
		if _, present := rule["condition"]; present {
			return false // non-dict condition never matches
		}
		return true
	}
	for key, want := range cond {
		var got string
		var present bool
		switch key {
		case "action":
			got, present = action, true
		case "resource":
			got, present = resource, true
		default:
			got, present = context[key]
		}
		if !present || !pyScalarEqual(got, want) {
			return false
		}
	}
	return true
}

func strList(list []any, raw any) []string {
	if list != nil {
		var out []string
		for _, item := range list {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	// Mirror Python iterating a bare string's characters.
	if s, ok := raw.(string); ok {
		return strings.Split(s, "")
	}
	return nil
}

// pyScalarEqual mirrors Python == for condition values. Scope values are
// always strings here, so only string wants can match (Python "x" == 1,
// True, None, or [] is always False).
func pyScalarEqual(got string, want any) bool {
	wantStr, ok := want.(string)
	return ok && got == wantStr
}

// Evaluate mirrors evaluate(): (decision, ruleID or "", message).
func Evaluate(pack map[string]any, action, resource string, context map[string]string) (string, string, string, error) {
	rulesRaw, ok := pack["rules"].([]any)
	if !ok {
		return "", "", "", fmt.Errorf("policy pack misses rules[]")
	}
	type hit struct {
		index int
		rule  map[string]any
	}
	var hits []hit
	for i, item := range rulesRaw {
		rule, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if ruleMatches(rule, action, resource, context) {
			hits = append(hits, hit{i, rule})
		}
	}
	rankOf := func(rule map[string]any) (int, int) {
		effect, _ := rule["effect"].(string)
		rank := 3
		switch effect {
		case "deny":
			rank = 0
		case "require-approval":
			rank = 1
		case "allow":
			rank = 2
		}
		cond := 1
		if _, ok := rule["condition"]; ok {
			cond = 0
		}
		return rank, cond
	}
	sort.SliceStable(hits, func(i, j int) bool {
		ri, ci := rankOf(hits[i].rule)
		rj, cj := rankOf(hits[j].rule)
		if ri != rj {
			return ri < rj
		}
		return ci < cj
	})
	for _, h := range hits {
		if effect, ok := h.rule["effect"].(string); ok {
			for _, d := range Decisions {
				if effect == d {
					msg, _ := h.rule["message"].(string)
					id, _ := h.rule["id"].(string)
					return effect, id, msg, nil
				}
			}
		}
	}
	def, _ := pack["defaultEffect"].(string)
	known := false
	for _, d := range Decisions {
		if def == d {
			known = true
		}
	}
	if !known {
		def = "deny"
	}
	return def, "", "no rule matched (default " + def + ")", nil
}

// EvaluateFile loads a pack file then evaluates it.
func EvaluateFile(path, action, resource string, context map[string]string) (string, string, string, error) {
	pack, err := LoadPack(path)
	if err != nil {
		return "", "", "", err
	}
	return Evaluate(pack, action, resource, context)
}

// LoadHooks mirrors load_hooks().
func LoadHooks(path string) ([]any, error) {
	if path == "" {
		path = DefaultHooksPath()
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unreadable hooks registry %s: %s", path, fsErrText(err))
	}
	doc, err := jsoncanon.Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("unreadable hooks registry %s: %s", path, err)
	}
	list, ok := doc.([]any)
	if !ok {
		return nil, fmt.Errorf("hooks registry %s must be a list", path)
	}
	return list, nil
}

// MatchHooks mirrors match_hooks().
func MatchHooks(hooks []any, event, target string) []map[string]any {
	var matched []map[string]any
	for _, item := range hooks {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		ev, _ := entry["event"].(string)
		if ev != event {
			continue
		}
		matcher, _ := entry["matcher"].(string)
		if fnmatchMatch(target, matcher) {
			matched = append(matched, entry)
		}
	}
	return matched
}

// CheckStep mirrors check_step(): project-relative pack path (or "" for
// the default pack), action/resource defaulting, fixed context keys.
// Returns (decision, ruleID, message, error).
func CheckStep(projectDir, packRef, workflow, stepID, actor, action, resource string) (string, string, string, error) {
	var packPath string
	if packRef != "" {
		packPath = packRef
		if !filepath.IsAbs(packPath) {
			packPath = filepath.Join(projectDir, packRef)
		}
		if info, err := os.Stat(packPath); err != nil || info.IsDir() {
			return "", "", "", fmt.Errorf("policy pack not found: %s", packRef)
		}
	} else {
		packPath = DefaultPackPath()
	}
	if action == "" {
		action = "workflow.step"
	}
	if resource == "" {
		resource = workflow + ":" + stepID
	}
	// Mirror Python falsy handling: explicit empty strings already defaulted.
	return EvaluateFile(packPath, action, resource, map[string]string{
		"actor": actor, "workflow": workflow, "step": stepID,
	})
}

func fsErrText(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
