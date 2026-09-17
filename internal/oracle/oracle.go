// Package oracle ports cli/oracle.py (stdlib only): acceptance + oracle
// locking (verification layer 1) with tamper/redefinition detection.
package oracle

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/shiploom/ai-builder/internal/auditlog"
	"github.com/shiploom/ai-builder/internal/manifest"
	"github.com/shiploom/ai-builder/internal/validate"
)

// OracleDirname is the hidden vault directory.
const OracleDirname = ".oracle"

// LockAction is the audit action for locks.
const LockAction = "acceptance.lock"

// OracleDir mirrors oracle_dir().
func OracleDir(projectDir string) string {
	return filepath.Join(projectDir, ".shiploom", OracleDirname)
}

// DiscoverAcceptance mirrors discover_acceptance().
func DiscoverAcceptance(projectDir string) []string {
	var out []string
	for _, path := range validate.CollectFiles(projectDir) {
		if filepath.Ext(path) != ".json" {
			continue
		}
		rel, err := filepath.Rel(projectDir, path)
		if err != nil {
			continue
		}
		parts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
		hasAcceptance, hasOracle := false, false
		for _, part := range parts {
			if part == "acceptance" {
				hasAcceptance = true
			}
			if part == OracleDirname {
				hasOracle = true
			}
		}
		// Mirror: suffix .json + "acceptance" in parts + no .oracle part.
		// (collect_files already excludes EXCLUDE_DIRS incl. .oracle, but a
		// literal ".oracle" path segment check matches Python exactly.)
		if hasAcceptance && !hasOracle {
			out = append(out, path)
		}
	}
	sort.Strings(out)
	return out
}

// Criterion is one loaded acceptance criterion: file + object.
type Criterion struct {
	File string
	Obj  map[string]any
}

// LoadCriteria mirrors load_criteria(). Returns (criteria, errors).
func LoadCriteria(files []string) (map[string]Criterion, []validate.Entry) {
	criteria := map[string]Criterion{}
	var errors []validate.Entry
	for _, path := range files {
		doc, err := readJSONFile(path)
		if err != nil {
			errors = append(errors, validate.Entry{Path: path,
				Message: fmt.Sprintf("invalid JSON: %s", err)})
			continue
		}
		var objs []any
		if list, ok := doc.([]any); ok {
			objs = list
		} else {
			objs = []any{doc}
		}
		for _, item := range objs {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			id, _ := obj["id"].(string)
			if _, present := obj["id"]; !present {
				continue
			}
			if prev, dup := criteria[id]; dup {
				errors = append(errors, validate.Entry{Path: path,
					Message: fmt.Sprintf("duplicate acceptance id %s (also %s)",
						validate.PyRepr(id), prev.File)})
			} else {
				criteria[id] = Criterion{File: path, Obj: obj}
			}
		}
	}
	return criteria, errors
}

func readJSONFile(path string) (any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return validate.DecodeJSON(raw)
}

// vaultModeOK mirrors _vault_mode_ok().
func vaultModeOK(vault string) bool {
	if isWindows() {
		return true
	}
	info, err := os.Stat(vault)
	if err != nil {
		return false
	}
	return info.Mode().Perm() == 0o700
}

func isWindows() bool {
	return os.PathSeparator == '\\'
}

// gitStatus mirrors _git_status().
func gitStatus(path, projectDir string) string {
	git, err := exec.LookPath("git")
	if err != nil {
		return "unknown"
	}
	rel, err := filepath.Rel(projectDir, path)
	if err != nil {
		return "unknown"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ignored := exec.CommandContext(ctx, git, "-C", projectDir, "check-ignore", "-q", rel)
	if ignored.Run() == nil {
		return "ignored"
	}
	tracked := exec.CommandContext(ctx, git, "-C", projectDir, "ls-files", "--error-unmatch", rel)
	if tracked.Run() == nil {
		return "tracked"
	}
	// Divergence note: CPython returns "untracked" for any non-zero exit
	// including timeouts/crashes; Go matches that (no error channel here).
	return "untracked"
}

// resolveInside mirrors _resolve_inside(): vault-relative oracleRef
// confined to the vault; "" when it escapes. Symlinks resolve on the
// deepest existing ancestor (EvalSymlinks needs existence); missing
// targets fall back to a lexical check so the is_file gate below can
// report "missing" instead.
func resolveInside(vault, ref string) string {
	candidate := filepath.Join(vault, ref)
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		resolved = filepath.Clean(candidate)
	}
	vaultResolved, err := filepath.EvalSymlinks(vault)
	if err != nil {
		vaultResolved = filepath.Clean(vault)
	}
	rel, err := filepath.Rel(vaultResolved, resolved)
	if err != nil || rel == ".." || len(rel) >= 3 && rel[:3] == "../" {
		return ""
	}
	// Absolute refs escape by construction (Join discards nothing here,
	// so check explicitly like Python's resolve() semantics).
	if filepath.IsAbs(ref) {
		return ""
	}
	return candidate
}

