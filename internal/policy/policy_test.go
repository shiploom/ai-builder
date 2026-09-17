package policy

import (
	"os"
	"path/filepath"
	"testing"
)

// fnmatchTruth mirrors CPython fnmatch.fnmatchcase, generated 2026-09-17
// (pattern -> sorted matching names). Any change here means drift.
var fnmatchTruth = map[string][]string{
	"deploy.*":        {"deploy.prod", "deploy.staging9"},
	"*":               {"a.json", "a]b]c", "abe", "ace", "axe", "cap.1", "cap.12", "db.x-prod", "deploy", "deploy.prod", "deploy.staging9", "infra", "infra/vpc", "mcp__fs__write_file", "merge.protected", "push.main", "req-001", "req-01", "x["},
	"merge.protected": {"merge.protected"},
	"push.main":       {"push.main"},
	"db.*-prod":       {"db.x-prod"},
	"cap.?":           {"cap.1"},
	"a[bcd]e":         {"abe", "ace"},
	"a[!b]c":          {},
	"x[":              {"x["},
	"a[]b]c":          {},
	"*.json":          {"a.json"},
	"req-???":         {"req-001"},
	"mcp__*__write.*": {},
	"infra/*":         {"infra/vpc"},
}

var fnmatchNames = []string{"deploy.prod", "deploy.staging9", "deploy", "merge.protected",
	"push.main", "db.x-prod", "cap.1", "cap.12", "abe", "ace", "axe", "x[", "a]b]c",
	"a.json", "req-001", "req-01", "mcp__fs__write_file", "infra", "infra/vpc"}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestFnmatchMatchesCPython(t *testing.T) {
	for pattern, want := range fnmatchTruth {
		var got []string
		for _, name := range fnmatchNames {
			if fnmatchMatch(name, pattern) {
				got = append(got, name)
			}
		}
		// Normalize nil vs empty for comparison.
		if got == nil {
			got = []string{}
		}
		if want == nil {
			want = []string{}
		}
		// Sort both (map order is random; truth lists are sorted).
		for i := 1; i < len(got); i++ {
			for j := i; j > 0 && got[j] < got[j-1]; j-- {
				got[j], got[j-1] = got[j-1], got[j]
			}
		}
		if !sameStrings(got, want) {
			t.Errorf("pattern %q: got %v want %v", pattern, got, want)
		}
	}
}

func testPack() map[string]any {
	return map[string]any{
		"policyId": "t", "version": "1.0.0", "defaultEffect": "deny",
		"rules": []any{
			map[string]any{"id": "deny-prod-migrate", "effect": "deny",
				"actions": []any{"db.migrate-prod"}, "resources": []any{"*"}},
			map[string]any{"id": "approve-deploy", "effect": "require-approval",
				"actions": []any{"deploy.*"}, "resources": []any{"*"}},
			map[string]any{"id": "approve-human-prod", "effect": "require-approval",
				"actions": []any{"deploy.prod"}, "resources": []any{"prod"},
				"condition": map[string]any{"actor": "agent"}},
			map[string]any{"id": "allow-reads", "effect": "allow",
				"actions": []any{"repo.*"}, "resources": []any{"*"}},
		},
	}
}

func TestEvaluatePrecedence(t *testing.T) {
	pack := testPack()
	decision, rule, _ := mustEval(t, pack, "db.migrate-prod", "prod", nil)
	if decision != "deny" || rule != "deny-prod-migrate" {
		t.Fatalf("deny must win: %s %s", decision, rule)
	}
	// Conditional exception beats the general glob at the same effect.
	decision, rule, _ = mustEval(t, pack, "deploy.prod", "prod", map[string]string{"actor": "agent"})
	if decision != "require-approval" || rule != "approve-human-prod" {
		t.Fatalf("conditional must win ties: %s %s", decision, rule)
	}
	decision, rule, _ = mustEval(t, pack, "deploy.prod", "prod", map[string]string{"actor": "human"})
	if decision != "require-approval" || rule != "approve-deploy" {
		t.Fatalf("unconditional glob applies: %s %s", decision, rule)
	}
	decision, _, _ = mustEval(t, pack, "repo.read", "x", nil)
	if decision != "allow" {
		t.Fatalf("allow failed: %s", decision)
	}
	decision, rule, msg := mustEval(t, pack, "unknown.thing", "x", nil)
	if decision != "deny" || rule != "" || msg == "" {
		t.Fatalf("default fallback wrong: %s %s %s", decision, rule, msg)
	}
}

func mustEval(t *testing.T, pack map[string]any, action, resource string,
	context map[string]string) (string, string, string) {
	t.Helper()
	decision, rule, msg, err := Evaluate(pack, action, resource, context)
	if err != nil {
		t.Fatal(err)
	}
	return decision, rule, msg
}

func TestEvaluateBadPack(t *testing.T) {
	if _, _, _, err := Evaluate(map[string]any{}, "a", "r", nil); err == nil {
		t.Fatal("pack without rules must error")
	}
	if _, _, _, err := EvaluateFile("/nonexistent/pack.json", "a", "r", nil); err == nil {
		t.Fatal("missing file must error")
	}
}

func TestMatchHooks(t *testing.T) {
	hooks := []any{
		map[string]any{"event": "before_file_write", "matcher": ".shiploom/.oracle/*", "action": "deny"},
		map[string]any{"event": "before_merge", "matcher": "main", "action": "require-approval"},
		"junk",
	}
	hits := MatchHooks(hooks, "before_file_write", ".shiploom/.oracle/x.sh")
	if len(hits) != 1 {
		t.Fatalf("want 1 hit, got %v", hits)
	}
	if len(MatchHooks(hooks, "before_merge", "feature/x")) != 0 {
		t.Fatal("non-matching target must miss")
	}
	if len(MatchHooks(hooks, "nope", "*")) != 0 {
		t.Fatal("unknown event must miss")
	}
}

func TestLoadHooksErrors(t *testing.T) {
	if _, err := LoadHooks("/nonexistent/hooks.json"); err == nil {
		t.Fatal("missing registry must error")
	}
	path := filepath.Join(t.TempDir(), "hooks.json")
	if err := os.WriteFile(path, []byte(`{"not": "a list"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadHooks(path); err == nil {
		t.Fatal("non-list registry must error")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "core", "VERSION")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("repo root not found (core/VERSION missing)")
		}
		dir = parent
	}
}

func TestCheckStepAgainstRealDefaultPack(t *testing.T) {
	t.Setenv("SHIPLOOM_CORE_DIR", repoRoot(t))
	dir := t.TempDir()
	decision, rule, _, err := CheckStep(dir, "", "wf", "s", "human", "db.destroy", "prod")
	if err != nil {
		t.Fatal(err)
	}
	if decision != "deny" || rule != "deny-destructive" {
		t.Fatalf("default pack must deny: %s %s", decision, rule)
	}
	if _, _, _, err := CheckStep(dir, "policies/missing.json", "wf", "s",
		"human", "", ""); err == nil {
		t.Fatal("missing pack ref must error")
	}
}
