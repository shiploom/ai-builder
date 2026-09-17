// Command body for `approvals` (stdlib only, P3).
package cli

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/shiploom/ai-builder/internal/approvals"
	"github.com/shiploom/ai-builder/internal/validate"
)

func init() {
	Register("approvals", RunApprovals)
}

// RunApprovals implements `approvals [--watch] [--interval S] [--json]`.
func RunApprovals(args []string) int {
	watch := false
	interval := 5.0
	jsonOut := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--watch":
			watch = true
		case arg == "--json":
			jsonOut = true
		case arg == "--interval":
			if i+1 >= len(args) {
				return fail("argument --interval: expected one argument", ExitValidation)
			}
			i++
			f, err := parseInterval(args[i])
			if err != nil {
				return fail(fmt.Sprintf("argument --interval: %s", err), ExitValidation)
			}
			interval = f
		case strings.HasPrefix(arg, "--interval="):
			f, err := parseInterval(strings.TrimPrefix(arg, "--interval="))
			if err != nil {
				return fail(fmt.Sprintf("argument --interval: %s", err), ExitValidation)
			}
			interval = f
		case arg == "--help" || arg == "-h":
			fmt.Println("usage: shiploom approvals [--watch] [--interval S] [--json]")
			fmt.Println()
			fmt.Println("List pending human gates.")
			return ExitOK
		case strings.HasPrefix(arg, "-"):
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		default:
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		}
	}
	if interval <= 0 {
		return fail("--interval must be positive", ExitValidation)
	}
	schemasDir, err := validate.DefaultSchemasDir()
	if err != nil {
		return fail(err.Error(), ExitValidation)
	}
	schemas := validate.NewSchemas(schemasDir)
	show := func() ([]approvals.Entry, int) {
		entries, err := approvals.PendingApprovals(schemas, ".")
		if err != nil {
			return nil, fail(err.Error(), ExitValidation)
		}
		if jsonOut {
			list := make([]any, 0, len(entries))
			for _, e := range entries {
				list = append(list, e.ToMap())
			}
			printJSON(map[string]any{"ok": true, "pending": list})
			return entries, ExitOK
		}
		if len(entries) == 0 {
			fmt.Println("no pending approvals")
			return entries, ExitOK
		}
		for _, e := range entries {
			message := e.Message
			if message == "" {
				message = "awaiting human decision"
			}
			fmt.Printf("%-12s %-14s %-8s attempts=%d %s\n",
				e.Gate, e.Kind, e.State, e.Attempts, message)
		}
		fmt.Println("note: full rationale cards need manifest rationale (schema follow-up)")
		return entries, ExitOK
	}
	entries, code := show()
	if code != ExitOK || jsonOut {
		return code
	}
	if watch && len(entries) > 0 {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		for {
			select {
			case <-sig:
				return ExitOK
			case <-time.After(time.Duration(interval * float64(time.Second))):
			}
			entries, code = show()
			if code != ExitOK || len(entries) == 0 {
				return code
			}
		}
	}
	return ExitOK
}

// parseInterval mirrors argparse type=float for --interval.
func parseInterval(raw string) (float64, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		if nos := strings.ReplaceAll(strings.TrimSpace(raw), "_", ""); nos != strings.TrimSpace(raw) {
			if f2, err2 := strconv.ParseFloat(nos, 64); err2 == nil {
				return f2, nil
			}
		}
		return 0, fmt.Errorf("invalid float value: %s", validate.PyRepr(raw))
	}
	return f, nil
}
