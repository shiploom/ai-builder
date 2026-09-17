// Command bodies for `lock` and `verify` (stdlib only, P4).
//
// Flag parsing is hand-rolled like run/status: covered behaviors are
// byte-identical to cli/shiploom.py; argparse-only error paths (usage
// wrapping, --help text) stay Python-side.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shiploom/ai-builder/internal/gates"
	"github.com/shiploom/ai-builder/internal/jsoncanon"
	"github.com/shiploom/ai-builder/internal/oracle"
	"github.com/shiploom/ai-builder/internal/validate"
)

func init() {
	Register("lock", RunLock)
	Register("verify", RunVerify)
}

// RunLock implements `lock [--check] [--actor ACTOR] [--json]`.
func RunLock(args []string) int {
	check := false
	actor := "human"
	jsonOut := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--check":
			check = true
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
		case arg == "--help" || arg == "-h":
			fmt.Println("usage: shiploom lock [--check] [--actor ACTOR] [--json]")
			fmt.Println()
			fmt.Println("Hash-lock acceptance + oracles (or --check).")
			return ExitOK
		case strings.HasPrefix(arg, "-"):
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		default:
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		}
	}
	if check {
		res := oracle.CheckLock(".")
		if jsonOut {
			printJSON(map[string]any{
				"ok": res.Ok, "errors": entriesToAny(res.Errors),
				"warnings": entriesToAny(res.Warnings),
			})
			return codeFor(res.Ok)
		}
		if res.Ok {
			fmt.Println("acceptance lock: ok")
		} else {
			fmt.Println("acceptance lock: BROKEN")
		}
		for _, e := range res.Errors {
			fmt.Printf("  fail: %s: %s\n", e.Path, e.Message)
		}
		for _, w := range res.Warnings {
			fmt.Printf("  warn: %s: %s\n", w.Path, w.Message)
		}
		return codeFor(res.Ok)
	}
	schemasDir, err := validate.DefaultSchemasDir()
	if err != nil {
		return fail(err.Error(), ExitValidation)
	}
	schemas := validate.NewSchemas(schemasDir)
	res := oracle.Lock(schemas, ".", actor)
	if jsonOut {
		var summaryAny map[string]any
		if len(res.Summary) == 0 {
			summaryAny = map[string]any{}
		} else {
			summaryAny = map[string]any{
				"criteria": int64(res.Summary["criteria"]),
				"files":    int64(res.Summary["files"]),
			}
		}
		printJSON(map[string]any{
			"ok": res.Ok, "errors": entriesToAny(res.Errors),
			"warnings": entriesToAny(res.Warnings),
			"summary":  summaryAny,
		})
		return codeFor(res.Ok)
	}
	if res.Ok {
		fmt.Printf("locked %d criteria from %d files (by %s)\n",
			res.Summary["criteria"], res.Summary["files"], actor)
	} else {
		fmt.Println("lock failed:")
	}
	for _, e := range res.Errors {
		fmt.Printf("  fail: %s: %s\n", e.Path, e.Message)
	}
	for _, w := range res.Warnings {
		path := w.Path
		if path == "" {
			path = "."
		}
		fmt.Printf("  warn: %s: %s\n", path, w.Message)
	}
	return codeFor(res.Ok)
}

// RunVerify implements `verify [--report] [--gates a,b] [--json]`.
func RunVerify(args []string) int {
	report := false
	var gatesFlag *string
	jsonOut := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--report":
			report = true
		case arg == "--json":
			jsonOut = true
		case arg == "--gates":
			if i+1 >= len(args) {
				return fail("argument --gates: expected one argument", ExitValidation)
			}
			i++
			v := args[i]
			gatesFlag = &v
		case strings.HasPrefix(arg, "--gates="):
			v := strings.TrimPrefix(arg, "--gates=")
			gatesFlag = &v
		case arg == "--help" || arg == "-h":
			fmt.Println("usage: shiploom verify [--report] [--gates a,b] [--json]")
			fmt.Println()
			fmt.Println("Run deterministic gates (+ quality table).")
			return ExitOK
		case strings.HasPrefix(arg, "-"):
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		default:
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		}
	}
	var selected []string
	if gatesFlag != nil {
		var list []string
		for _, g := range strings.Split(*gatesFlag, ",") {
			g = strings.TrimSpace(g)
			if g != "" {
				list = append(list, g)
			}
		}
		// Mirror Python: empty selection means full run (None).
		if len(list) > 0 {
			selected = list
		}
	}
	rep := gates.RunGates(".", selected)
	if report {
		out := filepath.Join(".", "verification", "gate-report.json")
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return fail(err.Error(), ExitValidation)
		}
		raw, err := jsoncanon.Marshal(rep.ToMap())
		if err != nil {
			return fail(err.Error(), ExitValidation)
		}
		raw = append(raw, '\n')
		if err := os.WriteFile(out, raw, 0o644); err != nil {
			return fail(err.Error(), ExitValidation)
		}
	}
	if jsonOut {
		printJSON(rep.ToMap())
		return codeFor(rep.Ok)
	}
	fmt.Printf("verify: %s\n", rep.Verdict)
	for _, gid := range verifyGateOrder(selected) {
		g, ok := rep.Gates[gid]
		if !ok {
			continue
		}
		fmt.Printf("  [%-4s] %-10s %s\n", strings.ToUpper(g.Status), gid, g.Detail)
	}
	for _, key := range verifyQualityOrder() {
		val, ok := rep.Quality[key]
		if !ok {
			continue
		}
		fmt.Printf("  quality %-12s %s\n", key, val)
	}
	for _, e := range rep.Errors {
		fmt.Printf("  fail: %s\n", e)
	}
	if report {
		fmt.Println("  wrote verification/gate-report.json")
	}
	return codeFor(rep.Ok)
}

// verifyGateOrder mirrors run_gates() insertion order: fixed GateOrder,
// filtered by selection (preserving GateOrder, not input order).
func verifyGateOrder(selected []string) []string {
	if selected == nil {
		return append([]string{}, gates.GateOrder...)
	}
	keep := map[string]bool{}
	for _, g := range selected {
		keep[g] = true
	}
	var out []string
	for _, g := range gates.GateOrder {
		if keep[g] {
			out = append(out, g)
		}
	}
	return out
}

// verifyQualityOrder mirrors the quality dict insertion order.
func verifyQualityOrder() []string {
	return []string{"compile", "secrets", "tests", "license", "mutation", "determinism"}
}

func entriesToAny(entries []validate.Entry) []any {
	out := make([]any, 0, len(entries))
	for _, e := range entries {
		out = append(out, validate.EntryMap(e))
	}
	return out
}

func codeFor(ok bool) int {
	if ok {
		return ExitOK
	}
	return ExitValidation
}
