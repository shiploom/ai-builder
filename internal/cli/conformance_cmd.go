// Command body for `conformance` (stdlib only, P4).
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shiploom/ai-builder/internal/conformance"
	"github.com/shiploom/ai-builder/internal/jsoncanon"
	"github.com/shiploom/ai-builder/internal/validate"
)

func init() {
	Register("conformance", RunConformance)
}

// RunConformance implements `conformance [--harness H] [--record] [--json]`.
func RunConformance(args []string) int {
	harness := "all"
	record := false
	jsonOut := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--record":
			record = true
		case arg == "--json":
			jsonOut = true
		case arg == "--harness":
			if i+1 >= len(args) {
				return fail("argument --harness: expected one argument", ExitValidation)
			}
			i++
			harness = args[i]
		case strings.HasPrefix(arg, "--harness="):
			harness = strings.TrimPrefix(arg, "--harness=")
		case arg == "--help" || arg == "-h":
			fmt.Println("usage: shiploom conformance [--harness H] [--record] [--json]")
			fmt.Println()
			fmt.Println("Deterministic harness-conformance checks.")
			return ExitOK
		case strings.HasPrefix(arg, "-"):
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		default:
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		}
	}
	schemasDir, err := validate.DefaultSchemasDir()
	if err != nil {
		return fail(err.Error(), ExitValidation)
	}
	schemas := validate.NewSchemas(schemasDir)
	toolRoot := filepath.Dir(schemasDir)
	if jsonOut {
		if harness == "all" {
			ok, results := conformance.RunAll(toolRoot, schemas, record)
			mapped := map[string]any{}
			for name, report := range results {
				mapped[name] = report.ToMap()
			}
			printJSON(map[string]any{"ok": ok, "results": mapped})
			return codeFor(ok)
		}
		ok, report := conformance.CheckHarness(toolRoot, harness, schemas)
		if record && ok {
			recordSingle(toolRoot, report)
		}
		doc := map[string]any{"ok": ok}
		for k, v := range report.ToMap() {
			doc[k] = v
		}
		printJSON(doc)
		return codeFor(ok)
	}
	if harness == "all" {
		ok, results := conformance.RunAll(toolRoot, schemas, record)
		for _, name := range conformance.AllHarnesses {
			report := results[name]
			verdict := "PASS"
			if !report.Ok {
				verdict = "FAIL"
			}
			fmt.Printf("%-8s %s (%d fail)\n", name, verdict, report.Failures)
			for _, c := range report.Checks {
				if c.Status == "fail" {
					fmt.Printf("  fail: %s: %s\n", c.Name, c.Detail)
				}
			}
		}
		return codeFor(ok)
	}
	ok, report := conformance.CheckHarness(toolRoot, harness, schemas)
	if record && ok {
		recordSingle(toolRoot, report)
	}
	verdict := "PASS"
	if !ok {
		verdict = "FAIL"
	}
	fmt.Printf("%s: %s\n", harness, verdict)
	for _, c := range report.Checks {
		mark := "x"
		if c.Status == "pass" {
			mark = "+"
		}
		fmt.Printf("  [%s] %-24s %s\n", mark, c.Name, c.Detail)
	}
	for _, e := range report.Errors {
		fmt.Printf("  fail: %s\n", e)
	}
	return codeFor(ok)
}

// recordSingle mirrors the single-harness --record write (same bytes as
// RunAll's per-harness record).
func recordSingle(toolRoot string, report conformance.Report) {
	dir := filepath.Join(toolRoot, "tests", "conformance", "_records")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	raw, err := jsoncanon.Marshal(report.ToMap())
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, report.Harness+".json"), append(raw, '\n'), 0o644)
}
