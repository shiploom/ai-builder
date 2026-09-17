// Package run ports cli/run.py (stdlib only): the idempotent workflow
// stepper. Takes schemas explicitly (Python resolves TOOL_ROOT internally).
package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/shiploom/ai-builder/internal/auditlog"
	"github.com/shiploom/ai-builder/internal/execrun"
	"github.com/shiploom/ai-builder/internal/gates"
	"github.com/shiploom/ai-builder/internal/jsoncanon"
	"github.com/shiploom/ai-builder/internal/manifest"
	"github.com/shiploom/ai-builder/internal/oracle"
	"github.com/shiploom/ai-builder/internal/policy"
	"github.com/shiploom/ai-builder/internal/validate"
	"github.com/shiploom/ai-builder/internal/workflow"
)

// Exit codes mirror cli/run.py.
const (
	ExitOK         = 0
	ExitValidation = 2
	ExitPolicy     = 3
	ExitBudget     = 4
)

// Report mirrors run_workflow()'s report dict.
type Report struct {
	Workflow  string
	Advanced  []string
	Paused    any // nil or string
	Completed bool
	Errors    []string
	Warnings  []map[string]any
}

// Marshal renders the report with exact run_workflow() keys for JSON output.
func (r *Report) Marshal() []byte {
	advanced := make([]any, 0, len(r.Advanced))
	for _, s := range r.Advanced {
		advanced = append(advanced, s)
	}
	errs := make([]any, 0, len(r.Errors))
	for _, e := range r.Errors {
		errs = append(errs, e)
	}
	warns := make([]any, 0, len(r.Warnings))
	for _, w := range r.Warnings {
		warns = append(warns, w)
	}
	doc := map[string]any{
		"workflow": r.Workflow, "advanced": advanced, "paused": r.Paused,
		"completed": r.Completed, "errors": errs, "warnings": warns,
	}
	out, err := jsoncanon.Marshal(doc)
	if err != nil {
		return []byte("{}")
	}
	return out
}