// LockResult mirrors lock()'s (ok, errors, warnings, summary).
type LockResult struct {
	Ok       bool
	Errors   []validate.Entry
	Warnings []validate.Entry
	Summary  map[string]int
}

// Lock mirrors lock().
func Lock(schemas *validate.Schemas, projectDir, actor string) LockResult {
	res := LockResult{Summary: map[string]int{}}
	root := projectDir
	files := DiscoverAcceptance(root)
	if len(files) == 0 {
		res.Errors = append(res.Errors, validate.Entry{Path: root,
			Message: "no acceptance/*.json found"})
		return res
	}
	for _, path := range files {
		fileErrors, fileWarnings, _ := validate.ValidatePath(schemas, path, true)
		for _, e := range fileErrors {
			res.Errors = append(res.Errors, e)
		}
		for _, w := range fileWarnings {
			res.Warnings = append(res.Warnings, w)
		}
	}
	if len(res.Errors) > 0 {
		return res
	}
	criteria, dupErrors := LoadCriteria(files)
	for _, e := range dupErrors {
		res.Errors = append(res.Errors, e)
	}
	if len(res.Errors) > 0 {
		return res
	}
	vault := OracleDir(root)
	if info, err := os.Stat(vault); err != nil || !info.IsDir() {
		res.Errors = append(res.Errors, validate.Entry{Path: vault,
			Message: "missing oracle vault (run shiploom init)"})
		return res
	}
	if !vaultModeOK(vault) {
		res.Errors = append(res.Errors, validate.Entry{Path: vault,
			Message: fmt.Sprintf("vault mode is not 0700 (run chmod 700 %s)", vault)})
		return res
	}
	locked := map[string]any{}
	cids := make([]string, 0, len(criteria))
	for cid := range criteria {
		cids = append(cids, cid)
	}
	sort.Strings(cids)
	for _, cid := range cids {
		crit := criteria[cid]
		ref, _ := crit.Obj["oracleRef"].(string)
		target := resolveInside(vault, ref)
		if target == "" {
			res.Errors = append(res.Errors, validate.Entry{Path: crit.File,
				Message: fmt.Sprintf("%s: oracleRef escapes the vault: %s", cid, validate.PyRepr(ref))})
			continue
		}
		if info, err := os.Stat(target); err != nil || info.IsDir() {
			res.Errors = append(res.Errors, validate.Entry{Path: crit.File,
				Message: fmt.Sprintf("%s: missing oracle implementation: %s", cid, target)})
			continue
		}
		switch leak := gitStatus(target, root); leak {
		case "tracked":
			res.Errors = append(res.Errors, validate.Entry{Path: crit.File,
				Message: fmt.Sprintf("%s: oracle is tracked by git (leak): %s", cid, target)})
			continue
		case "untracked", "unknown":
			res.Warnings = append(res.Warnings, validate.Entry{Path: crit.File,
				Message: fmt.Sprintf("%s: oracle git-ignore unverified (%s)", cid, leak)})
		}
		relFile, err := filepath.Rel(root, crit.File)
		if err != nil {
			relFile = crit.File
		}
		fileHash, err := manifest.Sha256File(crit.File)
		if err != nil {
			res.Errors = append(res.Errors, validate.Entry{Path: crit.File,
				Message: fmt.Sprintf("unreadable: %s", err)})
			continue
		}
		relOracle, err := filepath.Rel(root, target)
		if err != nil {
			relOracle = target
		}
		oracleHash, err := manifest.Sha256File(target)
		if err != nil {
			res.Errors = append(res.Errors, validate.Entry{Path: crit.File,
				Message: fmt.Sprintf("unreadable: %s", err)})
			continue
		}
		locked[cid] = map[string]any{
			"file": relFile, "hash": fileHash,
			"oracle": relOracle, "oracleHash": oracleHash,
		}
	}
	if len(res.Errors) > 0 {
		return res
	}
	data, err := manifest.Load(root)
	if err != nil {
		res.Errors = append(res.Errors, validate.Entry{Path: root,
			Message: fmt.Sprintf("no manifest: %s (run shiploom init)", err)})
		return res
	}
	data["acceptance"] = map[string]any{"lockedAt": manifest.Utcnow(), "lockedBy": actor,
		"criteria": locked}
	if _, err := manifest.Save(root, data); err != nil {
		res.Errors = append(res.Errors, validate.Entry{Path: root,
			Message: fmt.Sprintf("cannot save manifest: %s", err)})
		return res
	}
	if _, err := auditlog.Append(root, actor, LockAction,
		fmt.Sprintf("acceptance (%d criteria)", len(locked)), "-"); err != nil {
		res.Errors = append(res.Errors, validate.Entry{Path: root,
			Message: fmt.Sprintf("cannot append audit: %s", err)})
		return res
	}
	res.Ok = true
	res.Summary = map[string]int{"criteria": len(locked), "files": len(files)}
	return res
}

