// Package status ports validators/status.py (stdlib only): read-only
// artifact states + trace summary. Budgets stay manifest-owned; the
// orchestrator overrides Payload.Budgets (see cmd_status in P4).
package status

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/shiploom/ai-builder/internal/jsoncanon"
	"github.com/shiploom/ai-builder/internal/trace"
	"github.com/shiploom/ai-builder/internal/validate"
	"github.com/shiploom/ai-builder/internal/version"
)

// UnavailableBudgets mirrors the PR3 stub status_of() always embeds.
func UnavailableBudgets() map[string]any {
	return map[string]any{
		"status": "unavailable",
		"reason": "manifest-owned (merged by `shiploom status` when" +
			" `./.shiploom/manifest.json` exists)",
	}
}

// Artifact mirrors one status artifact row (raw frontmatter values).
type Artifact struct {
	Path    string
	ID      any
	Kind    any
	Title   any
	Status  any
	Owner   any
	Version any
}

// TraceSummary mirrors payload["trace"].
type TraceSummary struct {
	Nodes  int
	Edges  int
	Errors []string
}

// Payload mirrors status_of() output (camelCase keys via ToMap).
// Budgets is any: the orchestrator replaces the stub with the manifest
// budgets verbatim (mirroring manifest.get("budgets", ...)).
type Payload struct {
	CoreVersion string
	Target      string
	Artifacts   []Artifact
	ByStatus    map[string]int
	Invalid     []validate.Entry
	Trace       TraceSummary
	Budgets     any
}

// StatusOf mirrors status_of(). Returns (payload, error-string or "").
func StatusOf(schemas *validate.Schemas, target string) (Payload, string) {
	payload := Payload{Budgets: UnavailableBudgets(), ByStatus: map[string]int{}}
	info, err := os.Stat(target)
	if err != nil {
		return payload, fmt.Sprintf("no such file or directory: %s", target)
	}
	base := target
	if !info.IsDir() {
		base = filepath.Dir(target)
	}
	for _, path := range validate.CollectFiles(target) {
		if validate.IsExcluded(path, base) || filepath.Ext(path) != ".md" {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			payload.Invalid = append(payload.Invalid, validate.Entry{Path: path,
				Message: fmt.Sprintf("unreadable: %s", fsErrText(err))})
			continue
		}
		if !utf8.Valid(raw) {
			// Robustness note: CPython read_text(strict) raises here;
			// recording INVALID instead of crashing (pathological only).
			payload.Invalid = append(payload.Invalid, validate.Entry{Path: path,
				Message: fmt.Sprintf("unreadable: invalid UTF-8 in %s", path)})
			continue
		}
		fm, _, perr := validate.ParseFrontmatter(string(raw))
		if fm == nil {
			if perr != "" {
				payload.Invalid = append(payload.Invalid, validate.Entry{Path: path, Message: perr})
			}
			continue
		}
		if _, hasID := fm["id"]; !hasID {
			continue
		}
		if _, hasKind := fm["kind"]; !hasKind {
			continue
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			rel = path
		}
		payload.Artifacts = append(payload.Artifacts, Artifact{
			Path: rel, ID: fm["id"], Kind: fm["kind"], Title: fm["title"],
			Status: fm["status"], Owner: fm["owner"], Version: fm["version"],
		})
	}
	sort.Slice(payload.Artifacts, func(i, j int) bool {
		ai := validate.PyStr(payload.Artifacts[i].ID)
		aj := validate.PyStr(payload.Artifacts[j].ID)
		if ai != aj {
			return ai < aj
		}
		return payload.Artifacts[i].Path < payload.Artifacts[j].Path
	})
	for _, art := range payload.Artifacts {
		payload.ByStatus[validate.PyStr(art.Status)]++
	}
	traceMap, traceErrors, _ := trace.BuildTrace(schemas, target, false)
	edges := 0
	for _, links := range traceMap {
		for _, targets := range links {
			edges += len(targets)
		}
	}
	payload.Trace.Nodes = len(traceMap)
	payload.Trace.Edges = edges
	for _, e := range traceErrors {
		payload.Trace.Errors = append(payload.Trace.Errors, e.Message)
	}
	if payload.Trace.Errors == nil {
		payload.Trace.Errors = []string{}
	}
	if payload.Artifacts == nil {
		payload.Artifacts = []Artifact{}
	}
	if payload.Invalid == nil {
		payload.Invalid = []validate.Entry{}
	}
	payload.CoreVersion = version.CoreVersion
	payload.Target = target
	return payload, ""
}

