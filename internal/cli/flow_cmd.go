// Command bodies for `approve`, `budget`, and `resume` (stdlib only, P4).
//
// Flag parsing is hand-rolled like the other ported commands: covered
// behaviors are byte-identical to cli/shiploom.py; argparse-only error
// paths stay Python-side.
package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/shiploom/ai-builder/internal/approvals"
	"github.com/shiploom/ai-builder/internal/auditlog"
	"github.com/shiploom/ai-builder/internal/jsoncanon"
	"github.com/shiploom/ai-builder/internal/manifest"
	"github.com/shiploom/ai-builder/internal/run"
	"github.com/shiploom/ai-builder/internal/validate"
	"github.com/shiploom/ai-builder/internal/workflow"
)

func init() {
	Register("approve", RunApprove)
	Register("budget", RunBudget)
	Register("resume", RunResume)
}

// RunApprove implements `approve [--deny] [--reason R] [--actor A] [--json] gate_id`.
func RunApprove(args []string) int {
	deny := false
	var reason *string
	actor := "human"
	jsonOut := false
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--deny":
			deny = true
		case arg == "--json":
			jsonOut = true
		case arg == "--actor":
			if i+1 >= len(args) {
				return fail("argument --actor: expected one argument", ExitValidation)
			}
			i++
			actor = args[i]
		case strings.HasPrefix(arg, "--actor="):
			actor = strings.TrimPrefix(arg, "--actor=")
		case arg == "--reason":
			if i+1 >= len(args) {
				return fail("argument --reason: expected one argument", ExitValidation)
			}
			i++
			v := args[i]
			reason = &v
		case strings.HasPrefix(arg, "--reason="):
			v := strings.TrimPrefix(arg, "--reason=")
			reason = &v
		case arg == "--help" || arg == "-h":
			fmt.Println("usage: shiploom approve [--deny] [--reason R] [--actor A] [--json] gate_id")
			fmt.Println()
			fmt.Println("Record a human-approval gate.")
			return ExitOK
		case strings.HasPrefix(arg, "-"):
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		default:
			positional = append(positional, arg)
		}
	}
	if len(positional) == 0 {
		return fail("the following arguments are required: gate_id", ExitValidation)
	}
	if len(positional) > 1 {
		return fail(fmt.Sprintf("unrecognized arguments: %s",
			strings.Join(positional[1:], " ")), ExitValidation)
	}
	gateID := positional[0]
	data, err := run.LoadManifest(".")
	if err != nil {
		return fail(err.Error(), ExitValidation)
	}
	wfName, _ := data["workflow"].(string)
	if wfName == "" {
		// Mirror data.get("workflow") or "" then %r of the raw value.
		return fail(fmt.Sprintf("manifest workflow %s not found",
			validate.PyRepr(data["workflow"])), ExitValidation)
	}
	wfPath := workflow.FindWorkflow(wfName, ".")
	if wfPath == "" {
		return fail(fmt.Sprintf("manifest workflow %s not found",
			validate.PyRepr(data["workflow"])), ExitValidation)
	}
	schemasDir, serr := validate.DefaultSchemasDir()
	if serr != nil {
		return fail(serr.Error(), ExitValidation)
	}
	schemas := validate.NewSchemas(schemasDir)
	fm, wfErrors := workflow.LoadWorkflow(schemas, wfPath)
	if fm == nil {
		return fail(strings.Join(wfErrors, "; "), ExitValidation)
	}
	var step map[string]any
	if steps, ok := fm["steps"].([]any); ok {
		for _, s := range steps {
			if m, ok := s.(map[string]any); ok {
				if id, _ := m["id"].(string); id == gateID {
					step = m
					break
				}
			}
		}
	}
	if step == nil {
		name, _ := fm["name"].(string)
		return fail(fmt.Sprintf("unknown gate %s in workflow %s",
			validate.PyRepr(gateID), name), ExitValidation)
	}
	gate, _ := step["gate"].(string)
	if gate == "" {
		gate = "none"
	}
	if gate != "human-approval" && gate != "policy" {
		return fail(fmt.Sprintf("gate %s is not an approvable gate (human-approval|policy)",
			validate.PyRepr(gateID)), ExitValidation)
	}
	if deny && reason == nil {
		return fail("denying requires --reason", ExitValidation)
	}
	state := "passed"
	action := "approve." + gateID
	if deny {
		state = "denied"
		action = "deny." + gateID
	}
	entry := map[string]any{"state": state, "by": actor, "at": manifest.Utcnow()}
	if reason != nil {
		entry["reason"] = *reason
	}
	gates, _ := data["gates"].(map[string]any)
	if gates == nil {
		gates = map[string]any{}
		data["gates"] = gates
	}
	gates[gateID] = entry
	if _, err := manifest.Save(".", data); err != nil {
		return fail(err.Error(), ExitValidation)
	}
	if _, err := auditlog.Append(".", actor, action, gateID, "-"); err != nil {
		return fail(err.Error(), ExitValidation)
	}
	if jsonOut {
		doc := map[string]any{"ok": true, "gate": gateID}
		for k, v := range entry {
			doc[k] = v
		}
		printJSON(doc)
		return ExitOK
	}
	fmt.Printf("%s %s by %s\n", state, gateID, actor)
	return ExitOK
}

