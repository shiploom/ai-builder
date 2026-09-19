// Command bodies for `install` and `init` (stdlib only, P4).
//
// With these, the full Python CLI surface is ported: every subcommand in
// cli/shiploom.py build_parser() has a Go implementation.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shiploom/ai-builder/internal/auditlog"
	"github.com/shiploom/ai-builder/internal/jsoncanon"
	"github.com/shiploom/ai-builder/internal/manifest"
	"github.com/shiploom/ai-builder/internal/validate"
)

func init() {
	Register("install", RunInstall)
	Register("init", RunInit)
}

// SupportedHarnesses mirrors SUPPORTED_HARNESSES (argparse choices order
// is alphabetical in help; membership is what matters here).
var SupportedHarnesses = []string{"claude", "opencode", "auto"}

// RunInstall implements `install [--global|--local] [--version V]`.
func RunInstall(args []string) int {
	toGlobal := false
	toLocal := false
	var version *string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--global":
			toGlobal = true
		case arg == "--local":
			toLocal = true
		case arg == "--version":
			if i+1 >= len(args) {
				return fail("argument --version: expected one argument", ExitValidation)
			}
			i++
			v := args[i]
			version = &v
		case strings.HasPrefix(arg, "--version="):
			v := strings.TrimPrefix(arg, "--version=")
			version = &v
		case arg == "--help" || arg == "-h":
			fmt.Println("usage: shiploom install [--global|--local] [--version V]")
			fmt.Println()
			fmt.Println("Copy + verify core (offline MVP).")
			return ExitOK
		case strings.HasPrefix(arg, "-"):
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		default:
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		}
	}
	// Mirror main(): --global/--local conflict fails; neither defaults
	// to global.
	if toGlobal && toLocal {
		return fail("choose one of --global / --local", ExitValidation)
	}
	if !toGlobal && !toLocal {
		toGlobal = true
	}
	schemasDir, err := validate.DefaultSchemasDir()
	if err != nil {
		return fail(err.Error(), ExitValidation)
	}
	toolRoot := filepath.Dir(schemasDir)
	coreRaw, err := os.ReadFile(filepath.Join(toolRoot, "core", "VERSION"))
	if err != nil {
		return fail(fmt.Sprintf("unreadable tool version: %s", err), ExitValidation)
	}
	packaged := strings.TrimSpace(string(coreRaw))
	want := packaged
	if version != nil {
		want = *version
	}
	if want != packaged {
		return fail(fmt.Sprintf("version %s != packaged core %s (offline MVP: no download)",
			want, packaged), ExitValidation)
	}
	var dest string
	if toGlobal {
		home, err := os.UserHomeDir()
		if err != nil {
			return fail(err.Error(), ExitValidation)
		}
		dest = filepath.Join(home, ".shiploom", "core", want)
	} else {
		// Mirror Path.cwd(): resolve symlinks (/tmp -> /private/tmp).
		cwd, err := os.Getwd()
		if err != nil {
			return fail(err.Error(), ExitValidation)
		}
		if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
			cwd = resolved
		}
		dest = filepath.Join(cwd, ".shiploom", "core", want)
	}
	schemas := validate.NewSchemas(schemasDir)
	coreErrs, coreWarns, _ := validate.ValidatePath(schemas, filepath.Join(toolRoot, "core"), true)
	exampleErrs, _, _ := validate.ValidatePath(schemas, filepath.Join(toolRoot, "examples"), true)
	errs := append(append([]validate.Entry{}, coreErrs...), exampleErrs...)
	if len(errs) > 0 {
		fmt.Fprintln(os.Stderr, "shiploom: packaged core fails strict validation:")
		top := errs
		if len(top) > 10 {
			top = top[:10]
		}
		for _, e := range top {
			fmt.Fprintf(os.Stderr, "  %s: %s\n", e.Path, e.Message)
		}
		return ExitValidation
	}
	// validators/ left with the Python implementation in v1.3.0.
	for _, sub := range []string{"core", "schemas"} {
		if err := copyTree(filepath.Join(toolRoot, sub), filepath.Join(dest, sub)); err != nil {
			return fail(err.Error(), ExitValidation)
		}
	}
	receipt := map[string]any{
		"version": want, "installedAt": manifest.Utcnow(),
		"source": toolRoot, "warnings": int64(len(coreWarns)),
	}
	raw, err := jsoncanon.Marshal(receipt)
	if err != nil {
		return fail(err.Error(), ExitValidation)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fail(err.Error(), ExitValidation)
	}
	if err := os.WriteFile(filepath.Join(dest, "receipt.json"), append(raw, '\n'), 0o644); err != nil {
		return fail(err.Error(), ExitValidation)
	}
	fmt.Printf("installed shiploom core %s -> %s\n", want, dest)
	return ExitOK
}

// copyTree mirrors shutil.copytree(dirs_exist_ok, ignore __pycache__).
func copyTree(src, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == "__pycache__" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dest, rel), 0o755)
		}
		if strings.Contains(rel, "__pycache__") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, raw, info.Mode().Perm())
	})
}

