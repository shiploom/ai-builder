// Command body for `doctor` (stdlib only, P4).
package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shiploom/ai-builder/internal/doctor"
	"github.com/shiploom/ai-builder/internal/validate"
)

func init() {
	Register("doctor", RunDoctor)
}

// RunDoctor implements `doctor [--json]`.
func RunDoctor(args []string) int {
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "--help", "-h":
			fmt.Println("usage: shiploom doctor [--json]")
			fmt.Println()
			fmt.Println("Harness + MCP + toolchain compat (offline).")
			return ExitOK
		default:
			if strings.HasPrefix(arg, "-") {
				return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
			}
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		}
	}
	schemasDir, err := validate.DefaultSchemasDir()
	if err != nil {
		return fail(err.Error(), ExitValidation)
	}
	schemas := validate.NewSchemas(schemasDir)
	toolRoot := filepath.Dir(schemasDir)
	report := doctor.RunChecks(schemas, schemasDir, toolRoot, ".")
	if jsonOut {
		printJSON(report.ToMap())
		return codeForExit(report.Ok)
	}
	fmt.Print(report.FormatHuman())
	return codeForExit(report.Ok)
}

// codeForExit maps doctor health to exit codes: 0 clean, 5 on failure
// (harness mismatch), mirroring cmd_doctor().
func codeForExit(ok bool) int {
	if ok {
		return ExitOK
	}
	return ExitHarness
}
