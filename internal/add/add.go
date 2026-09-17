// Package add ports cli/add.py (stdlib only): content packs installed
// into the project overlay with strict validation, rollback on failure,
// and unsigned provenance.
package add

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/shiploom/ai-builder/internal/jsoncanon"
	"github.com/shiploom/ai-builder/internal/manifest"
	"github.com/shiploom/ai-builder/internal/validate"
)

// Kind describes one overlay content kind.
type Kind struct {
	Overlay string
	Style   string // "dir" or "file"
	Marker  string // dir marker file
	Suffix  string // file suffix
}

// Kinds mirrors KINDS.
var Kinds = map[string]Kind{
	"skill":    {Overlay: "skills", Style: "dir", Marker: "SKILL.md"},
	"workflow": {Overlay: "workflows", Style: "file", Suffix: ".md"},
	"hook":     {Overlay: "hooks", Style: "file", Suffix: ".json"},
	"policy":   {Overlay: "policies", Style: "file", Suffix: ".json"},
	"adapter":  {Overlay: "adapters", Style: "dir", Marker: "mapping.json"},
}

// ProvenanceFile mirrors PROVENANCE_FILE.
const ProvenanceFile = "_provenance.json"

// SortedKinds lists kind names in argparse-choices order.
func SortedKinds() []string {
	out := make([]string, 0, len(Kinds))
	for k := range Kinds {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// isURL mirrors _is_url().
func isURL(ref string) bool {
	return strings.Contains(ref, "://") || strings.HasPrefix(ref, "git@") ||
		strings.HasSuffix(ref, ".git")
}

// fetch mirrors _fetch() for local paths (git URLs use cloneSource).
func fetch(source, workdir string) (string, error) {
	if _, err := os.Stat(source); err != nil {
		return "", fmt.Errorf("source not found: %s", source)
	}
	return source, nil
}

// cloneSource clones a git URL at a tag into workdir/source.
func cloneSource(source, tag, workdir string) (string, error) {
	git, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git not on PATH (needed for URL sources)")
	}
	dest := filepath.Join(workdir, "source")
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, git, "clone", "--quiet", "--depth", "1",
		"--branch", tag, source, dest)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("git clone failed: %s", ctx.Err())
		}
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("git clone failed: %s", msg)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git clone failed: exit %d", exitErr.ExitCode())
		}
		return "", fmt.Errorf("git clone failed: %s", err)
	}
	return dest, nil
}

// locate mirrors _locate().
func locate(kind, name, payload string) (string, error) {
	spec := Kinds[kind]
	base := payload
	if spec.Style == "dir" {
		if info, err := os.Stat(filepath.Join(base, spec.Marker)); err == nil && !info.IsDir() {
			return base, nil
		}
		if info, err := os.Stat(filepath.Join(base, name, spec.Marker)); err == nil && !info.IsDir() {
			return filepath.Join(base, name), nil
		}
		return "", fmt.Errorf("no %s found under %s (want %s or %s/%s)",
			spec.Marker, payload, spec.Marker, name, spec.Marker)
	}
	if info, err := os.Stat(base); err == nil && !info.IsDir() {
		return base, nil
	}
	candidate := filepath.Join(base, name+spec.Suffix)
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate, nil
	}
	return "", fmt.Errorf("no %s file found under %s (want %s%s)",
		kind, payload, name, spec.Suffix)
}

// destFor mirrors _dest_for().
func destFor(kind, name, projectDir string) string {
	spec := Kinds[kind]
	if spec.Style == "dir" {
		return filepath.Join(projectDir, ".shiploom", spec.Overlay, name)
	}
	return filepath.Join(projectDir, ".shiploom", spec.Overlay, name+spec.Suffix)
}