// RunWorkflow mirrors run_workflow(). fromStep/only are nil when the flag
// is absent (Python None). Returns (exit_code, report).
func RunWorkflow(schemas *validate.Schemas, projectDir, workflowName string,
	fromStep, only *string, budgetOverrides map[string]any, actor string) (int, *Report) {
	report := &Report{Workflow: workflowName, Advanced: []string{},
		Errors: []string{}, Warnings: []map[string]any{}}
	root := projectDir

	wfPath := workflow.FindWorkflow(workflowName, root)
	if wfPath == "" {
		report.Errors = append(report.Errors,
			fmt.Sprintf("unknown workflow %s", validate.PyRepr(workflowName)))
		return ExitValidation, report
	}
	fm, wfErrors := workflow.LoadWorkflow(schemas, wfPath)
	if fm == nil {
		report.Errors = append(report.Errors, wfErrors...)
		return ExitValidation, report
	}
	var steps []any
	if raw, ok := fm["steps"].([]any); ok {
		steps = raw
	}
	var stepIDs []string
	for _, s := range steps {
		if obj, ok := s.(map[string]any); ok {
			if id, _ := obj["id"].(string); id != "" {
				stepIDs = append(stepIDs, id)
			}
		}
	}
	wfName, _ := fm["name"].(string)

	data, err := loadManifest(root)
	if err != nil {
		report.Errors = append(report.Errors, err.Error())
		return ExitValidation, report
	}

	// Adopt or guard the workflow binding.
	if wf, _ := data["workflow"].(string); wf != wfName {
		progress, _ := data["steps"].(map[string]any)
		anyDone := false
		for _, v := range progress {
			if st, _ := v.(map[string]any); st != nil {
				if s, _ := st["state"].(string); s == "done" {
					anyDone = true
					break
				}
			}
		}
		if anyDone {
			report.Errors = append(report.Errors, fmt.Sprintf("workflow %s in progress (manifest binds %s)",
				validate.PyRepr(data["workflow"]), validate.PyRepr(wfName)))
			return ExitValidation, report
		}
		data["workflow"] = wfName
		if ver, ok := fm["version"]; ok {
			data["workflowVersion"] = ver
		} else {
			data["workflowVersion"] = "1.0.0"
		}
		data["steps"] = map[string]any{}
	}

	// Budgets: adopt wallClockH default, apply overrides.
	budgets, _ := data["budgets"].(map[string]any)
	if budgets == nil {
		budgets = manifest.DefaultBudgets()
		data["budgets"] = budgets
	}
	wfBudgets, _ := fm["budgets"].(map[string]any)
	if wfBudgets == nil {
		wfBudgets = map[string]any{}
	}
	if wall, ok := wfBudgets["wallClockH"]; ok {
		if _, present := budgets["wallClockH"]; !present {
			budgets["wallClockH"] = map[string]any{"limit": wall, "used": 0.0}
		}
	}
	for key, value := range budgetOverrides {
		if key != "tokens" && key != "spendUSD" && key != "wallClockH" {
			report.Errors = append(report.Errors,
				fmt.Sprintf("unknown budget key %s", validate.PyRepr(key)))
			return ExitValidation, report
		}
		slot, _ := budgets[key].(map[string]any)
		if slot == nil {
			slot = map[string]any{"limit": value, "used": int64(0)}
			budgets[key] = slot
		}
		slot["limit"] = value
	}
	if _, ok := data["startedAt"]; !ok {
		data["startedAt"] = manifest.Utcnow()
	}

	budgetExceeded := func() bool {
		slot, _ := budgets["wallClockH"].(map[string]any)
		if len(slot) == 0 {
			return false
		}
		limit, ok := numFloat(slot["limit"])
		if !ok {
			return false
		}
		return elapsedH(data) > limit
	}

	if budgetExceeded() {
		data["budgets"] = budgets
		if err := saveManifest(root, data); err != nil {
			return saveErr(report, err)
		}
		pause(report, "wall-clock budget exceeded (exit 4)")
		return ExitBudget, report
	}

	// Slice selection.
	var targets []string
	if only != nil {
		if !contains(stepIDs, *only) {
			report.Errors = append(report.Errors,
				fmt.Sprintf("unknown step %s", validate.PyRepr(*only)))
			return ExitValidation, report
		}
		idx := indexOf(stepIDs, *only)
		stepsMap, _ := data["steps"].(map[string]any)
		for _, prev := range stepIDs[:idx] {
			prevState, _ := stepsMap[prev].(map[string]any)
			if prevState == nil {
				report.Errors = append(report.Errors, fmt.Sprintf(
					"predecessor %s incomplete (run without --only first)",
					validate.PyRepr(prev)))
				return ExitValidation, report
			}
			if s, _ := prevState["state"].(string); s != "done" {
				report.Errors = append(report.Errors, fmt.Sprintf(
					"predecessor %s incomplete (run without --only first)",
					validate.PyRepr(prev)))
				return ExitValidation, report
			}
		}
		targets = []string{*only}
	} else {
		targets = append([]string{}, stepIDs...)
		if fromStep != nil {
			if !contains(stepIDs, *fromStep) {
				report.Errors = append(report.Errors,
					fmt.Sprintf("unknown step %s", validate.PyRepr(*fromStep)))
				return ExitValidation, report
			}
			resetFrom := indexOf(stepIDs, *fromStep)
			stepsMap, _ := data["steps"].(map[string]any)
			retriesMap, _ := data["retries"].(map[string]any)
			for _, sid := range stepIDs[resetFrom:] {
				if stepsMap != nil {
					delete(stepsMap, sid)
				}
				if retriesMap != nil {
					delete(retriesMap, sid)
				}
			}
		}
	}

	if err := auditAppend(root, actor, "run.start",
		fmt.Sprintf("%s (targets: %s)", wfName, strings.Join(targets, ","))); err != nil {
		return auditErr(report, err)
	}

	for _, sid := range targets {
		if budgetExceeded() {
			if err := saveManifest(root, data); err != nil {
				return saveErr(report, err)
			}
			pause(report, "wall-clock budget exceeded (exit 4)")
			if err := auditAppend(root, "system", "run.paused", "budget exceeded"); err != nil {
				return auditErr(report, err)
			}
			return ExitBudget, report
		}
		var step map[string]any
		for _, s := range steps {
			if obj, ok := s.(map[string]any); ok {
				if id, _ := obj["id"].(string); id == sid {
					step = obj
					break
				}
			}
		}
		stepsMap := getMap(data, "steps")
		state := getMapInto(stepsMap, sid)
		if _, ok := state["state"]; !ok {
			state["state"] = "pending"
		}
		if st, _ := state["state"].(string); st == "done" && only == nil {
			if !outputsHold(schemas, step, root) || !gateHolds(step, data) {
				state["state"] = "pending"
				if err := auditAppend(root, "system", "run.reopened", sid); err != nil {
					return auditErr(report, err)
				}
			} else {
				continue
			}
		}

		ref, _ := step["uses"].(string)
		if ref != "" && workflow.ResolveUses(ref, root) == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("step %s: unknown uses ref %s",
				validate.PyRepr(sid), validate.PyRepr(ref)))
			return ExitValidation, report
		}

		var consumes []any
		if c, ok := step["consumes"].([]any); ok {
			consumes = c
		}
		var missingInputs []string
		for _, c := range consumes {
			pat, _ := c.(string)
			if len(workflow.GlobHits(pat, root)) == 0 {
				missingInputs = append(missingInputs, pat)
			}
		}
		if len(missingInputs) > 0 {
			report.Errors = append(report.Errors, fmt.Sprintf("step %s: missing inputs: %s",
				validate.PyRepr(sid), strings.Join(missingInputs, ", ")))
			return ExitValidation, report
		}

		gate, _ := step["gate"].(string)
		if gate == "" {
			gate = "none"
		}
		if gate == "human-approval" {
			gateState := gateStateOf(data, sid)
			if gateState == "passed" {
				// proceed
			} else if gateState == "denied" {
				report.Errors = append(report.Errors,
					fmt.Sprintf("step %s: gate denied (re-approve to unblock)", validate.PyRepr(sid)))
				return ExitValidation, report
			} else {
				if err := saveManifest(root, data); err != nil {
					return saveErr(report, err)
				}
				pause(report, fmt.Sprintf("awaiting approval: %s (shiploom approve %s)", sid, sid))
				if err := auditAppend(root, "system", "run.paused",
					fmt.Sprintf("awaiting approval: %s", sid)); err != nil {
					return auditErr(report, err)
				}
				return ExitOK, report
			}
		} else if gate == "policy" {
			packRef, _ := step["policy"].(string)
			action, _ := step["action"].(string)
			resource, _ := step["resource"].(string)
			decision, ruleID, message, err := policy.CheckStep(
				root, packRef, wfName, sid, actor, action, resource)
			if err != nil {
				report.Errors = append(report.Errors,
					fmt.Sprintf("step %s: %s", validate.PyRepr(sid), err))
				return ExitValidation, report
			}
			if decision == "deny" {
				if err := auditAppend(root, "system", "run.policy.deny", sid); err != nil {
					return auditErr(report, err)
				}
				if err := saveManifest(root, data); err != nil {
					return saveErr(report, err)
				}
				suffix := ""
				if ruleID != "" {
					suffix = fmt.Sprintf(" (%s: %s)", ruleID, message)
				}
				report.Errors = append(report.Errors,
					fmt.Sprintf("step %s: policy deny%s", validate.PyRepr(sid), suffix))
				return ExitPolicy, report
			}
			if decision == "require-approval" {
				gateState := gateStateOf(data, sid)
				if gateState == "passed" {
					// proceed
				} else if gateState == "denied" {
					report.Errors = append(report.Errors, fmt.Sprintf(
						"step %s: gate denied (re-approve to unblock)", validate.PyRepr(sid)))
					return ExitValidation, report
				} else {
					if err := saveManifest(root, data); err != nil {
						return saveErr(report, err)
					}
					extra := ""
					if message != "" {
						extra = fmt.Sprintf(" [%s]", message)
					}
					pause(report, fmt.Sprintf(
						"policy requires approval: %s (shiploom approve %s)%s", sid, sid, extra))
					if err := auditAppend(root, "system", "run.paused",
						fmt.Sprintf("policy approval: %s", sid)); err != nil {
						return auditErr(report, err)
					}
					return ExitOK, report
				}
			}
			// allow: proceed; fall through to produces check
		} else if gate == "verification" {
			outcome, verrs, vwarns := checkVerificationGate(schemas, sid, root)
			for _, w := range vwarns {
				report.Warnings = append(report.Warnings, validate.EntryMap(w))
			}
			if outcome == nil {
				if err := saveManifest(root, data); err != nil {
					return saveErr(report, err)
				}
				pause(report, fmt.Sprintf("no verification checker wired for %s (PR8)",
					validate.PyRepr(sid)))
				return ExitOK, report
			}
			if !*outcome {
				noLock := false
				for _, e := range verrs {
					if strings.Contains(e.Message, "no acceptance lock") {
						noLock = true
						break
					}
				}
				if noLock {
					if err := saveManifest(root, data); err != nil {
						return saveErr(report, err)
					}
					pause(report, "acceptance not locked yet (run shiploom lock, then re-run)")
					return ExitOK, report
				}
				retriesMap := getMap(data, "retries")
				attempts := toInt(retriesMap[sid]) + 1
				retriesMap[sid] = int64(attempts)
				limit := toInt(step["retries"])
				var first3 []string
				for i, e := range verrs {
					if i >= 3 {
						break
					}
					first3 = append(first3, e.Message)
				}
				detail := strings.Join(first3, "; ")
				if attempts <= limit {
					if err := saveManifest(root, data); err != nil {
						return saveErr(report, err)
					}
					pause(report, fmt.Sprintf("verification failed on %s (attempt %d/%d): %s",
						validate.PyRepr(sid), attempts, limit, detail))
					if err := auditAppend(root, "system", "run.retry",
						fmt.Sprintf("%s attempt %d", sid, attempts)); err != nil {
						return auditErr(report, err)
					}
					return ExitOK, report
				}
				onFail, _ := step["onFail"].(string)
				if onFail == "" {
					onFail = "replan"
				}
				if onFail == "abort" {
					report.Errors = append(report.Errors, fmt.Sprintf(
						"step %s failed verification: %s", validate.PyRepr(sid), detail))
					return ExitValidation, report
				}
				stepsMap := getMap(data, "steps")
				idx := indexOf(stepIDs, sid)
				for _, later := range stepIDs[idx:] {
					delete(stepsMap, later)
				}
				delete(getMap(data, "retries"), sid)
				if err := saveManifest(root, data); err != nil {
					return saveErr(report, err)
				}
				appendCheckpoint(data, map[string]any{
					"id": "cp-replan-" + sid, "at": manifest.Utcnow(), "manifestHash": "replan"})
				if err := saveManifest(root, data); err != nil {
					return saveErr(report, err)
				}
				pause(report, fmt.Sprintf("replan required after %d attempts on %s: %s",
					attempts-1, validate.PyRepr(sid), detail))
				if err := auditAppend(root, "system", "run.replan", sid); err != nil {
					return auditErr(report, err)
				}
				return ExitOK, report
			}
			delete(getMap(data, "retries"), sid)
		}

		var produces []any
		if p, ok := step["produces"].([]any); ok {
			produces = p
		}
		var missingOutputs []string
		var producedFiles []string
		for _, p := range produces {
			pat, _ := p.(string)
			hits := workflow.GlobHits(pat, root)
			if len(hits) == 0 {
				missingOutputs = append(missingOutputs, pat)
			} else {
				producedFiles = append(producedFiles, hits...)
			}
		}
		if len(missingOutputs) > 0 {
			if err := saveManifest(root, data); err != nil {
				return saveErr(report, err)
			}
			hint := ""
			if ref != "" {
				hint = fmt.Sprintf(" (harness executes %s, then re-run)",
					filepath.Base(workflow.ResolveUses(ref, root)))
			}
			pause(report, fmt.Sprintf("step %s incomplete, missing outputs: %s%s",
				validate.PyRepr(sid), strings.Join(missingOutputs, ", "), hint))
			return ExitOK, report
		}
		for _, path := range producedFiles {
			if suffix := pySuffix(path); suffix != ".md" && suffix != ".json" {
				continue
			}
			fileErrors, _, _ := validate.ValidatePath(schemas, path, false)
			if len(fileErrors) > 0 {
				if err := saveManifest(root, data); err != nil {
					return saveErr(report, err)
				}
				pause(report, fmt.Sprintf("step %s outputs fail validation: %s",
					validate.PyRepr(sid), fileErrors[0].Message))
				return ExitOK, report
			}
		}

		state["state"] = "done"
		state["at"] = manifest.Utcnow()
		report.Advanced = append(report.Advanced, sid)
		done := map[string]any{
			"id": "cp-" + sid, "at": state["at"], "manifestHash": "pending"}
		appendCheckpoint(data, done)
		if err := saveManifest(root, data); err != nil {
			return saveErr(report, err)
		}
		hash, err := manifest.Sha256File(manifest.ManifestPath(root))
		if err != nil {
			return saveErr(report, err)
		}
		done["manifestHash"] = hash
		if err := saveManifest(root, data); err != nil {
			return saveErr(report, err)
		}
		if err := auditAppend(root, actor, "run.step.done", sid); err != nil {
			return auditErr(report, err)
		}
	}

	states, _ := data["steps"].(map[string]any)
	allDone := true
	for _, sid := range stepIDs {
		st, _ := states[sid].(map[string]any)
		if st == nil {
			allDone = false
			break
		}
		if s, _ := st["state"].(string); s != "done" {
			allDone = false
			break
		}
	}
	if allDone {
		report.Completed = true
		if err := auditAppend(root, actor, "run.complete", wfName); err != nil {
			return auditErr(report, err)
		}
	}
	if err := saveManifest(root, data); err != nil {
		return saveErr(report, err)
	}
	return ExitOK, report
}

