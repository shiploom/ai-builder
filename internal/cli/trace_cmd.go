// Command body for `trace` (stdlib only, P4).
//
// Flag parsing is hand-rolled like lock/verify: covered behaviors are
// byte-identical to cli/shiploom.py; argparse-only error paths stay
// Python-side.
package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/shiploom/ai-builder/internal/trace"
	"github.com/shiploom/ai-builder/internal/validate"
)

func init() {
	Register("trace", RunTrace)
}

// RunTrace implements `trace [--json] id [path]`.
func RunTrace(args []string) int {
	jsonOut := false
	var positional []string
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "--help", "-h":
			fmt.Println("usage: shiploom trace [--json] id [path]")
			fmt.Println()
			fmt.Println("Show trace subgraph for an artifact id.")
			return ExitOK
		default:
			if strings.HasPrefix(arg, "-") {
				return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
			}
			positional = append(positional, arg)
		}
	}
	if len(positional) == 0 {
		return fail("the following arguments are required: id", ExitValidation)
	}
	if len(positional) > 1 {
		// Second positional is the path; more is an error like argparse.
		if len(positional) > 2 {
			return fail(fmt.Sprintf("unrecognized arguments: %s",
				strings.Join(positional[2:], " ")), ExitValidation)
		}
	}
	id := positional[0]
	target := "."
	if len(positional) == 2 {
		target = positional[1]
	}
	schemasDir, err := validate.DefaultSchemasDir()
	if err != nil {
		return fail(err.Error(), ExitValidation)
	}
	schemas := validate.NewSchemas(schemasDir)
	index, errs, warnings := trace.BuildTrace(schemas, target, false)
	if len(errs) > 0 {
		for _, e := range errs {
			path := e.Path
			if path == "" {
				path = "?"
			}
			fmt.Printf("  fail: %s: %s\n", path, e.Message)
		}
		return ExitValidation
	}
	links, ok := index[id]
	if !ok {
		return fail(fmt.Sprintf("unknown id %s in trace index", validate.PyRepr(id)), ExitValidation)
	}
	// Incoming references: sorted source ids, sorted relations.
	type incoming struct {
		from     string
		relation string
	}
	var refs []incoming
	sources := make([]string, 0, len(index))
	for src := range index {
		sources = append(sources, src)
	}
	sort.Strings(sources)
	for _, src := range sources {
		rels := make([]string, 0, len(index[src]))
		for rel := range index[src] {
			rels = append(rels, rel)
		}
		sort.Strings(rels)
		for _, rel := range rels {
			for _, tgt := range index[src][rel] {
				if tgt == id {
					refs = append(refs, incoming{src, rel})
					break
				}
			}
		}
	}
	if jsonOut {
		linksAny := map[string]any{}
		for rel, targets := range links {
			list := make([]any, 0, len(targets))
			for _, t := range targets {
				list = append(list, t)
			}
			linksAny[rel] = list
		}
		inAny := make([]any, 0, len(refs))
		for _, r := range refs {
			inAny = append(inAny, map[string]any{"from": r.from, "relation": r.relation})
		}
		printJSON(map[string]any{
			"ok": true, "id": id, "links": linksAny,
			"referencedBy": inAny, "warnings": entriesToAny(warnings),
		})
		return ExitOK
	}
	fmt.Printf("%s:\n", id)
	rels := make([]string, 0, len(links))
	for rel := range links {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	for _, rel := range rels {
		joined := strings.Join(links[rel], ", ")
		if joined == "" {
			joined = "-"
		}
		fmt.Printf("  %s: %s\n", rel, joined)
	}
	fmt.Println("  referenced by:")
	if len(refs) > 0 {
		for _, r := range refs {
			fmt.Printf("    %s (%s)\n", r.from, r.relation)
		}
	} else {
		fmt.Println("    -")
	}
	return ExitOK
}