// recordProvenance mirrors _record_provenance().
func recordProvenance(projectDir, kind, name, source, tag string) error {
	spec := Kinds[kind]
	overlay := filepath.Join(projectDir, ".shiploom", spec.Overlay)
	if err := os.MkdirAll(overlay, 0o755); err != nil {
		return err
	}
	recordPath := filepath.Join(overlay, ProvenanceFile)
	records := map[string]any{}
	if raw, err := os.ReadFile(recordPath); err == nil {
		if doc, err := jsoncanon.Decode(raw); err == nil {
			if obj, ok := doc.(map[string]any); ok {
				records = obj
			}
		}
	}
	records[name] = map[string]any{"kind": kind, "source": source, "tag": tagAny(tag),
		"signed": false, "installedAt": manifest.Utcnow()}
	raw, err := jsoncanon.Marshal(records)
	if err != nil {
		return err
	}
	return os.WriteFile(recordPath, append(raw, '\n'), 0o644)
}

// copyPayload mirrors the copytree/copy2 install step.
func copyPayload(located, dest string) error {
	info, err := os.Stat(located)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return copyFile(located, dest, info.Mode())
	}
	return copyDir(located, dest)
}

func copyFile(src, dest string, mode os.FileMode) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dest, raw, mode.Perm())
}

func copyDir(src, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == "__pycache__" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dest, rel), 0o755)
		}
		if strings.Contains(rel, "__pycache__") {
			return nil
		}
		return copyFile(path, filepath.Join(dest, rel), info.Mode())
	})
}

func removeDest(dest string) {
	if info, err := os.Lstat(dest); err == nil {
		if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			os.RemoveAll(dest)
		} else {
			os.Remove(dest)
		}
	}
}

// tagAny mirrors argparse --tag default None (JSON null when absent).
func tagAny(tag string) any {
	if tag == "" {
		return nil
	}
	return tag
}

// Install mirrors install(). Returns (destRel, warnings).
func Install(schemas *validate.Schemas, kind, name, source, projectDir, tag string, force bool) (string, []string, error) {
	spec, ok := Kinds[kind]
	if !ok {
		return "", nil, fmt.Errorf("unknown kind %s (choose %s)",
			validate.PyRepr(kind), strings.Join(SortedKinds(), "|"))
	}
	if name == "" || strings.Contains(name, "/") || name == "." || name == ".." {
		return "", nil, fmt.Errorf("bad name %s", validate.PyRepr(name))
	}
	_ = spec
	dest := destFor(kind, name, projectDir)
	if _, err := os.Lstat(dest); err == nil && !force {
		return "", nil, fmt.Errorf("%s exists (use --force to overwrite)", dest)
	}
	workdir, err := os.MkdirTemp("", "shiploom-add-")
	if err != nil {
		return "", nil, err
	}
	defer os.RemoveAll(workdir)
	var payload string
	if isURL(source) {
		if tag == "" {
			return "", nil, fmt.Errorf("git sources require --tag (marketplace pins semver tags)")
		}
		payload, err = cloneSource(source, tag, workdir)
		if err != nil {
			return "", nil, err
		}
	} else {
		payload, err = fetch(source, workdir)
		if err != nil {
			return "", nil, err
		}
	}
	located, err := locate(kind, name, payload)
	if err != nil {
		return "", nil, err
	}
	if _, err := os.Lstat(dest); err == nil {
		removeDest(dest)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", nil, err
	}
	if err := copyPayload(located, dest); err != nil {
		return "", nil, err
	}
	verr, _, _ := validate.ValidatePath(schemas, dest, true)
	var kept []validate.Entry
	for _, e := range verr {
		if e.Rule != "links.dangling" {
			kept = append(kept, e)
		}
	}
	if len(kept) > 0 {
		removeDest(dest)
		return "", nil, fmt.Errorf("installed %s fails validation: %s", name, kept[0].Message)
	}
	if err := recordProvenance(projectDir, kind, name, source, tag); err != nil {
		return "", nil, err
	}
	warnings := []string{"unsigned provenance (threshold-sigs post-MVP)"}
	rel, err := filepath.Rel(projectDir, dest)
	if err != nil {
		rel = dest
	}
	return rel, warnings, nil
}
