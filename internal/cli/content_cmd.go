// Command bodies for `adapters` and `add` (stdlib only, P4).
package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shiploom/ai-builder/internal/adapters"
	"github.com/shiploom/ai-builder/internal/add"
	"github.com/shiploom/ai-builder/internal/auditlog"
	"github.com/shiploom/ai-builder/internal/validate"
)

func init() {
	Register("adapters", RunAdapters)
	Register("add", RunAdd)
}

// RunAdapters implements `adapters [--list] [--generate H] [--json]`.
func RunAdapters(args []string) int {
	list := false
	var generate *string
	jsonOut := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--list":
			list = true
		case arg == "--json":
			jsonOut = true
		case arg == "--generate":
			if i+1 >= len(args) {
				return fail("argument --generate: expected one argument", ExitValidation)
			}
			i++
			v := args[i]
			generate = &v
		case strings.HasPrefix(arg, "--generate="):
			v := strings.TrimPrefix(arg, "--generate=")
			generate = &v
		case arg == "--help" || arg == "-h":
			fmt.Println("usage: shiploom adapters [--list] [--generate H] [--json]")
			fmt.Println()
			fmt.Println("List / generate harness adapters.")
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
	toolRoot := filepath.Dir(schemasDir)
	if list || generate == nil {
		items := adapters.ListAdapters(toolRoot)
		if jsonOut {
			list := make([]any, 0, len(items))
			for _, a := range items {
				list = append(list, a.ToMap())
			}
			printJSON(map[string]any{"ok": true, "adapters": list})
			return ExitOK
		}
		for _, a := range items {
			outputs := strings.Join(a.Outputs, ", ")
			if outputs == "" {
				outputs = "none"
			}
			fmt.Printf("%-8s v%-6s %s\n    outputs: %s\n",
				a.Name, a.Version, a.Description, outputs)
		}
		return ExitOK
	}
	report, errs := adapters.Generate(toolRoot, *generate, ".")
	if report == nil {
		msg := "adapter failed"
		if len(errs) > 0 {
			msg = errs[0]
		}
		return fail(msg, ExitValidation)
	}
	if errs == nil {
		errs = []string{}
	}
	if jsonOut {
		items := make([]any, 0, len(errs))
		for _, e := range errs {
			items = append(items, e)
		}
		doc := map[string]any{"ok": len(errs) == 0, "errors": items}
		for k, v := range report.ToMap() {
			doc[k] = v
		}
		printJSON(doc)
		return codeFor(len(errs) == 0)
	}
	for _, key := range []string{"created", "updated", "unchanged"} {
		var rels []string
		switch key {
		case "created":
			rels = report.Created
		case "updated":
			rels = report.Updated
		default:
			rels = report.Unchanged
		}
		for _, rel := range rels {
			fmt.Printf("%-9s %s\n", key, rel)
		}
	}
	for _, e := range errs {
		fmt.Printf("  fail: %s\n", e)
	}
	return codeFor(len(errs) == 0)
}

// RunAdd implements `add KIND NAME --from SRC [--tag T] [--force] [--actor A] [--json]`.
func RunAdd(args []string) int {
	var source *string
	var tag *string
	force := false
	actor := "human"
	jsonOut := false
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--from":
			if i+1 >= len(args) {
				return fail("argument --from: expected one argument", ExitValidation)
			}
			i++
			v := args[i]
			source = &v
		case strings.HasPrefix(arg, "--from="):
			v := strings.TrimPrefix(arg, "--from=")
			source = &v
		case arg == "--tag":
			if i+1 >= len(args) {
				return fail("argument --tag: expected one argument", ExitValidation)
			}
			i++
			v := args[i]
			tag = &v
		case strings.HasPrefix(arg, "--tag="):
			v := strings.TrimPrefix(arg, "--tag=")
			tag = &v
		case arg == "--force":
			force = true
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
			fmt.Println("usage: shiploom add KIND NAME --from SRC [--tag T] [--force] [--actor A] [--json]")
			fmt.Println()
			fmt.Println("Install a content pack into the project overlay.")
			return ExitOK
		case strings.HasPrefix(arg, "-"):
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		default:
			positional = append(positional, arg)
		}
	}
	if len(positional) < 2 {
		missing := "kind"
		if len(positional) == 1 {
			missing = "name"
		}
		return fail(fmt.Sprintf("the following arguments are required: %s", missing), ExitValidation)
	}
	if len(positional) > 2 {
		return fail(fmt.Sprintf("unrecognized arguments: %s",
			strings.Join(positional[2:], " ")), ExitValidation)
	}
	if source == nil {
		return fail("the following arguments are required: --from", ExitValidation)
	}
	kind, name := positional[0], positional[1]
	if !isKnownKind(kind) {
		return fail(fmt.Sprintf("argument kind: invalid choice: %s (choose from %s)",
			kind, strings.Join(add.SortedKinds(), ", ")), ExitValidation)
	}
	tagStr := ""
	if tag != nil {
		tagStr = *tag
	}
	schemasDir, err := validate.DefaultSchemasDir()
	if err != nil {
		return fail(err.Error(), ExitValidation)
	}
	schemas := validate.NewSchemas(schemasDir)
	dest, warnings, err := add.Install(schemas, kind, name, *source, ".", tagStr, force)
	if err != nil {
		return fail(err.Error(), ExitValidation)
	}
	if _, err := auditlog.Append(".", actor, "content.add",
		fmt.Sprintf("%s:%s", kind, name), "-"); err != nil {
		return fail(err.Error(), ExitValidation)
	}
	if jsonOut {
		items := make([]any, 0, len(warnings))
		for _, w := range warnings {
			items = append(items, w)
		}
		printJSON(map[string]any{
			"ok": true, "kind": kind, "name": name,
			"dest": dest, "warnings": items,
		})
		return ExitOK
	}
	fmt.Printf("installed %s %s -> %s\n", kind, name, dest)
	for _, w := range warnings {
		fmt.Printf("  warn: %s\n", w)
	}
	return ExitOK
}

func isKnownKind(kind string) bool {
	_, ok := add.Kinds[kind]
	return ok
}
