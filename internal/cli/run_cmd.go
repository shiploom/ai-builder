// Command bodies for `run` and `status` (stdlib only, P3).
//
// Flag parsing is hand-rolled like validate.RunValidate: covered
// behaviors are byte-identical to cli/shiploom.py; argparse-only error
// paths (usage wrapping, --help text) stay Python-side until P4.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shiploom/ai-builder/internal/jsoncanon"
	"github.com/shiploom/ai-builder/internal/manifest"
	"github.com/shiploom/ai-builder/internal/run"
	"github.com/shiploom/ai-builder/internal/status"
	"github.com/shiploom/ai-builder/internal/validate"
	"github.com/shiploom/ai-builder/internal/workflow"
)

func init() {
	Register("run", RunRun)
	Register("status", RunStatus)
}

// BootstrapToolRoot mirrors TOOL_ROOT for data lookups: workflows,
// default policy pack, and hooks resolve under the schemas dir's parent.
// Called once at startup; explicit env (SHIPLOOM_CORE_DIR) always wins.
func BootstrapToolRoot() {
	if os.Getenv("SHIPLOOM_CORE_DIR") != "" && workflow.ToolRoot != "" {
		return
	}
	root := ""
	if env := os.Getenv("SHIPLOOM_SCHEMAS"); env != "" {
		abs, err := filepath.Abs(env)
		if err != nil {
			abs = env
		}
		root = filepath.Dir(abs)
	} else {
		candidates := []string{"schemas"}
		if exe, err := os.Executable(); err == nil {
			dir := filepath.Dir(exe)
			if resolved, err := filepath.EvalSymlinks(dir); err == nil {
				dir = resolved
			}
			candidates = append(candidates,
				filepath.Join(dir, "schemas"), filepath.Join(dir, "..", "schemas"))
		}
		for _, c := range candidates {
			if isDir(c) {
				abs, err := filepath.Abs(c)
				if err != nil {
					abs = c
				}
				root = filepath.Dir(abs)
				break
			}
		}
	}
	if root == "" {
		return
	}
	workflow.ToolRoot = root
	if os.Getenv("SHIPLOOM_CORE_DIR") == "" {
		os.Setenv("SHIPLOOM_CORE_DIR", root)
	}
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fail(message string, code int) int {
	fmt.Fprintf(os.Stderr, "shiploom: error: %s\n", message)
	return code
}

// RunStatus implements `status [--json] [path]`.
func RunStatus(args []string) int {
	jsonOut := false
	var positional []string
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "--help", "-h":
			fmt.Println("usage: shiploom status [--json] [path]")
			fmt.Println()
			fmt.Println("Manifest + artifact states + budgets. Exit 0, 2 if the target is missing.")
			return ExitOK
		default:
			if strings.HasPrefix(arg, "-") {
				return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
			}
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
	schemasDir, err := validate.DefaultSchemasDir()
	if err != nil {
		return fail(err.Error(), ExitValidation)
	}
	schemas := validate.NewSchemas(schemasDir)
	payload, errStr := status.StatusOf(schemas, target)
	if errStr != "" {
		printJSON(map[string]any{"ok": false, "error": errStr})
		return ExitValidation
	}
	manifestSection, manifestData := loadManifestSection(target)
	if manifestSection != nil {
		if budgets, ok := manifestData["budgets"]; ok {
			payload.Budgets = budgets
		}
	}
	if jsonOut {
		doc := payload.ToMap()
		doc["ok"] = true
		if manifestSection != nil {
			doc["manifest"] = manifestSection
		}
		printJSON(doc)
		return ExitOK
	}
	fmt.Print(status.FormatHuman(payload))
	if manifestSection != nil {
		steps, _ := manifestSection["steps"].(map[string]any)
		done := 0
		for _, st := range steps {
			if st == "done" {
				done++
			}
		}
		gates, _ := manifestSection["gates"].(map[string]any)
		fmt.Printf("workflow: %s  gates: %d  checkpoints: %d  steps: %d/%d done\n",
			validate.PyStr(manifestSection["workflow"]), len(gates),
			countOf(manifestSection["checkpoints"]), done, len(steps))
	}
	return ExitOK
}

// loadManifestSection mirrors cmd_status()'s manifest merge: absent or
// unreadable manifests silently fall back to file-only status.
func loadManifestSection(target string) (map[string]any, map[string]any) {
	data, err := manifest.Load(target)
	if err != nil {
		return nil, nil
	}
	steps := map[string]any{}
	if raw, ok := data["steps"].(map[string]any); ok {
		for sid, st := range raw {
			if stMap, ok := st.(map[string]any); ok {
				steps[sid], _ = stMap["state"].(string)
			} else {
				steps[sid] = st
			}
		}
	}
	gates, _ := data["gates"].(map[string]any)
	if gates == nil {
		gates = map[string]any{}
	}
	section := map[string]any{
		"workflow":        data["workflow"],
		"workflowVersion": data["workflowVersion"],
		"gates":           gates,
		"checkpoints":     countOf(data["checkpoints"]),
		"steps":           steps,
	}
	return section, data
}

func countOf(v any) int64 {
	switch t := v.(type) {
	case []any:
		return int64(len(t))
	case map[string]any:
		return int64(len(t))
	default:
		return 0
	}
}

// RunRun implements `run workflow [--from/--only/--resume/--budget/--actor/--json]`.
func RunRun(args []string) int {
	var fromStep, onlyStep *string
	var budgetFlags []string
	actor := "human"
	jsonOut := false
	var positional []string
	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "--json":
			jsonOut = true
		case arg == "--resume":
			// Explicit resume is the default behavior; accepted, ignored.
		case arg == "--from" || arg == "--only" || arg == "--actor":
			if i+1 >= len(args) {
				return fail(fmt.Sprintf("argument %s: expected one argument", arg), ExitValidation)
			}
			i++
			val := args[i]
			switch arg {
			case "--from":
				fromStep = &val
			case "--only":
				onlyStep = &val
			case "--actor":
				actor = val
			}
		case strings.HasPrefix(arg, "--budget="):
			budgetFlags = append(budgetFlags, strings.TrimPrefix(arg, "--budget="))
		case arg == "--budget":
			if i+1 >= len(args) {
				return fail("argument --budget: expected one argument", ExitValidation)
			}
			i++
			budgetFlags = append(budgetFlags, args[i])
		case arg == "--help" || arg == "-h":
			fmt.Println("usage: shiploom run [--from STEP] [--only STEP] [--budget K=V] [--actor ACTOR] [--json] workflow")
			fmt.Println()
			fmt.Println("Advance a workflow (idempotent stepper).")
			return ExitOK
		case strings.HasPrefix(arg, "-"):
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		default:
			positional = append(positional, arg)
		}
		i++
	}
	if len(positional) == 0 {
		return fail("the following arguments are required: workflow", ExitValidation)
	}
	if len(positional) > 1 {
		return fail(fmt.Sprintf("unrecognized arguments: %s",
			strings.Join(positional[1:], " ")), ExitValidation)
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
	code, report := run.RunWorkflow(schemas, ".", positional[0], fromStep, onlyStep, budgets, actor)
	if jsonOut {
		doc := map[string]any{
			"ok": code == ExitOK, "exit": int64(code),
			"workflow": report.Workflow, "paused": report.Paused,
			"completed": report.Completed,
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
		printJSON(doc)
		return code
	}
	if len(report.Advanced) > 0 {
		fmt.Printf("advanced: %s\n", strings.Join(report.Advanced, ", "))
	}
	if report.Completed {
		fmt.Printf("workflow %s complete\n", report.Workflow)
	} else if paused, ok := report.Paused.(string); ok && paused != "" {
		fmt.Printf("paused: %s\n", paused)
	}
	for _, e := range report.Errors {
		fmt.Printf("  fail: %s\n", e)
	}
	for _, w := range report.Warnings {
		path, _ := w["path"].(string)
		if path == "" {
			if _, present := w["path"]; !present {
				path = "?"
			}
		}
		fmt.Printf("  warn: %s: %s\n", path, w["message"])
	}
	return code
}

func printJSON(doc map[string]any) {
	out, err := jsoncanon.Marshal(doc)
	if err != nil {
		out = []byte("{}")
	}
	fmt.Println(string(out))
}