// RunInit implements `init [--green|--existing] [--harness H] [--stack S] [--force]`.
func RunInit(args []string) int {
	green := false
	existing := false
	harness := "auto"
	var stack *string
	force := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--green":
			green = true
		case arg == "--existing":
			existing = true
		case arg == "--force":
			force = true
		case arg == "--harness":
			if i+1 >= len(args) {
				return fail("argument --harness: expected one argument", ExitValidation)
			}
			i++
			harness = args[i]
		case strings.HasPrefix(arg, "--harness="):
			harness = strings.TrimPrefix(arg, "--harness=")
		case arg == "--stack":
			if i+1 >= len(args) {
				return fail("argument --stack: expected one argument", ExitValidation)
			}
			i++
			v := args[i]
			stack = &v
		case strings.HasPrefix(arg, "--stack="):
			v := strings.TrimPrefix(arg, "--stack=")
			stack = &v
		case arg == "--help" || arg == "-h":
			fmt.Println("usage: shiploom init [--green|--existing] [--harness H] [--stack S] [--force]")
			fmt.Println()
			fmt.Println("Scaffold ./.shiploom/ + seed starter artifact.")
			return ExitOK
		case strings.HasPrefix(arg, "-"):
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		default:
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		}
	}
	if green && existing {
		return fail("argument --existing: not allowed with argument --green", ExitValidation)
	}
	_ = green
	isExisting := existing
	supported := false
	for _, h := range SupportedHarnesses {
		if harness == h {
			supported = true
		}
	}
	if !supported {
		fmt.Fprintf(os.Stderr, "shiploom: error: unsupported harness %s (choose claude|opencode|auto)\n",
			validate.PyRepr(harness))
		return ExitHarness
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fail(err.Error(), ExitValidation)
	}
	// Mirror Path.cwd(): resolve symlinks (/tmp -> /private/tmp) so
	// reported paths match CPython byte-for-byte.
	if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = resolved
	}
	dot := filepath.Join(cwd, ".shiploom")
	if _, err := os.Stat(dot); err == nil && !force {
		return fail(fmt.Sprintf("%s exists (use --force to re-initialize)", dot), ExitValidation)
	}
	schemasDir, err := validate.DefaultSchemasDir()
	if err != nil {
		return fail(err.Error(), ExitValidation)
	}
	toolRoot := filepath.Dir(schemasDir)
	if err := os.MkdirAll(dot, 0o755); err != nil {
		return fail(err.Error(), ExitValidation)
	}
	oracle := filepath.Join(dot, ".oracle")
	if err := os.MkdirAll(oracle, 0o755); err != nil {
		return fail(err.Error(), ExitValidation)
	}
	_ = os.Chmod(oracle, 0o700)
	if err := os.WriteFile(filepath.Join(dot, ".gitignore"), []byte(".oracle/\n"), 0o644); err != nil {
		return fail(err.Error(), ExitValidation)
	}
	coreRaw, err := os.ReadFile(filepath.Join(toolRoot, "core", "VERSION"))
	if err != nil {
		return fail(fmt.Sprintf("unreadable tool version: %s", err), ExitValidation)
	}
	version := strings.TrimSpace(string(coreRaw))
	workflow := manifest.DefaultWorkflow(!isExisting)
	var stackAny any
	if stack != nil {
		stackAny = *stack
	}
	var adapterTargets []any
	if harness == "auto" {
		adapterTargets = []any{"claude", "opencode"}
	} else {
		adapterTargets = []any{harness}
	}
	config := map[string]any{
		"coreVersion": version, "harness": harness, "stack": stackAny,
		"workflow": workflow,
		// Budget literals so canonical JSON emits 800000/25.0/8.0.
		"budgets": map[string]any{
			"tokens":     int64(800000),
			"spendUSD":   json.Number("25.0"),
			"wallClockH": json.Number("8.0"),
		},
		"policyPack":     "default",
		"adapterTargets": adapterTargets,
		"mcpRegistry":    "./.shiploom/mcp-registry.json",
	}
	if err := writeJSONFile(filepath.Join(dot, "config.json"), config); err != nil {
		return fail(err.Error(), ExitValidation)
	}
	if err := writeJSONFile(filepath.Join(dot, "mcp-registry.json"),
		map[string]any{"capabilities": map[string]any{}}); err != nil {
		return fail(err.Error(), ExitValidation)
	}
	if _, err := manifest.Save(cwd, manifest.Genesis(version, workflow, nil)); err != nil {
		return fail(err.Error(), ExitValidation)
	}
	if _, err := auditlog.InitLog(cwd); err != nil {
		return fail(err.Error(), ExitValidation)
	}
	if _, err := auditlog.Append(cwd, "system", "project.init", ".", "default"); err != nil {
		return fail(err.Error(), ExitValidation)
	}
	seedNote := ""
	if !isExisting {
		seedNote = seedFile(toolRoot, cwd, "idea.md", "idea.md", force)
	} else {
		seedNote = seedFile(toolRoot, cwd, "brownfield/repo-map.md", "brownfield/repo-map.md", force)
	}
	kind := "greenfield"
	if isExisting {
		kind = "brownfield"
	}
	fmt.Printf("initialized %s project in %s\n", kind, cwd)
	fmt.Printf("  workflow: %s\n  harness: %s\n  %s\n", workflow, harness, seedNote)
	fmt.Println("next: edit idea.md, then run `shiploom validate --strict .`")
	return ExitOK
}

// seedFile mirrors _seed_file().
func seedFile(toolRoot, root, rel, templateRel string, force bool) string {
	dest := filepath.Join(root, rel)
	if _, err := os.Stat(dest); err == nil && !force {
		return fmt.Sprintf("kept %s (exists)", rel)
	}
	src := filepath.Join(toolRoot, "core", "artifacts-templates", templateRel)
	raw, err := os.ReadFile(src)
	if err != nil {
		return fmt.Sprintf("kept %s (exists)", rel)
	}
	_ = os.MkdirAll(filepath.Dir(dest), 0o755)
	_ = os.WriteFile(dest, raw, 0o644)
	return fmt.Sprintf("seeded %s", rel)
}