// ToMap renders the payload with exact status_of() keys for JSON output.
func (p Payload) ToMap() map[string]any {
	arts := make([]any, 0, len(p.Artifacts))
	for _, a := range p.Artifacts {
		arts = append(arts, map[string]any{
			"path": a.Path, "id": a.ID, "kind": a.Kind, "title": a.Title,
			"status": a.Status, "owner": a.Owner, "version": a.Version,
		})
	}
	invalid := make([]any, 0, len(p.Invalid))
	for _, e := range p.Invalid {
		invalid = append(invalid, validate.EntryMap(e))
	}
	byStatus := map[string]any{}
	for k, v := range p.ByStatus {
		byStatus[k] = int64(v)
	}
	errs := make([]any, 0, len(p.Trace.Errors))
	for _, m := range p.Trace.Errors {
		errs = append(errs, m)
	}
	return map[string]any{
		"coreVersion": p.CoreVersion,
		"target":      p.Target,
		"artifacts":   arts,
		"counts": map[string]any{
			"total":    int64(len(p.Artifacts)),
			"byStatus": byStatus,
			"invalid":  int64(len(p.Invalid)),
		},
		"invalid": invalid,
		"trace": map[string]any{
			"nodes":  int64(p.Trace.Nodes),
			"edges":  int64(p.Trace.Edges),
			"errors": errs,
		},
		"budgets": p.Budgets,
	}
}

// MarshalJSONDoc renders {"ok": True, **payload} like status --json.
func (p Payload) MarshalJSONDoc() []byte {
	doc := p.ToMap()
	doc["ok"] = true
	out, err := jsoncanon.Marshal(doc)
	if err != nil {
		return []byte("{}")
	}
	return out
}

// FormatHuman mirrors format_human().
func FormatHuman(payload Payload) string {
	var b strings.Builder
	fmt.Fprintf(&b, "shiploom status: %s (core %s)\n", payload.Target, payload.CoreVersion)
	total := len(payload.Artifacts)
	fmt.Fprintf(&b, "artifacts: %d  trace nodes/edges: %d/%d\n",
		total, payload.Trace.Nodes, payload.Trace.Edges)
	keys := make([]string, 0, len(payload.ByStatus))
	for k := range payload.ByStatus {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, payload.ByStatus[k]))
	}
	joined := strings.Join(parts, ", ")
	if joined == "" {
		joined = "none"
	}
	fmt.Fprintf(&b, "by status: %s\n", joined)
	for _, art := range payload.Artifacts {
		fmt.Fprintf(&b, "  %-10s %-12s %-10s %s\n",
			validate.PyStr(art.ID), validate.PyStr(art.Kind),
			validate.PyStr(art.Status), art.Path)
	}
	for _, inv := range payload.Invalid {
		fmt.Fprintf(&b, "  INVALID %s: %s\n", inv.Path, inv.Message)
	}
	for _, msg := range payload.Trace.Errors {
		fmt.Fprintf(&b, "  TRACE ERROR: %s\n", msg)
	}
	budgets, _ := payload.Budgets.(map[string]any)
	if budgets == nil {
		budgets = map[string]any{}
	}
	if st, _ := budgets["status"].(string); st == "unavailable" {
		reason, _ := budgets["reason"].(string)
		if reason == "" {
			reason = "budgets unavailable"
		}
		fmt.Fprintf(&b, "%s\n", reason)
	} else {
		var slots []string
		for _, key := range []string{"tokens", "spendUSD"} {
			if slot, ok := budgets[key].(map[string]any); ok {
				slots = append(slots, fmt.Sprintf("%s %s/%s", key,
					validate.PyStr(slot["used"]), validate.PyStr(slot["limit"])))
			}
		}
		joined := strings.Join(slots, ", ")
		if joined == "" {
			joined = "none tracked"
		}
		fmt.Fprintf(&b, "budgets: %s\n", joined)
	}
	return b.String()
}

func fsErrText(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