// checkVerificationGate mirrors _check_verification_gate(). Returns
// (ok|nil, errors, warnings); nil means no checker wired.
func checkVerificationGate(schemas *validate.Schemas, stepID, projectDir string) (*bool, []validate.Entry, []validate.Entry) {
	if stepID == "acceptance-lock" {
		res := oracle.CheckLock(projectDir)
		ok := res.Ok
		return &ok, res.Errors, res.Warnings
	}
	if stepID == "verify" || stepID == "regression-verify" {
		ok, errs, warns := gates.VerifyStepCheck(projectDir, schemas)
		return &ok, errs, warns
	}
	return nil, nil, nil
}

// outputsHold mirrors _outputs_hold().
func outputsHold(schemas *validate.Schemas, step map[string]any, root string) bool {
	var produces []any
	if p, ok := step["produces"].([]any); ok {
		produces = p
	}
	for _, p := range produces {
		pat, _ := p.(string)
		hits := workflow.GlobHits(pat, root)
		if len(hits) == 0 {
			return false
		}
		for _, path := range hits {
			if suffix := pySuffix(path); suffix != ".md" && suffix != ".json" {
				continue
			}
			fileErrors, _, _ := validate.ValidatePath(schemas, path, false)
			if len(fileErrors) > 0 {
				return false
			}
		}
	}
	return true
}

