// Package cli implements the shiploom command surface (stdlib only).
//
// P1 covers dispatch, --version parity, help, and exit codes. Command
// bodies port in P2-P4; until then they fail closed with exit 2 and a
// pointer to the Python fallback.
package cli

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/shiploom/ai-builder/internal/version"
)

// Exit codes mirror the Python CLI contract (AGENTS.md).
const (
	ExitOK         = 0
	ExitValidation = 2
	ExitPolicy     = 3
	ExitBudget     = 4
	ExitHarness    = 5
)

// Command is one subcommand. Help strings mirror cli/shiploom.py
// add_parser help texts; Run arrives per command in P2-P4.
type Command struct {
	Name string
	Help string
	Run  func(args []string) int
}

// Table lists every subcommand the Python CLI accepts.
var Table = []Command{
	{"install", "copy + verify core (offline MVP)", nil},
	{"init", "scaffold ./.shiploom/ + seed starter artifact", nil},
	{"validate", "schemas + frontmatter + links", nil},
	{"status", "manifest + artifact states + budgets", nil},
	{"doctor", "harness + MCP + toolchain compat (offline)", nil},
	{"audit", "verify + export the audit log", nil},
	{"lock", "hash-lock acceptance + oracles (or --check)", nil},
	{"run", "advance a workflow (idempotent stepper)", nil},
	{"approve", "record a human-approval gate (policy enforced by `run` gates; deny exits 3)", nil},
	{"verify", "run deterministic gates (+ quality table)", nil},
	{"adapters", "list / generate harness adapters", nil},
	{"trace", "show trace subgraph for an artifact id", nil},
	{"budget", "show or set manifest budget limits", nil},
	{"resume", "report position and advance the bound workflow", nil},
	{"approvals", "list pending human gates", nil},
	{"add", "install a content pack into the project overlay", nil},
	{"conformance", "deterministic harness-conformance checks", nil},
	{"pin", "write/check ./.shiploom/lock.json reproducibility pin", nil},
	{"upgrade", "move project to the tool core version (backup + rollback)", nil},
	{"characterize", "capture/diff behavior snapshots (report-only)", nil},
}

// Main is the CLI entry point. It mirrors the Python surface contract:
// --version prints versions, bare invocation errors exit 2, unknown
// commands exit 2. Returns the process exit code.
func Main(argv []string) int {
	if len(argv) == 0 {
		fmt.Fprintln(os.Stderr, "shiploom: error: a command is required (try --help)")
		return ExitValidation
	}
	if argv[0] == "--version" {
		fmt.Printf("shiploom %s (core %s, go %s)\n",
			version.CoreVersion, version.CoreVersion, runtime.Version())
		return ExitOK
	}
	if argv[0] == "--help" || argv[0] == "-h" {
		printHelp()
		return ExitOK
	}
	for _, cmd := range Table {
		if argv[0] == cmd.Name {
			if cmd.Run == nil {
				fmt.Fprintf(os.Stderr,
					"shiploom: error: command '%s' is not yet ported to the Go binary\n",
					cmd.Name)
				return ExitValidation
			}
			return cmd.Run(argv[1:])
		}
	}
	choices := make([]string, 0, len(Table))
	for _, cmd := range Table {
		choices = append(choices, "'"+cmd.Name+"'")
	}
	fmt.Fprintf(os.Stderr,
		"shiploom: error: argument command: invalid choice: '%s' (choose from %s)\n",
		argv[0], strings.Join(choices, ", "))
	return ExitValidation
}

func printHelp() {
	fmt.Println("usage: shiploom [--version] <command> [options]")
	fmt.Println()
	fmt.Println("Shiploom Core CLI (Go port, stdlib only).")
	fmt.Println()
	fmt.Println("commands:")
	for _, cmd := range Table {
		fmt.Printf("  %-12s %s\n", cmd.Name, cmd.Help)
	}
}
