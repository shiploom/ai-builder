// Package trace ports validators/trace.py (stdlib only): the
// Requirement -> Decision -> Implementation -> Test -> Verification
// Result index plus the trace CLI surface data.
package trace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"unicode/utf8"

	"github.com/shiploom/ai-builder/internal/validate"
)

// Relations in fixed order (mirrors RELATIONS).
var Relations = []string{"requires", "decided_by", "implemented_by", "tested_by", "verified_by"}

// EmptyLinks mirrors empty_links().
func EmptyLinks() map[string][]string {
	return map[string][]string{
		"requires": {}, "decided_by": {}, "implemented_by": {},
		"tested_by": {}, "verified_by": {},
	}
}

// Trace maps artifact id -> relation -> targets.
type Trace map[string]map[string][]string

// BuildTrace mirrors build_trace(). Returns (trace, errors, warnings);
// errors/warnings are unsorted (callers sort, like trace.main).
func BuildTrace(schemas *validate.Schemas, target string, strict bool) (Trace, []validate.Entry, []validate.Entry) {
	var errors []validate.Entry
	var warnings []validate.Entry
	trace := Trace{}
	root := target
	base := target
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		base = filepath.Dir(target)
	}
	files := validate.CollectFiles(root)
	for _, path := range files {
		if validate.IsExcluded(path, base) {
			continue
		}
		if filepath.Ext(path) == ".md" {
			raw, err := os.ReadFile(path)
			if err != nil {
				errors = append(errors, validate.Entry{Path: path,
					Message: fmt.Sprintf("unreadable: %s", fsErrText(err))})
				continue
			}
			text, ok := utf8String(raw)
			if !ok {
				errors = append(errors, validate.Entry{Path: path,
					Message: fmt.Sprintf("unreadable: invalid UTF-8 in %s", path)})
				continue
			}
			fm, _, perr := validate.ParseFrontmatter(text)
			if fm == nil {
				if perr != "" {
					errors = append(errors, validate.Entry{Path: path,
						Message: perr, Rule: "frontmatter"})
				}
				continue
			}
			if _, hasID := fm["id"]; !hasID {
				continue
			}
			if _, hasKind := fm["kind"]; !hasKind {
				continue
			}
			aid, ok := fm["id"].(string)
			if !ok {
				continue
			}
			if _, dup := trace[aid]; dup {
				errors = append(errors, validate.Entry{Path: path,
					Message: fmt.Sprintf("duplicate artifact id %s", validate.PyRepr(aid)),
					Rule:    "trace.duplicate"})
				continue
			}
			links := EmptyLinks()
			if rawLinks, ok := fm["links"].(map[string]any); ok {
				for _, rel := range Relations {
					if vals, ok := rawLinks[rel].([]any); ok {
						for _, v := range vals {
							if s, ok := v.(string); ok {
								links[rel] = append(links[rel], s)
							}
						}
					}
					if links[rel] == nil {
						links[rel] = []string{}
					}
				}
			}
			trace[aid] = links
		} else if filepath.Ext(path) == ".json" && filepath.Base(path) != "trace.json" {
			raw, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			doc, err := validate.DecodeJSON(raw)
			if err != nil {
				continue
			}
			if obj, ok := doc.(map[string]any); ok {
				if _, hasSchema := obj["$schema"]; hasSchema {
					continue
				}
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
				if _, hasR := obj["results"]; hasR {
					if _, hasV := obj["verdict"]; hasV {
						if rid, ok := obj["id"].(string); ok {
							if _, exists := trace[rid]; !exists {
								var tested []string
								if results, ok := obj["results"].([]any); ok {
									for _, r := range results {
										if robj, ok := r.(map[string]any); ok {
											if acc, ok := robj["acceptanceId"].(string); ok {
												tested = append(tested, acc)
											}
										}
									}
								}
								if tested == nil {
									tested = []string{}
								}
								entry := EmptyLinks()
								entry["tested_by"] = tested
								trace[rid] = entry
							}
						}
					}
				} else if _, hasH := obj["howToVerify"]; hasH {
					if id, ok := obj["id"].(string); ok {
						if _, exists := trace[id]; !exists {
							trace[id] = EmptyLinks()
						}
					}
				}
			}
		}
	}
	schema, err := schemas.Load("trace")
	if err == nil {
		for _, msg := range validate.ValidateAgainstSchema(traceToAny(trace), schema, "$") {
			errors = append(errors, validate.Entry{Path: root, Message: msg, Rule: "trace.schema"})
		}
	}
	known := map[string]bool{}
	for aid := range trace {
		known[aid] = true
	}
	aids := make([]string, 0, len(trace))
	for aid := range trace {
		aids = append(aids, aid)
	}
	sort.Strings(aids)
	for _, aid := range aids {
		for _, rel := range Relations {
			for _, tgt := range trace[aid][rel] {
				if !known[tgt] {
					msg := fmt.Sprintf("%s: dangling link %s -> %s",
						aid, rel, validate.PyRepr(tgt))
					entry := validate.Entry{Path: root, Message: msg, Rule: "links.dangling"}
					if strict {
						errors = append(errors, entry)
					} else {
						warnings = append(warnings, entry)
					}
				}
			}
		}
	}
	return trace, errors, warnings
}

func traceToAny(trace Trace) map[string]any {
	out := map[string]any{}
	for aid, links := range trace {
		m := map[string]any{}
		for rel, targets := range links {
			list := make([]any, 0, len(targets))
			for _, t := range targets {
				list = append(list, t)
			}
			m[rel] = list
		}
		out[aid] = m
	}
	return out
}

func utf8String(raw []byte) (string, bool) {
	if !utf8.Valid(raw) {
		return "", false
	}
	return string(raw), true
}

func fsErrText(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
