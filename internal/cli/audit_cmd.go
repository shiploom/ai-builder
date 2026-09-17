// Command body for `audit` (stdlib only, P4).
package cli

import (
	"fmt"
	"strings"

	"github.com/shiploom/ai-builder/internal/auditlog"
)

func init() {
	Register("audit", RunAudit)
}

// RunAudit implements `audit [--export json|md]`.
func RunAudit(args []string) int {
	var exportFmt *string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--export":
			if i+1 >= len(args) {
				return fail("argument --export: expected one argument", ExitValidation)
			}
			i++
			v := args[i]
			exportFmt = &v
		case strings.HasPrefix(arg, "--export="):
			v := strings.TrimPrefix(arg, "--export=")
			exportFmt = &v
		case arg == "--help" || arg == "-h":
			fmt.Println("usage: shiploom audit [--export json|md]")
			fmt.Println()
			fmt.Println("Verify + export the audit log.")
			return ExitOK
		case strings.HasPrefix(arg, "-"):
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		default:
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		}
	}
	if exportFmt != nil && *exportFmt != "json" && *exportFmt != "md" {
		return fail(fmt.Sprintf("argument --export: invalid choice: %s (choose from 'json', 'md')",
			*exportFmt), ExitValidation)
	}
	ok, errors := auditlog.Verify(".")
	mode := ""
	if exportFmt != nil {
		mode = *exportFmt
	}
	switch mode {
	case "json":
		entries, err := auditlog.ReadAll(".")
		if err != nil {
			return fail(err.Error(), ExitValidation)
		}
		list := make([]any, 0, len(entries))
		for _, e := range entries {
			m := map[string]any{}
			for k, v := range e {
				m[k] = v
			}
			list = append(list, m)
		}
		errs := make([]any, 0, len(errors))
		for _, e := range errors {
			errs = append(errs, e)
		}
		printJSON(map[string]any{"ok": ok, "errors": errs, "entries": list})
		return codeFor(ok)
	case "md":
		doc, err := auditlog.ExportMd(".")
		if err != nil {
			return fail(err.Error(), ExitValidation)
		}
		fmt.Print(doc)
		return codeFor(ok)
	default:
		entries, err := auditlog.ReadAll(".")
		if err != nil {
			return fail(err.Error(), ExitValidation)
		}
		chain := "ok"
		if !ok {
			chain = "BROKEN"
		}
		fmt.Printf("audit: %d entries, chain %s\n", len(entries), chain)
		for _, e := range errors {
			fmt.Printf("  %s\n", e)
		}
		return codeFor(ok)
	}
}