// gateHolds mirrors _gate_holds().
func gateHolds(step map[string]any, data map[string]any) bool {
	gate, _ := step["gate"].(string)
	if gate == "" {
		gate = "none"
	}
	if gate != "human-approval" && gate != "policy" {
		return true
	}
	sid, _ := step["id"].(string)
	return gateStateOf(data, sid) == "passed"
}

func gateStateOf(data map[string]any, sid string) string {
	gatesMap, _ := data["gates"].(map[string]any)
	if gatesMap == nil {
		return ""
	}
	entry, _ := gatesMap[sid].(map[string]any)
	if entry == nil {
		return ""
	}
	state, _ := entry["state"].(string)
	return state
}

// elapsedH mirrors _elapsed_h().
func elapsedH(data map[string]any) float64 {
	started, _ := data["startedAt"].(string)
	if started == "" {
		return 0.0
	}
	start, err := time.Parse("2006-01-02T15:04:05Z", started)
	if err != nil {
		return 0.0
	}
	hours := time.Since(start).Hours()
	if hours < 0 {
		return 0.0
	}
	return hours
}

func pause(report *Report, reason string) {
	report.Paused = reason
}

// ParseBudgetFlags mirrors parse_budget_flags().
func ParseBudgetFlags(flags []string) (map[string]any, error) {
	out := map[string]any{}
	for _, flag := range flags {
		idx := strings.Index(flag, "=")
		if idx < 0 {
			return nil, fmt.Errorf("bad --budget %s (want KEY=VALUE)", validate.PyRepr(flag))
		}
		key := strings.TrimSpace(flag[:idx])
		raw := flag[idx+1:]
		if strings.Contains(raw, ".") {
			f, err := pyFloat(raw)
			if err != nil {
				return nil, fmt.Errorf("bad --budget %s (value must be numeric)", validate.PyRepr(flag))
			}
			out[key] = f
		} else {
			n, err := pyInt(raw)
			if err != nil {
				return nil, fmt.Errorf("bad --budget %s (value must be numeric)", validate.PyRepr(flag))
			}
			out[key] = n
		}
	}
	return out, nil
}

