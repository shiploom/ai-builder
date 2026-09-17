// Package approvals ports cli/approvals.py (stdlib only):
// pending-approval discovery (gate-state reads only).
package approvals

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/shiploom/ai-builder/internal/policy"
	"github.com/shiploom/ai-builder/internal/run"
	"github.com/shiploom/ai-builder/internal/validate"
	"github.com/shiploom/ai-builder/internal/workflow"
)

// Entry mirrors a pending_approvals() entry dict.
type Entry struct {
	Gate     string
	Kind     string
	Step     string
	State    string
	Attempts int
	Message  string
}

// ToMap renders the entry with exact pending_approvals() keys.
func (e Entry) ToMap() map[string]any {
	return map[string]any{
		"gate": e.Gate, "kind": e.Kind, "step": e.Step,
		"state": e.State, "attempts": int64(e.Attempts), "message": e.Message,
	}
}

// PendingApprovals mirrors pending_approvals(). Raises (as error) without
// manifest/workflow. Takes schemas explicitly.
func PendingApprovals(schemas *validate.Schemas, projectDir string) ([]Entry, error) {
	data, err := run.LoadManifest(projectDir)
	if err != nil {
		return nil, err
	}
	wfName, _ := data["workflow"].(string)
	wfPath := workflow.FindWorkflow(wfName, projectDir)
	if wfPath == "" {
		return nil, fmt.Errorf("manifest workflow %s not found", validate.PyRepr(data["workflow"]))
	}
	fm, wfErrors := workflow.LoadWorkflow(schemas, wfPath)
	if fm == nil {
		return nil, fmt.Errorf("%s", joinStrings(wfErrors, "; "))
	}

	var entries []Entry
	gatesMap, _ := data["gates"].(map[string]any)
	retriesMap, _ := data["retries"].(map[string]any)
	var steps []any
	if raw, ok := fm["steps"].([]any); ok {
		steps = raw
	}
	for _, s := range steps {
		step, ok := s.(map[string]any)
		if !ok {
			continue
		}
		sid, _ := step["id"].(string)
		gate, _ := step["gate"].(string)
		if gate == "" {
			gate = "none"
		}
		attempts := toInt(retriesMap[sid])
		if gate == "human-approval" {
			state := ""
			if g, ok := gatesMap[sid].(map[string]any); ok {
				state, _ = g["state"].(string)
			}
			if state == "" {
				state = "awaiting"
			}
			if state != "passed" {
				display := "awaiting"
				if state == "denied" {
					display = "denied"
				}
				entries = append(entries, Entry{Gate: sid, Kind: "human-approval",
					Step: sid, State: display, Attempts: attempts})
			}
		} else if gate == "policy" {
			packRef, _ := step["policy"].(string)
			action, _ := step["action"].(string)
			resource, _ := step["resource"].(string)
			decision, ruleID, message, err := policy.CheckStep(
				projectDir, packRef, wfName, sid, "human", action, resource)
			if err != nil {
				entries = append(entries, Entry{Gate: sid, Kind: "policy",
					Step: sid, State: "error", Attempts: attempts, Message: fmt.Sprint(err)})
				continue
			}
			gateState := ""
			if g, ok := gatesMap[sid].(map[string]any); ok {
				gateState, _ = g["state"].(string)
			}
			if decision == "deny" {
				msg := message
				if ruleID != "" {
					msg = ruleID + ": " + message
				}
				entries = append(entries, Entry{Gate: sid, Kind: "policy",
					Step: sid, State: "blocked", Attempts: attempts, Message: msg})
			} else if decision == "require-approval" && gateState != "passed" {
				display := "awaiting"
				if gateState == "denied" {
					display = "denied"
				}
				entries = append(entries, Entry{Gate: sid, Kind: "policy",
					Step: sid, State: display, Attempts: attempts, Message: message})
			}
		}
	}
	if entries == nil {
		entries = []Entry{}
	}
	return entries, nil
}

func toInt(v any) int {
	switch t := v.(type) {
	case int64:
		return int(t)
	case int:
		return t
	case float64:
		return int(t)
	case json.Number:
		if n, err := strconv.ParseInt(string(t), 10, 64); err == nil {
			return int(n)
		}
		if f, err := strconv.ParseFloat(string(t), 64); err == nil {
			return int(f)
		}
		return 0
	default:
		return 0
	}
}

func joinStrings(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}