// CheckResult mirrors check_lock()'s (ok, errors, warnings).
type CheckResult struct {
	Ok       bool
	Errors   []validate.Entry
	Warnings []validate.Entry
}

// CheckLock mirrors check_lock().
func CheckLock(projectDir string) CheckResult {
	res := CheckResult{}
	root := projectDir
	data, err := manifest.Load(root)
	if err != nil {
		res.Errors = append(res.Errors, validate.Entry{Path: root,
			Message: fmt.Sprintf("no manifest: %s", err)})
		return res
	}
	acceptance, _ := data["acceptance"].(map[string]any)
	var locked map[string]any
	if acceptance != nil {
		locked, _ = acceptance["criteria"].(map[string]any)
	}
	if len(locked) == 0 {
		res.Errors = append(res.Errors, validate.Entry{Path: root,
			Message: "no acceptance lock (run shiploom lock)"})
		return res
	}
	vault := OracleDir(root)
	if info, err := os.Stat(vault); err == nil && info.IsDir() && !vaultModeOK(vault) {
		res.Errors = append(res.Errors, validate.Entry{Path: vault,
			Message: "vault mode is not 0700"})
	}
	seen := map[string]bool{}
	cids := make([]string, 0, len(locked))
	for cid := range locked {
		cids = append(cids, cid)
	}
	sort.Strings(cids)
	for _, cid := range cids {
		entry, _ := locked[cid].(map[string]any)
		if entry == nil {
			continue
		}
		file, _ := entry["file"].(string)
		oracle, _ := entry["oracle"].(string)
		fpath := filepath.Join(root, file)
		opath := filepath.Join(root, oracle)
		seen[file] = true
		if info, err := os.Stat(fpath); err != nil || info.IsDir() {
			res.Errors = append(res.Errors, validate.Entry{Path: file,
				Message: fmt.Sprintf("%s: locked acceptance file missing", cid)})
		} else if hash, err := manifest.Sha256File(fpath); err != nil || hash != entry["hash"] {
			if err != nil {
				res.Errors = append(res.Errors, validate.Entry{Path: file,
					Message: fmt.Sprintf("unreadable: %s", err)})
			} else {
				res.Errors = append(res.Errors, validate.Entry{Path: file,
					Message: fmt.Sprintf("%s: acceptance redefined after lock", cid)})
			}
		}
		if info, err := os.Stat(opath); err != nil || info.IsDir() {
			res.Errors = append(res.Errors, validate.Entry{Path: oracle,
				Message: fmt.Sprintf("%s: locked oracle missing", cid)})
		} else if hash, err := manifest.Sha256File(opath); err != nil || hash != entry["oracleHash"] {
			if err != nil {
				res.Errors = append(res.Errors, validate.Entry{Path: oracle,
					Message: fmt.Sprintf("unreadable: %s", err)})
			} else {
				res.Errors = append(res.Errors, validate.Entry{Path: oracle,
					Message: fmt.Sprintf("%s: oracle changed after lock", cid)})
			}
		} else if gitStatus(opath, root) == "tracked" {
			res.Errors = append(res.Errors, validate.Entry{Path: oracle,
				Message: fmt.Sprintf("%s: oracle is tracked by git (leak)", cid)})
		}
	}
	for _, path := range DiscoverAcceptance(root) {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		if !seen[rel] {
			res.Errors = append(res.Errors, validate.Entry{Path: rel,
				Message: "acceptance file not covered by lock (re-run shiploom lock)"})
		}
	}
	res.Ok = len(res.Errors) == 0
	return res
}
