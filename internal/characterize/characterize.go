// Package characterize ports cli/characterize.py (stdlib only):
// behavior-snapshot capture/diff for brownfield work. Report-only by
// design: changed snapshots warn, tool errors fail.
package characterize

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/shiploom/ai-builder/internal/auditlog"
	"github.com/shiploom/ai-builder/internal/difflib"
	"github.com/shiploom/ai-builder/internal/execrun"
	"github.com/shiploom/ai-builder/internal/jsoncanon"
	"github.com/shiploom/ai-builder/internal/manifest"
)

// SnapshotDirname is the snapshot directory under .shiploom/.
const SnapshotDirname = "characterization"

// TailLimit mirrors characterize.TAIL_LIMIT (runes).
const TailLimit = 20000

// DiffContext mirrors DIFF_CONTEXT.
const DiffContext = 3

// DiffMaxLines mirrors DIFF_MAX_LINES.
const DiffMaxLines = 60

// DefaultTimeoutS mirrors characterize.DEFAULT_TIMEOUT_S.
const DefaultTimeoutS = 600

var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// SnapshotDir mirrors snapshot_dir().
func SnapshotDir(projectDir string) string {
	return filepath.Join(projectDir, ".shiploom", SnapshotDirname)
}

func checkName(name string) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("bad snapshot name %q (letters/digits/._-, max 64)", name)
	}
	return nil
}

// message parity note: Python formats %r of the name (single-quoted for
// these inputs); %q matches for names without quotes/backslashes, which
// the regex above guarantees. Identical output on all valid inputs.

// resolveCommand mirrors _resolve_command().
func resolveCommand(projectDir, command string, timeoutS float64) (string, float64, error) {
	if command != "" {
		return command, timeoutS, nil
	}
	configured := execrun.LoadGateConfig(projectDir)
	cmd, timeout, _, ok := execrun.ResolveTestCommand(projectDir)
	_ = configured
	if !ok {
		// Mirror the ValueError text, including the reason.
		_, _, reason, _ := execrun.ResolveTestCommand(projectDir)
		return "", 0, fmt.Errorf("no command: pass --command or configure gates.test (%s)", reason)
	}
	if timeoutS != 0 {
		return cmd, timeoutS, nil
	}
	return cmd, timeout, nil
}

// Capture mirrors capture(). Returns the entry.
func Capture(projectDir, name, command string, timeoutS float64, actor string) (map[string]any, error) {
	if err := checkName(name); err != nil {
		return nil, err
	}
	command, timeout, err := resolveCommand(projectDir, command, timeoutS)
	if err != nil {
		return nil, err
	}
	if timeout == 0 {
		timeout = DefaultTimeoutS
	}
	result := execrun.RunCommand(projectDir, command, timeout)
	out := result.Tail
	entry := map[string]any{
		"name": name, "command": command, "exit": int64(result.Exit),
		"outputSha": shaOf(out), "tail": tailRunes(out, TailLimit),
		"timeoutS": timeout, "capturedAt": manifest.Utcnow(), "actor": actor,
	}
	dest := SnapshotDir(projectDir)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, err
	}
	raw, err := jsoncanon.Marshal(entry)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dest, name+".json"), append(raw, '\n'), 0o644); err != nil {
		return nil, err
	}
	if _, err := auditlog.Append(projectDir, actor, "characterize.capture", name, "-"); err != nil {
		return nil, err
	}
	return entry, nil
}

// load mirrors _load().
func load(projectDir, name string) (map[string]any, error) {
	if err := checkName(name); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(SnapshotDir(projectDir), name+".json"))
	if err != nil {
		return nil, fmt.Errorf("no snapshot %q (capture first): %s", name, fsErrText(err))
	}
	doc, err := jsoncanon.Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("no snapshot %q (capture first): %s", name, err)
	}
	obj, ok := doc.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("corrupt snapshot %q (recapture)", name)
	}
	if _, ok := obj["outputSha"]; !ok {
		return nil, fmt.Errorf("corrupt snapshot %q (recapture)", name)
	}
	return obj, nil
}

