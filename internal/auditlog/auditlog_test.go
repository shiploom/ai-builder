package auditlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Fixed interop vectors captured from CPython (2026-09-17, ts pinned).
// The Go implementation must reproduce these hashes byte-for-byte:
// any drift breaks cross-implementation chains.
const (
	wantGenesisHash = "sha256:f9c388f86a46665a9a55fdc89d97a98f7851e4e3d2465d263b16b9f3033f2688"
	wantSecondHash  = "sha256:97c59633eb9240b858104614197a09460a45f78a0d95c90a21cb55422531d345"
	fixedTs         = "2026-09-17T10:00:00Z"
)

func fixedEntry(prev, actor, action, target, policy string) (map[string]any, error) {
	body := map[string]any{
		"ts": fixedTs, "actor": actor, "action": action,
		"target": target, "policy": policy, "prev": prev,
	}
	hash, err := entryHash(prev, body)
	if err != nil {
		return nil, err
	}
	body["hash"] = "sha256:" + hash
	return body, nil
}

func TestHashMatchesPython(t *testing.T) {
	genesis, err := fixedEntry(GenesisPrev, "system", "log.genesis", ".", "-")
	if err != nil {
		t.Fatal(err)
	}
	if genesis["hash"] != wantGenesisHash {
		t.Fatalf("genesis hash drift:\n got: %s\nwant: %s", genesis["hash"], wantGenesisHash)
	}
	second, err := fixedEntry(wantGenesisHash, "human:priya", "approve.prod", "DEP-001", "-")
	if err != nil {
		t.Fatal(err)
	}
	if second["hash"] != wantSecondHash {
		t.Fatalf("second hash drift:\n got: %s\nwant: %s", second["hash"], wantSecondHash)
	}
}

func TestInitAppendVerify(t *testing.T) {
	dir := t.TempDir()
	if _, err := InitLog(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := Append(dir, "human:priya", "approve.prod", "DEP-001", "-"); err != nil {
		t.Fatal(err)
	}
	entries, err := ReadAll(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	if ok, errs := Verify(dir); !ok {
		t.Fatalf("chain must verify: %v", errs)
	}
}

func TestVerifyDetectsTamper(t *testing.T) {
	dir := t.TempDir()
	if _, err := InitLog(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := Append(dir, "human:priya", "approve.prod", "DEP-001", "-"); err != nil {
		t.Fatal(err)
	}
	path := AuditPath(dir)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(raw), "approve.prod", "approve.everything", 1)
	if err := os.WriteFile(path, []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, errs := Verify(dir); ok || len(errs) == 0 {
		t.Fatalf("tamper must fail verification: ok=%v errs=%v", ok, errs)
	}
}

func TestVerifyEmptyAndMissing(t *testing.T) {
	dir := t.TempDir()
	if _, err := InitLog(dir); err != nil {
		t.Fatal(err)
	}
	path := AuditPath(dir)
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, errs := Verify(dir); ok || len(errs) == 0 {
		t.Fatalf("empty log must fail: %v %v", ok, errs)
	}
	if ok, errs := Verify(filepath.Join(dir, "missing")); ok || len(errs) == 0 {
		t.Fatalf("missing log must fail: %v %v", ok, errs)
	}
}

func TestExportMdShape(t *testing.T) {
	dir := t.TempDir()
	if _, err := InitLog(dir); err != nil {
		t.Fatal(err)
	}
	md, err := ExportMd(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(md, "# Audit log (1 entries)\n\n| ts | actor | action | target | hash |") {
		t.Fatalf("header wrong:\n%s", md)
	}
	if !strings.Contains(md, "| system | log.genesis | . | `…") {
		t.Fatalf("row wrong:\n%s", md)
	}
}