// RunBudget implements `budget [--set K=V] [--actor A] [--json] [path]`.
func RunBudget(args []string) int {
	var setFlags []string
	actor := "human"
	jsonOut := false
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			jsonOut = true
		case arg == "--set":
			if i+1 >= len(args) {
				return fail("argument --set: expected one argument", ExitValidation)
			}
			i++
			setFlags = append(setFlags, args[i])
		case strings.HasPrefix(arg, "--set="):
			setFlags = append(setFlags, strings.TrimPrefix(arg, "--set="))
		case arg == "--actor":
			if i+1 >= len(args) {
				return fail("argument --actor: expected one argument", ExitValidation)
			}
			i++
			actor = args[i]
		case strings.HasPrefix(arg, "--actor="):
			actor = strings.TrimPrefix(arg, "--actor=")
		case arg == "--help" || arg == "-h":
			fmt.Println("usage: shiploom budget [--set K=V] [--actor A] [--json] [path]")
			fmt.Println()
			fmt.Println("Show or set manifest budget limits.")
			return ExitOK
		case strings.HasPrefix(arg, "-"):
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		default:
			positional = append(positional, arg)
		}
	}
	if len(positional) > 1 {
		return fail(fmt.Sprintf("unrecognized arguments: %s",
			strings.Join(positional[1:], " ")), ExitValidation)
	}
	target := "."
	if len(positional) == 1 {
		target = positional[0]
	}
	data, err := run.LoadManifest(target)
	if err != nil {
		return fail(err.Error(), ExitValidation)
	}
	if len(setFlags) > 0 {
		overrides, err := run.ParseBudgetFlags(setFlags)
		if err != nil {
			return fail(err.Error(), ExitValidation)
		}
		for key := range overrides {
			if key != "tokens" && key != "spendUSD" && key != "wallClockH" {
				return fail(fmt.Sprintf("unknown budget key %s", validate.PyRepr(key)), ExitValidation)
			}
		}
		budgets, _ := data["budgets"].(map[string]any)
		if budgets == nil {
			budgets = manifest.DefaultBudgets()
			data["budgets"] = budgets
		}
		for key, value := range overrides {
			slot, _ := budgets[key].(map[string]any)
			if slot == nil {
				slot = map[string]any{"limit": value, "used": int64(0)}
				budgets[key] = slot
			}
			slot["limit"] = value
		}
		if _, err := manifest.Save(target, data); err != nil {
			return fail(err.Error(), ExitValidation)
		}
		keys := make([]string, 0, len(overrides))
		for k := range overrides {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if _, err := auditlog.Append(target, actor, "budget.set", strings.Join(keys, ","), "-"); err != nil {
			return fail(err.Error(), ExitValidation)
		}
	}
	budgets, _ := data["budgets"].(map[string]any)
	if budgets == nil {
		budgets = map[string]any{}
	}
	if jsonOut {
		printJSON(map[string]any{"ok": true, "budgets": budgets})
		return ExitOK
	}
	if len(budgets) == 0 {
		fmt.Println("no budgets tracked")
		return ExitOK
	}
	keys := make([]string, 0, len(budgets))
	for k := range budgets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		slot, _ := budgets[key].(map[string]any)
		if slot == nil {
			continue
		}
		fmt.Printf("%s: %s/%s\n", key,
			validate.PyStr(slot["used"]), validate.PyStr(slot["limit"]))
	}
	return ExitOK
}

