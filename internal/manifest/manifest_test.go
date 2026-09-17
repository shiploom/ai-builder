package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenesisShape(t *testing.T) {
	g := Genesis("1.1.0", GreenfieldWorkflow, nil)
	if g["coreVersion"] != "1.1.0" || g["workflow"] != "greenfield-full-lite" {
		t.Fatalf("genesis fields wrong: %v", g)
	}
	if g["workflowVersion"] != WorkflowVersion {
		t.Fatal("workflow version pin missing")
	}
	if DefaultWorkflow(true) != GreenfieldWorkflow || DefaultWorkflow(false) != BrownfieldWorkflow {
		t.Fatal("default workflow selection wrong")
	}
}

func TestSaveLoadRoundTripByteExact(t *testing.T) {
	dir := t.TempDir()
	g := Genesis("1.1.0", GreenfieldWorkflow, nil)
	// Freeze the timestamp for determinism.
	g["initializedAt"] = "2026-09-17T10:00:00Z"
	path, err := Save(dir, g)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{
		`"limit": 25.0`, `"used": 0.0`, `"limit": 800000`, `"used": 0`,
		"\"checkpoints\": [],\n", "\"artifacts\": {},\n",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("save bytes diverge (want %q):\n%s", want, text)
		}
	}
	if !strings.HasSuffix(text, "}\n") || strings.HasPrefix(text, "\n") {
		t.Fatal("framing wrong: want doc + single trailing newline")
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded["coreVersion"] != "1.1.0" {
		t.Fatalf("reload wrong: %v", loaded["coreVersion"])
	}
}

func TestLoadErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir); !os.IsNotExist(err) {
		t.Fatalf("missing manifest must be IsNotExist, got %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".shiploom"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ManifestPath(dir), []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil || os.IsNotExist(err) {
		t.Fatalf("corrupt manifest must be a non-NotExist error, got %v", err)
	} else if !strings.Contains(err.Error(), "unreadable manifest") {
		t.Fatalf("wrong message: %s", err)
	}
}

func TestSha256File(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.bin")
	if err := os.WriteFile(path, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Sha256File(path)
	if err != nil {
		t.Fatal(err)
	}
	// echo -n abc | sha256sum
	if got != "sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("wrong digest: %s", got)
	}
}
