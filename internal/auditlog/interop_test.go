package auditlog

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestAuditChainInterop proves byte-level audit compatibility across
// implementations: entries written by Python verify under Go and vice
// versa, in both orders. Skipped where python3 is unavailable (CI
// provides it via setup-python).
func TestAuditChainInterop(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not on PATH")
	}
	repo := repoRoot(t)
	cli := filepath.Join(repo, "cli", "shiploom.py")

	runPy := func(dir string, code string) {
		t.Helper()
		cmd := exec.Command(py, "-c", code)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "PYTHONPATH="+repo)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("python failed: %s\n%s", err, out)
		}
	}
	runAudit := func(dir string) {
		t.Helper()
		cmd := exec.Command(py, cli, "audit")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("python audit failed: %s\n%s", err, out)
		}
	}

	// Python creates, Go extends + verifies, Python re-verifies.
	dir := t.TempDir()
	runPy(dir, "from cli.auditlog import init_log, append;"+
		"init_log('.'); append('.', actor='human:priya', action='approve.prod', target='DEP-001')")
	entry, err := Append(dir, "system", "rotate.keys", "token-id", "-")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := entry["hash"].(string); !ok {
		t.Fatal("append must return the entry")
	}
	if ok, errs := Verify(dir); !ok {
		t.Fatalf("go must verify python-started chain: %v", errs)
	}
	runAudit(dir)

	// Go creates, Python extends + verifies, Go re-verifies.
	dir2 := t.TempDir()
	if _, err := InitLog(dir2); err != nil {
		t.Fatal(err)
	}
	if _, err := Append(dir2, "human:sam", "approve.prod", "DEP-009", "-"); err != nil {
		t.Fatal(err)
	}
	runAudit(dir2)
	runPy(dir2, "from cli.auditlog import append, verify;"+
		"append('.', actor='system', action='x', target='.');"+
		"ok, errs = verify('.'); assert ok, errs")
	if ok, errs := Verify(dir2); !ok {
		t.Fatalf("go must verify python-extended chain: %v", errs)
	}

	// Tamper is visible to both implementations.
	raw, err := os.ReadFile(filepath.Join(dir2, ".shiploom", "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(raw), "approve.prod", "approve.everything", 1)
	if err := os.WriteFile(filepath.Join(dir2, ".shiploom", "audit.jsonl"),
		[]byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, _ := Verify(dir2); ok {
		t.Fatal("go must reject tampered chain")
	}
	cmd := exec.Command(py, cli, "audit")
	cmd.Dir = dir2
	if err := cmd.Run(); err == nil {
		t.Fatal("python must reject tampered chain")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "core", "VERSION")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("repo root not found")
		}
		dir = parent
	}
}
