// Package manifest ports cli/manifest.py (stdlib only).
package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/shiploom/ai-builder/internal/jsoncanon"
)

const (
	// ManifestName is the orchestrator-owned state file.
	ManifestName = "manifest.json"
	// WorkflowVersion stamps genesis manifests.
	WorkflowVersion = "1.0.0"
	// GreenfieldWorkflow is the default greenfield flow.
	GreenfieldWorkflow = "greenfield-full-lite"
	// BrownfieldWorkflow is the default brownfield flow.
	BrownfieldWorkflow = "brownfield-fix"
)

// Utcnow mirrors Python utcnow(): "2026-09-17T10:00:00Z".
func Utcnow() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

// DefaultWorkflow mirrors default_workflow().
func DefaultWorkflow(green bool) string {
	if green {
		return GreenfieldWorkflow
	}
	return BrownfieldWorkflow
}

// DefaultBudgets mirrors default_budgets(). Floats use literals so the
// canonical writer emits "25.0"/"0.0" exactly like CPython.
func DefaultBudgets() map[string]any {
	return map[string]any{
		"tokens":   map[string]any{"limit": int64(800000), "used": int64(0)},
		"spendUSD": map[string]any{"limit": json.Number("25.0"), "used": json.Number("0.0")},
	}
}

// Genesis builds a genesis manifest dict (not yet saved).
func Genesis(coreVersion, workflow string, budgets map[string]any) map[string]any {
	if budgets == nil {
		budgets = DefaultBudgets()
	}
	return map[string]any{
		"coreVersion":     coreVersion,
		"workflow":        workflow,
		"workflowVersion": WorkflowVersion,
		"artifacts":       map[string]any{},
		"gates":           map[string]any{},
		"budgets":         budgets,
		"retries":         map[string]any{},
		"checkpoints":     []any{},
		"initializedAt":   Utcnow(),
	}
}

// ManifestPath mirrors manifest_path().
func ManifestPath(projectDir string) string {
	return filepath.Join(projectDir, ".shiploom", ManifestName)
}

// Load mirrors load(): os.ErrNotExist when missing, wrapped message when corrupt.
func Load(projectDir string) (map[string]any, error) {
	path := ManifestPath(projectDir)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, fmt.Errorf("unreadable manifest %s: %s", path, fsErrText(err))
	}
	doc, err := jsoncanon.Decode(raw)
	if err != nil {
		// Mirror "unreadable manifest %s: %s"; suffix is engine-specific.
		return nil, fmt.Errorf("unreadable manifest %s: %s", path, err)
	}
	obj, ok := doc.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unreadable manifest %s: not an object", path)
	}
	return obj, nil
}

// Save atomically writes the manifest (kill-safe): temp file + rename,
// best-effort temp cleanup on failure.
func Save(projectDir string, data map[string]any) (string, error) {
	path := ManifestPath(projectDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	out, err := jsoncanon.Marshal(data)
	if err != nil {
		return "", err
	}
	out = append(out, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".manifest-*.tmp")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return "", err
	}
	// os.Rename cannot replace an existing file on Windows, so remove
	// first (POSIX keeps atomic replace; the crash window on Windows is
	// accepted — manifests are rewritten on every run).
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		os.Remove(tmpName)
		return "", err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return "", err
	}
	return path, nil
}

// Sha256File mirrors sha256_file().
func Sha256File(path string) (string, error) {
	fh, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer fh.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, fh); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(digest.Sum(nil)), nil
}

func fsErrText(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