// pyInt mirrors int(raw): base-10, surrounding whitespace and
// underscores allowed, else error.
func pyInt(raw string) (int64, error) {
	s := strings.TrimSpace(raw)
	s = strings.ReplaceAll(s, "_", "")
	if s == "" {
		return 0, fmt.Errorf("bad int")
	}
	return strconv.ParseInt(s, 10, 64)
}

// pyFloat mirrors float(raw).
func pyFloat(raw string) (float64, error) {
	s := strings.TrimSpace(raw)
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f, nil
	}
	nos := strings.ReplaceAll(s, "_", "")
	if nos != s {
		if f, err := strconv.ParseFloat(nos, 64); err == nil {
			return f, nil
		}
	}
	return 0, fmt.Errorf("bad float")
}

// numFloat mirrors float(slot["limit"]): numerics, numeric strings,
// bools; anything else is not-a-number.
func numFloat(v any) (float64, bool) {
	if f, ok := execrun.ToFloat(v); ok {
		return f, true
	}
	switch t := v.(type) {
	case string:
		if f, err := pyFloat(t); err == nil {
			return f, true
		}
		return 0, false
	case bool:
		if t {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// toInt mirrors the int() reads of retries/limits (JSON numbers decode
// as json.Number; absent/malformed reads as 0).
func toInt(v any) int {
	if f, ok := numFloat(v); ok {
		return int(f)
	}
	return 0
}

// pySuffix mirrors pathlib suffix: extension after the last dot of the
// final component, empty for dotfiles like ".foo".
func pySuffix(path string) string {
	base := filepath.Base(path)
	dot := strings.LastIndexByte(base, '.')
	if dot <= 0 {
		return ""
	}
	return base[dot:]
}

// Manifest store helpers (setdefault parity; save errors surface via
// the report since run must stay total).

func loadManifest(root string) (map[string]any, error) {
	return LoadManifest(root)
}

// LoadManifest loads the project manifest with run_workflow()'s
// "no manifest: ..." error shape (also used by approvals).
func LoadManifest(root string) (map[string]any, error) {
	data, err := manifest.Load(root)
	if err != nil {
		if os.IsNotExist(err) {
			// Byte-parity with FileNotFoundError str().
			return nil, fmt.Errorf("no manifest: [Errno 2] No such file or directory: '%s' (run shiploom init)",
				manifest.ManifestPath(root))
		}
		return nil, fmt.Errorf("no manifest: %s (run shiploom init)", err)
	}
	return data, nil
}

func saveManifest(root string, data map[string]any) error {
	_, err := manifest.Save(root, data)
	return err
}

func getMap(data map[string]any, key string) map[string]any {
	m, _ := data[key].(map[string]any)
	if m == nil {
		m = map[string]any{}
		data[key] = m
	}
	return m
}

func getMapInto(parent map[string]any, key string) map[string]any {
	m, _ := parent[key].(map[string]any)
	if m == nil {
		m = map[string]any{}
		parent[key] = m
	}
	return m
}

// appendCheckpoint appends to data["checkpoints"] keeping the JSON
// []any shape the canonical writer supports.
func appendCheckpoint(data map[string]any, cp map[string]any) {
	var list []any
	if raw, ok := data["checkpoints"].([]any); ok {
		list = raw
	}
	data["checkpoints"] = append(list, cp)
}

func auditAppend(root, actor, action, target string) error {
	_, err := auditlog.Append(root, actor, action, target, "-")
	return err
}

// saveErr/auditErr: audit/save failures crash CPython (uncaught); Go
// records them as validation errors (documented robustness divergence).
func saveErr(report *Report, err error) (int, *Report) {
	report.Errors = append(report.Errors, fmt.Sprintf("manifest save failed: %s", err))
	return ExitValidation, report
}

func auditErr(report *Report, err error) (int, *Report) {
	report.Errors = append(report.Errors, fmt.Sprintf("audit append failed: %s", err))
	return ExitValidation, report
}

func contains(list []string, s string) bool {
	return indexOf(list, s) >= 0
}

func indexOf(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}
