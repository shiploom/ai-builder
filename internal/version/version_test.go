package version

// Static half of scripts/version-check.sh: every version file tracks
// core/VERSION. (The live --version halves are covered by the release
// job's version-check step.)
//
// Since v1.3.0 the Python implementation is gone, so pyproject.toml is
// gone with it; the gate is core/VERSION == wrappers/npx/package.json.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

func TestVersionFilesAgree(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "core", "VERSION"))
	if err != nil {
		t.Fatalf("read core/VERSION: %s", err)
	}
	core := strings.TrimSpace(string(raw))
	if core == "" {
		t.Fatal("empty core/VERSION")
	}
	pkgRaw, err := os.ReadFile(filepath.Join(root, "wrappers", "npx", "package.json"))
	if err != nil {
		t.Fatalf("read wrappers/npx/package.json: %s", err)
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(pkgRaw, &pkg); err != nil {
		t.Fatalf("parse wrappers/npx/package.json: %s", err)
	}
	if pkg.Version != core {
		t.Fatalf("npx %s != core %s", pkg.Version, core)
	}
}
