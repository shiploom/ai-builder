// Command body for `characterize` (stdlib only, P4).
package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/shiploom/ai-builder/internal/characterize"
	"github.com/shiploom/ai-builder/internal/validate"
)

func init() {
	Register("characterize", RunCharacterize)
}

// RunCharacterize implements
// `characterize (--capture NAME | --diff NAME | --list) [--command CMD]
// [--timeout N] [--actor A] [--json]`.
func RunCharacterize(args []string) int {
	var capture, diff *string
	list := false
	modes := 0
	command := ""
	timeout := 0
	actor := "human"
	jsonOut := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--capture":
			if i+1 >= len(args) {
				return fail("argument --capture: expected one argument", ExitValidation)
			}
			i++
			v := args[i]
			capture = &v
			modes++
		case strings.HasPrefix(arg, "--capture="):
			v := strings.TrimPrefix(arg, "--capture=")
			capture = &v
			modes++
		case arg == "--diff":
			if i+1 >= len(args) {
				return fail("argument --diff: expected one argument", ExitValidation)
			}
			i++
			v := args[i]
			diff = &v
			modes++
		case strings.HasPrefix(arg, "--diff="):
			v := strings.TrimPrefix(arg, "--diff=")
			diff = &v
			modes++
		case arg == "--list":
			list = true
			modes++
		case arg == "--command":
			if i+1 >= len(args) {
				return fail("argument --command: expected one argument", ExitValidation)
			}
			i++
			command = args[i]
		case strings.HasPrefix(arg, "--command="):
			command = strings.TrimPrefix(arg, "--command=")
		case arg == "--timeout":
			if i+1 >= len(args) {
				return fail("argument --timeout: expected one argument", ExitValidation)
			}
			i++
			n, err := parseTimeout(args[i])
			if err != nil {
				return fail(fmt.Sprintf("argument --timeout: %s", err), ExitValidation)
			}
			timeout = n
		case strings.HasPrefix(arg, "--timeout="):
			n, err := parseTimeout(strings.TrimPrefix(arg, "--timeout="))
			if err != nil {
				return fail(fmt.Sprintf("argument --timeout: %s", err), ExitValidation)
			}
			timeout = n
		case arg == "--actor":
			if i+1 >= len(args) {
				return fail("argument --actor: expected one argument", ExitValidation)
			}
			i++
			actor = args[i]
		case strings.HasPrefix(arg, "--actor="):
			actor = strings.TrimPrefix(arg, "--actor=")
		case arg == "--json":
			jsonOut = true
		case arg == "--help" || arg == "-h":
			fmt.Println("usage: shiploom characterize (--capture NAME | --diff NAME | --list) [--command CMD]")
			fmt.Println()
			fmt.Println("Capture/diff behavior snapshots (report-only).")
			return ExitOK
		case strings.HasPrefix(arg, "-"):
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		default:
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		}
	}
	if modes == 0 {
		return fail("one of the arguments --capture --diff --list is required", ExitValidation)
	}
	if modes > 1 {
		return fail("not allowed with argument: only one of --capture --diff --list", ExitValidation)
	}
	if timeout < 0 {
		return fail("--timeout must be >= 0", ExitValidation)
	}
	if list {
		names := characterize.ListSnapshots(".")
		if jsonOut {
			items := make([]any, 0, len(names))
			for _, n := range names {
				items = append(items, n)
			}
			printJSON(map[string]any{"ok": true, "snapshots": items})
			return ExitOK
		}
		joined := strings.Join(names, ", ")
		if joined == "" {
			joined = "none"
		}
		fmt.Printf("snapshots: %s\n", joined)
		return ExitOK
	}
	if capture != nil {
		entry, err := characterize.Capture(".", *capture, command, float64(timeout), actor)
		if err != nil {
			return fail(err.Error(), ExitValidation)
		}
		if jsonOut {
			printJSON(map[string]any{"ok": true, "snapshot": entry})
			return ExitOK
		}
		sha, _ := entry["outputSha"].(string)
		short := sha
		if len(short) > 12 {
			short = short[len(short)-12:]
		}
		fmt.Printf("captured %s: exit %s sha %s\n",
			entry["name"], validate.PyStr(entry["exit"]), short)
		return ExitOK
	}
	result := characterize.Diff(".", *diff, float64(timeout))
	if result.HasError {
		if jsonOut {
			printJSON(map[string]any{"ok": false, "result": map[string]any{
				"name": result.Name, "error": result.Err,
			}})
			return ExitValidation
		}
		fmt.Printf("diff %s error: %s\n", *diff, result.Err)
		return ExitValidation
	}
	if jsonOut {
		lines := make([]any, 0, len(result.UnifiedDiff))
		for _, l := range result.UnifiedDiff {
			lines = append(lines, l)
		}
		printJSON(map[string]any{"ok": true, "result": map[string]any{
			"name": result.Name, "changed": result.Changed,
			"exitChanged": result.ExitChanged, "outputChanged": result.OutputChanged,
			"unifiedDiff": lines, "current": result.Current,
		}})
		return ExitOK
	}
	if !result.Changed {
		fmt.Printf("unchanged: %s\n", result.Name)
		return ExitOK
	}
	fmt.Printf("changed: %s (exitChanged=%s, outputChanged=%s)\n",
		result.Name, validate.PyStr(result.ExitChanged), validate.PyStr(result.OutputChanged))
	for _, line := range result.UnifiedDiff {
		fmt.Printf("  %s\n", strings.TrimRight(line, "\n"))
	}
	return ExitOK
}

// parseTimeout mirrors argparse type=int for --timeout.
func parseTimeout(raw string) (int, error) {
	s := strings.TrimSpace(raw)
	if n, err := strconv.Atoi(s); err == nil {
		return n, nil
	}
	if nos := strings.ReplaceAll(s, "_", ""); nos != s {
		if n, err := strconv.Atoi(nos); err == nil {
			return n, nil
		}
	}
	return 0, fmt.Errorf("invalid int value: %s", validate.PyRepr(raw))
}