// ListSnapshots mirrors list_snapshots().
func ListSnapshots(projectDir string) []string {
	dest := SnapshotDir(projectDir)
	items, err := os.ReadDir(dest)
	if err != nil {
		return []string{}
	}
	var out []string
	for _, item := range items {
		if item.IsDir() || filepath.Ext(item.Name()) != ".json" {
			continue
		}
		out = append(out, strings.TrimSuffix(item.Name(), ".json"))
	}
	sort.Strings(out)
	return out
}

// DiffResult mirrors diff()'s return mapping.
type DiffResult struct {
	Name          string
	Changed       bool
	ExitChanged   bool
	OutputChanged bool
	UnifiedDiff   []string
	Current       map[string]any
	Err           string
	HasError      bool
}

// Diff mirrors diff(). Changed behavior is data (warn), not failure.
func Diff(projectDir, name string, timeoutS float64) DiffResult {
	entry, err := load(projectDir, name)
	if err != nil {
		return DiffResult{Name: name, Err: err.Error(), HasError: true}
	}
	var timeout float64
	if timeoutS != 0 {
		timeout = timeoutS
	} else if t, ok := execrun.ToFloat(entry["timeoutS"]); ok && t != 0 {
		timeout = t
	} else {
		timeout = DefaultTimeoutS
	}
	command, _ := entry["command"].(string)
	result := execrun.RunCommand(projectDir, command, timeout)
	out := result.Tail
	exitChanged := !valuesEqual(result.Exit, entry["exit"])
	outputChanged := shaOf(out) != strField(entry, "outputSha")
	var unified []string
	if outputChanged {
		oldTail, _ := entry["tail"].(string)
		unified = unifiedDiff(splitLines(oldTail), splitLines(out))
		if len(unified) > DiffMaxLines {
			unified = unified[:DiffMaxLines]
		}
	}
	// audit best-effort (mirrors try/except pass).
	_, _ = auditlog.Append(projectDir, "system", "characterize.diff", name, "-")
	return DiffResult{Name: name, Changed: exitChanged || outputChanged,
		ExitChanged: exitChanged, OutputChanged: outputChanged, UnifiedDiff: unified,
		Current: map[string]any{"exit": int64(result.Exit), "outputSha": shaOf(out)}}
}

func strField(obj map[string]any, key string) string {
	s, _ := obj[key].(string)
	return s
}

func tailRunes(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[len(runes)-limit:])
}

func fsErrText(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

func unifiedDiff(oldLines, newLines []string) []string {
	return difflib.UnifiedDiff(oldLines, newLines)
}

func shaOf(text string) string {
	sum := sha256.Sum256([]byte(text))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// valuesEqual mirrors Python !=/== for exit-code comparison.
func valuesEqual(a, b any) bool {
	// Python True == 1 cross-type equality, checked before numerics
	// (numFloat rejects bools, mirroring isinstance int-not-bool).
	if ab, ok := a.(bool); ok {
		if bb, ok := b.(bool); ok {
			return ab == bb
		}
		if bf, ok := numFloat(b); ok {
			return (bf == 1 && ab) || (bf == 0 && !ab)
		}
		return false
	}
	if bb, ok := b.(bool); ok {
		if af, ok := numFloat(a); ok {
			return (af == 1 && bb) || (af == 0 && !bb)
		}
		return false
	}
	af, aok := numFloat(a)
	bf, bok := numFloat(b)
	if aok && bok {
		return af == bf
	}
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if as, ok := a.(string); ok {
		bs, ok := b.(string)
		return ok && as == bs
	}
	return false
}

func numFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case int64:
		return float64(t), true
	case int:
		return float64(t), true
	case float64:
		return t, true
	case json.Number:
		if f, err := strconv.ParseFloat(string(t), 64); err == nil {
			return f, true
		}
		return 0, false
	default:
		return 0, false
	}
}

func splitLines(text string) []string {
	if text == "" {
		return []string{}
	}
	// Mirror str.splitlines for \n texts (corpus uses \n).
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	return lines
}