// RunResume implements `resume [--budget K=V] [--actor A] [--json]`.
func RunResume(args []string) int {
	var budgetFlags []string
	actor := "human"
	jsonOut := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			jsonOut = true
		case arg == "--budget":
			if i+1 >= len(args) {
				return fail("argument --budget: expected one argument", ExitValidation)
			}
			i++
			budgetFlags = append(budgetFlags, args[i])
		case strings.HasPrefix(arg, "--budget="):
			budgetFlags = append(budgetFlags, strings.TrimPrefix(arg, "--budget="))
		case arg == "--actor":
			if i+1 >= len(args) {
				return fail("argument --actor: expected one argument", ExitValidation)
			}
			i++
			actor = args[i]
		case strings.HasPrefix(arg, "--actor="):
			actor = strings.TrimPrefix(arg, "--actor=")
		case arg == "--help" || arg == "-h":
			fmt.Println("usage: shiploom resume [--budget K=V] [--actor A] [--json]")
			fmt.Println()
			fmt.Println("Report position and advance the bound workflow.")
			return ExitOK
		case strings.HasPrefix(arg, "-"):
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		default:
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		}
	}
	data, err := run.LoadManifest(".")
	if err != nil {
		return fail(err.Error(), ExitValidation)
	}
	wfName, _ := data["workflow"].(string)
	if wfName == "" {
		return fail("no workflow bound (run shiploom init or shiploom run <workflow>)", ExitValidation)
	}
	budgets, err := run.ParseBudgetFlags(budgetFlags)
	if err != nil {
		return fail(err.Error(), ExitValidation)
	}
	schemasDir, serr := validate.DefaultSchemasDir()
	if serr != nil {
		return fail(serr.Error(), ExitValidation)
	}
	schemas := validate.NewSchemas(schemasDir)
	var order []string
	if wfPath := workflow.FindWorkflow(wfName, "."); wfPath != "" {
		if fm, _ := workflow.LoadWorkflow(schemas, wfPath); fm != nil {
			if steps, ok := fm["steps"].([]any); ok {
				for _, s := range steps {
					if m, ok := s.(map[string]any); ok {
						if id, _ := m["id"].(string); id != "" {
							order = append(order, id)
						}
					}
				}
			}
		}
	}
	pending, err := approvals.PendingApprovals(schemas, ".")
	if err != nil {
		return fail(err.Error(), ExitValidation)
	}
	states, _ := data["steps"].(map[string]any)
	if states == nil {
		states = map[string]any{}
	}
	stateOf := func(sid string) string {
		if m, ok := states[sid].(map[string]any); ok {
			if st, _ := m["state"].(string); st != "" {
				return st
			}
		}
		return ""
	}
	done := 0
	for _, sid := range order {
		if stateOf(sid) == "done" {
			done++
		}
	}
	next := ""
	for _, sid := range order {
		if stateOf(sid) != "done" {
			next = sid
			break
		}
	}
	var nextAny any
	if next != "" {
		nextAny = next
	}
	gateNames := make([]any, 0, len(pending))
	gateStrs := make([]string, 0, len(pending))
	for _, e := range pending {
		gateNames = append(gateNames, e.Gate)
		gateStrs = append(gateStrs, e.Gate)
	}
	position := map[string]any{
		"workflow": wfName, "done": int64(done), "total": int64(len(order)),
		"next": nextAny, "pendingGates": gateNames,
	}
	code, report := run.RunWorkflow(schemas, ".", wfName, nil, nil, budgets, actor)
	if jsonOut {
		doc := map[string]any{
			"ok": code == ExitOK, "exit": int64(code),
			"position": position, "workflow": report.Workflow,
			"paused": report.Paused, "completed": report.Completed,
		}
		advanced := make([]any, 0, len(report.Advanced))
		for _, s := range report.Advanced {
			advanced = append(advanced, s)
		}
		errs := make([]any, 0, len(report.Errors))
		for _, e := range report.Errors {
			errs = append(errs, e)
		}
		warns := make([]any, 0, len(report.Warnings))
		for _, w := range report.Warnings {
			warns = append(warns, w)
		}
		doc["advanced"] = advanced
		doc["errors"] = errs
		doc["warnings"] = warns
		raw, merr := jsoncanon.Marshal(doc)
		if merr != nil {
			return fail(merr.Error(), ExitValidation)
		}
		fmt.Println(string(raw))
		return code
	}
	nextDisplay := next
	if nextDisplay == "" {
		nextDisplay = "complete"
	}
	fmt.Printf("resume %s: %d/%d done, next: %s\n", wfName, done, len(order), nextDisplay)
	if len(gateStrs) > 0 {
		fmt.Printf("pending gates: %s\n", strings.Join(gateStrs, ", "))
	}
	printRunReport(report.Workflow, report.Advanced, report.Completed, report.Paused,
		report.Errors, report.Warnings)
	return code
}

// printRunReport mirrors _print_run_report() for resume.
func printRunReport(workflow string, advanced []string, completed bool, paused any,
	errs []string, warns []map[string]any) {
	if len(advanced) > 0 {
		fmt.Printf("advanced: %s\n", strings.Join(advanced, ", "))
	}
	if completed {
		fmt.Printf("workflow %s complete\n", workflow)
	} else if pausedStr, ok := paused.(string); ok && pausedStr != "" {
		fmt.Printf("paused: %s\n", pausedStr)
	}
	for _, e := range errs {
		fmt.Printf("  fail: %s\n", e)
	}
	for _, w := range warns {
		path, _ := w["path"].(string)
		if path == "" {
			if _, present := w["path"]; !present {
				path = "?"
			}
		}
		fmt.Printf("  warn: %s: %s\n", path, w["message"])
	}
}
