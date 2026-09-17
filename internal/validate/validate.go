package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/shiploom/ai-builder/internal/jsoncanon"
)

// ExcludeDirs mirrors Python EXCLUDE_DIRS.
var ExcludeDirs = map[string]bool{
	".git": true, ".venv": true, ".conda": true, "__pycache__": true,
	"node_modules": true, "dist": true, "build": true, ".oracle": true,
	".validator-cache": true,
}

// ExcludeFiles mirrors Python EXCLUDE_FILES.
var ExcludeFiles = map[string]bool{"trace.json": true}

// VagueTerms mirrors Python VAGUE_TERMS (order matters for messages).
var VagueTerms = []string{
	"fast", "secure", "user-friendly", "user friendly", "scalable", "robust",
	"intuitive", "seamless", "high-performance", "high performance",
	"blazing", "easy", "simple", "quickly", "best", "state-of-the-art",
	"military-grade", "bank-grade",
}

var vagueRes = func() []*regexp.Regexp {
	out := make([]*regexp.Regexp, 0, len(VagueTerms))
	for _, t := range VagueTerms {
		out = append(out, regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(t)+`\b`))
	}
	return out
}()

// ----------------------------------------------------------------------------
// Per-kind semantic checks
// ----------------------------------------------------------------------------

func vagueTermsIn(statement string) []string {
	var out []string
	for i, rx := range vagueRes {
		if rx.MatchString(statement) {
			out = append(out, VagueTerms[i])
		}
	}
	return out
}

func strField(obj map[string]any, key string) string {
	s, _ := obj[key].(string)
	return s
}

func checkAcceptanceSemantics(obj map[string]any, strict bool) (errs, warns []string) {
	errs = []string{}
	warns = []string{}
	var hov map[string]any
	if h, ok := obj["howToVerify"].(map[string]any); ok {
		hov = h
	}
	htype, _ := hov["type"].(string)
	command, _ := hov["command"].(string)
	expect, _ := hov["expect"].(string)
	if (htype == "script" || htype == "http" || htype == "browser") && command == "" {
		errs = append(errs, fmt.Sprintf("howToVerify.command required for type %s", pyRepr(htype)))
	}
	if vague := vagueTermsIn(strField(obj, "statement")); len(vague) > 0 {
		measurable := expect != "" && len([]rune(expect)) >= 10 && htype != "human"
		msg := fmt.Sprintf("vague term(s) %s without measurable howToVerify",
			strings.Join(vague, ", "))
		switch {
		case measurable && !strict:
			warns = append(warns, msg+" (strict will fail)")
		case measurable && strict:
			warns = append(warns, msg+" (measurable verifier present)")
		case strict:
			errs = append(errs, msg)
		default:
			warns = append(warns, msg+" (use --strict to enforce)")
		}
	}
	if oracle, ok := obj["oracleRef"].(string); ok {
		if strings.HasPrefix(oracle, "/") {
			errs = append(errs, "oracleRef must be relative, got absolute path")
		}
		for _, part := range strings.Split(oracle, "/") {
			if part == ".." {
				errs = append(errs, "oracleRef must not contain '..'")
				break
			}
		}
	}
	return errs, warns
}

func checkSkillSemantics(fm map[string]any, filePath, body string) (errs, warns []string) {
	errs = []string{}
	name, _ := fm["name"].(string)
	parent := filepath.Base(filepath.Dir(filePath))
	if name != parent {
		errs = append(errs, fmt.Sprintf("skill name %s must equal directory name %s",
			pyRepr(name), pyRepr(parent)))
	}
	if lineCount(body) > 500 {
		errs = append(errs, fmt.Sprintf("skill body %d lines exceeds 500-line limit", lineCount(body)))
	}
	return errs, warns
}

func lineCount(body string) int {
	if body == "" {
		return 0
	}
	n := strings.Count(body, "\n")
	if !strings.HasSuffix(body, "\n") {
		n++
	}
	// Python "".splitlines() == [] (0 lines) but "a".splitlines() == ["a"] (1).
	// "\n".splitlines() == [""] (1 line!). Mirror: count "\n"s, plus one
	// unless the body is empty or ends with exactly one trailing newline
	// after content... precisely: len(splitlines) = newlines + (1 if the
	// text doesn't end with \n or is exactly "\n"-terminated-nonempty?).
	// Recompute directly: split and count like Python for \n texts.
	lines := strings.Split(body, "\n")
	if body[len(body)-1] == '\n' {
		return len(lines) - 1 + 1 // trailing "" element still counts as a line
	}
	return len(lines)
}

func checkWorkflowSemantics(fm map[string]any) (errs, warns []string) {
	errs = []string{}
	var steps []any
	if s, ok := fm["steps"].([]any); ok {
		steps = s
	}
	seen := map[string]bool{}
	for _, step := range steps {
		obj, ok := step.(map[string]any)
		if !ok {
			continue
		}
		sid, _ := obj["id"].(string)
		// Divergence note: Python set() detects duplicates for any
		// hashable id and crashes on unhashable ones; schemas require
		// string ids, so string-only detection matches all valid flows.
		if _, ok := obj["id"]; ok {
			if seen[sid+typeTag(obj["id"])] {
				errs = append(errs, fmt.Sprintf("duplicate step id %s", pyRepr(obj["id"])))
			}
			seen[sid+typeTag(obj["id"])] = true
		}
	}
	return errs, warns
}

func typeTag(v any) string {
	switch v.(type) {
	case nil:
		return "\x00nil"
	case bool:
		return "\x00bool"
	case string:
		return "\x00str"
	default:
		return "\x00num"
	}
}

func checkHookSemantics(obj map[string]any) (errs, warns []string) {
	errs = []string{}
	warns = []string{}
	action, _ := obj["action"].(string)
	_, hasRun := obj["run"]
	if (action == "run-validator" || action == "run-script") && !hasRun {
		errs = append(errs, fmt.Sprintf("hook action %s requires 'run' {kind, ref}",
			pyRepr(action)))
	}
	if (action == "deny" || action == "notify") && hasRun {
		warns = append(warns, fmt.Sprintf("hook action %s ignores 'run'", pyRepr(action)))
	}
	return errs, warns
}

func checkPolicySemantics(obj map[string]any) (errs, warns []string) {
	errs = []string{}
	warns = []string{}
	def, _ := obj["defaultEffect"].(string)
	if _, present := obj["defaultEffect"]; !present {
		def = "deny"
	}
	if def != "deny" {
		warns = append(warns, "policy defaultEffect should be 'deny'")
	}
	return errs, warns
}

// ----------------------------------------------------------------------------
// File discovery + validation
// ----------------------------------------------------------------------------

func isExcluded(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	for _, part := range strings.Split(filepath.Clean(rel), string(filepath.Separator)) {
		if ExcludeDirs[part] {
			return true
		}
	}
	return false
}

// IsExcluded is the exported exclusion check for sibling packages.
func IsExcluded(path, root string) bool {
	return isExcluded(path, root)
}

func CollectFiles(root string) []string {
	info, err := os.Stat(root)
	if err == nil && !info.IsDir() {
		return []string{root}
	}
	var out []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if info.Name() != "." && ExcludeDirs[info.Name()] && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".md" && ext != ".json" {
			return nil
		}
		if ExcludeFiles[filepath.Base(path)] && filepath.Dir(path) != root {
			if filepath.Base(path) == "trace.json" {
				return nil
			}
		}
		out = append(out, path)
		return nil
	})
	sortStrings(out)
	return out
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func looksLikeSchemaDoc(doc any) bool {
	obj, ok := doc.(map[string]any)
	if !ok {
		return false
	}
	_, hasSchema := obj["$schema"]
	_, hasID := obj["$id"]
	return hasSchema && hasID
}

func pathPartsLower(path string) []string {
	clean := filepath.Clean(path)
	var parts []string
	for _, p := range strings.Split(clean, string(filepath.Separator)) {
		parts = append(parts, strings.ToLower(p))
	}
	return parts
}

func hasPart(parts []string, want string) bool {
	for _, p := range parts {
		if p == want {
			return true
		}
	}
	return false
}

// ClassifyJSON returns the schema short-name for a JSON doc, or "" to skip.
func ClassifyJSON(path string, doc any) string {
	if looksLikeSchemaDoc(doc) {
		return ""
	}
	if filepath.Base(path) == "trace.json" {
		return "trace"
	}
	if obj, ok := doc.(map[string]any); ok {
		if _, ok := obj["policyId"]; ok {
			return "policy"
		}
		if _, ok := obj["capabilities"]; ok {
			return "mcp-registry"
		}
		if _, hasR := obj["results"]; hasR {
			if _, hasV := obj["verdict"]; hasV {
				return "verification-report"
			}
		}
		if _, hasE := obj["event"]; hasE {
			if _, hasM := obj["matcher"]; hasM {
				return "hook"
			}
		}
		if _, ok := obj["howToVerify"]; ok {
			return "acceptance"
		}
		if id, ok := obj["id"].(string); ok && strings.HasPrefix(id, "ACC-") {
			return "acceptance"
		}
	}
	if list, ok := doc.([]any); ok && len(list) > 0 {
		if first, ok := list[0].(map[string]any); ok {
			if _, hasE := first["event"]; hasE {
				if _, hasM := first["matcher"]; hasM {
					return "hook"
				}
			}
			if _, ok := first["howToVerify"]; ok {
				return "acceptance"
			}
		}
	}
	parts := pathPartsLower(path)
	if hasPart(parts, "policies") {
		return "policy"
	}
	if hasPart(parts, "acceptance") {
		return "acceptance"
	}
	if hasPart(parts, "hooks") {
		return "hook"
	}
	return ""
}

func hasKey(obj map[string]any, key string) bool {
	_, ok := obj[key]
	return ok
}

func validateMarkdownFile(path string, strict bool, schemas *Schemas,
	errors, warnings *[]Entry, artifacts map[string]string) {
	text, err := readFileUTF8(path)
	if err != nil {
		// Divergence note: CPython raises UnicodeDecodeError (a ValueError,
		// not OSError) on non-UTF8 .md, escaping this handler with a
		// traceback. Go reports a clean error instead; strictly more robust.
		*errors = append(*errors, Entry{Path: path,
			Message: fmt.Sprintf("unreadable: %s", readErrText(path, err))})
		return
	}
	fm, body, perr := ParseFrontmatter(text)
	if fm == nil && perr == "" {
		return
	}
	if perr != "" {
		*errors = append(*errors, Entry{Path: path, Message: perr, Rule: "frontmatter"})
		return
	}
	fname := filepath.Base(path)
	if fname == "SKILL.md" || (hasKey(fm, "name") && hasKey(fm, "description") &&
		!hasKey(fm, "id") && !hasKey(fm, "steps")) {
		schema, _ := schemas.Load("skill")
		for _, msg := range ValidateAgainstSchema(fm, schema, "$") {
			*errors = append(*errors, Entry{Path: path, Message: msg, Rule: "skill.schema"})
		}
		es, ws := checkSkillSemantics(fm, path, body)
		for _, msg := range es {
			*errors = append(*errors, Entry{Path: path, Message: msg, Rule: "skill.semantics"})
		}
		for _, msg := range ws {
			*warnings = append(*warnings, Entry{Path: path, Message: msg, Rule: "skill.semantics"})
		}
		return
	}
	if hasKey(fm, "steps") || (strings.HasSuffix(fname, ".md") && hasPart(pathPartsLower(path), "workflows")) {
		schema, _ := schemas.Load("workflow")
		for _, msg := range ValidateAgainstSchema(fm, schema, "$") {
			*errors = append(*errors, Entry{Path: path, Message: msg, Rule: "workflow.schema"})
		}
		es, _ := checkWorkflowSemantics(fm)
		for _, msg := range es {
			*errors = append(*errors, Entry{Path: path, Message: msg, Rule: "workflow.semantics"})
		}
		return
	}
	if hasKey(fm, "id") && hasKey(fm, "kind") {
		schema, _ := schemas.Load("artifact-frontmatter")
		for _, msg := range ValidateAgainstSchema(fm, schema, "$") {
			*errors = append(*errors, Entry{Path: path, Message: msg, Rule: "artifact-frontmatter.schema"})
		}
		if aid, ok := fm["id"].(string); ok {
			if prev, dup := artifacts[aid]; dup {
				*errors = append(*errors, Entry{Path: path,
					Message: fmt.Sprintf("duplicate artifact id %s (also %s)",
						pyRepr(aid), prev),
					Rule: "artifact.duplicate"})
			} else {
				artifacts[aid] = path
			}
		}
		return
	}
	if hasKey(fm, "name") && hasKey(fm, "steps") {
		schema, _ := schemas.Load("workflow")
		for _, msg := range ValidateAgainstSchema(fm, schema, "$") {
			*errors = append(*errors, Entry{Path: path, Message: msg, Rule: "workflow.schema"})
		}
	} else if hasKey(fm, "name") {
		schema, _ := schemas.Load("skill")
		for _, msg := range ValidateAgainstSchema(fm, schema, "$") {
			*errors = append(*errors, Entry{Path: path, Message: msg, Rule: "skill.schema"})
		}
	} else {
		*warnings = append(*warnings, Entry{Path: path,
			Message: "frontmatter ignored: no id/kind (artifact), name (skill), or steps (workflow)",
			Rule:    "frontmatter.shape"})
	}
}

func readErrText(path string, err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

func validateJSONFile(path string, strict bool, schemas *Schemas,
	errors, warnings *[]Entry) {
	text, err := readFileUTF8(path)
	if err != nil {
		*errors = append(*errors, Entry{Path: path,
			Message: fmt.Sprintf("invalid JSON: %s", readErrText(path, err))})
		return
	}
	doc, err := decodeJSON(text)
	if err != nil {
		*errors = append(*errors, Entry{Path: path,
			Message: fmt.Sprintf("invalid JSON: %s", err)})
		return
	}
	kind := ClassifyJSON(path, doc)
	if kind == "" {
		return
	}
	if kind == "hook" {
		if list, ok := doc.([]any); ok {
			schema, _ := schemas.Load("hook")
			for i, item := range list {
				prefix := fmt.Sprintf("$[%d]", i)
				for _, msg := range ValidateAgainstSchema(item, schema, prefix) {
					*errors = append(*errors, Entry{Path: path, Message: msg, Rule: "hook.schema"})
				}
				if obj, ok := item.(map[string]any); ok {
					es, ws := checkHookSemantics(obj)
					for _, msg := range es {
						*errors = append(*errors, Entry{Path: path,
							Message: fmt.Sprintf("%s: %s", prefix, msg), Rule: "hook.semantics"})
					}
					for _, msg := range ws {
						*warnings = append(*warnings, Entry{Path: path,
							Message: fmt.Sprintf("%s: %s", prefix, msg), Rule: "hook.semantics"})
					}
				}
			}
			return
		}
	}
	if kind == "acceptance" {
		if list, ok := doc.([]any); ok {
			schema, _ := schemas.Load("acceptance")
			for i, item := range list {
				prefix := fmt.Sprintf("$[%d]", i)
				for _, msg := range ValidateAgainstSchema(item, schema, prefix) {
					*errors = append(*errors, Entry{Path: path, Message: msg, Rule: "acceptance.schema"})
				}
				if obj, ok := item.(map[string]any); ok {
					es, ws := checkAcceptanceSemantics(obj, strict)
					for _, msg := range es {
						*errors = append(*errors, Entry{Path: path,
							Message: fmt.Sprintf("%s: %s", prefix, msg), Rule: "acceptance.semantics"})
					}
					for _, msg := range ws {
						*warnings = append(*warnings, Entry{Path: path,
							Message: fmt.Sprintf("%s: %s", prefix, msg), Rule: "acceptance.semantics"})
					}
				}
			}
			return
		}
	}
	schema, _ := schemas.Load(kind)
	for _, msg := range ValidateAgainstSchema(doc, schema, "$") {
		*errors = append(*errors, Entry{Path: path, Message: msg, Rule: kind + ".schema"})
	}
	if obj, ok := doc.(map[string]any); ok {
		switch kind {
		case "acceptance":
			es, ws := checkAcceptanceSemantics(obj, strict)
			for _, msg := range es {
				*errors = append(*errors, Entry{Path: path, Message: msg, Rule: "acceptance.semantics"})
			}
			for _, msg := range ws {
				*warnings = append(*warnings, Entry{Path: path, Message: msg, Rule: "acceptance.semantics"})
			}
		case "hook":
			es, ws := checkHookSemantics(obj)
			for _, msg := range es {
				*errors = append(*errors, Entry{Path: path, Message: msg, Rule: "hook.semantics"})
			}
			for _, msg := range ws {
				*warnings = append(*warnings, Entry{Path: path, Message: msg, Rule: "hook.semantics"})
			}
		case "policy":
			_, ws := checkPolicySemantics(obj)
			for _, msg := range ws {
				*warnings = append(*warnings, Entry{Path: path, Message: msg, Rule: "policy.semantics"})
			}
		}
	}
}

func checkTraceLinks(artifacts map[string]string, files []string, strict bool,
	errors, warnings *[]Entry) map[string]bool {
	known := map[string]bool{}
	for aid := range artifacts {
		known[aid] = true
	}
	for _, path := range files {
		if filepath.Ext(path) != ".json" {
			continue
		}
		text, err := readFileUTF8(path)
		if err != nil {
			continue
		}
		doc, err := decodeJSON(text)
		if err != nil {
			continue
		}
		if looksLikeSchemaDoc(doc) {
			continue
		}
		var objs []any
		if list, ok := doc.([]any); ok {
			objs = list
		} else {
			objs = []any{doc}
		}
		for _, o := range objs {
			obj, ok := o.(map[string]any)
			if !ok {
				continue
			}
			if id, ok := obj["id"].(string); ok {
				known[id] = true
			}
			if acc, ok := obj["acceptanceId"].(string); ok {
				known[acc] = true
			}
		}
	}
	return known
}

// ValidatePath validates a file or directory tree.
// Returns (errors, warnings, artifacts).
func ValidatePath(schemas *Schemas, target string, strict bool) ([]Entry, []Entry, map[string]string) {
	errors := []Entry{}
	warnings := []Entry{}
	artifacts := map[string]string{}
	root := target
	base := target
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		base = filepath.Dir(target)
	}
	files := CollectFiles(root)
	for _, path := range files {
		if isExcluded(path, base) {
			continue
		}
		switch filepath.Ext(path) {
		case ".md":
			validateMarkdownFile(path, strict, schemas, &errors, &warnings, artifacts)
		case ".json":
			if filepath.Base(path) == "trace.json" && isDir(root) {
				continue
			}
			validateJSONFile(path, strict, schemas, &errors, &warnings)
		}
	}
	known := checkTraceLinks(artifacts, files, strict, &errors, &warnings)
	aids := make([]string, 0, len(artifacts))
	for aid := range artifacts {
		aids = append(aids, aid)
	}
	sortStrings(aids)
	for _, aid := range aids {
		apath := artifacts[aid]
		text, err := readFileUTF8(apath)
		if err != nil {
			continue
		}
		fm, _, perr := ParseFrontmatter(text)
		if fm == nil || perr != "" {
			continue
		}
		links, _ := fm["links"].(map[string]any)
		if links == nil {
			continue
		}
		for rel, targets := range links {
			list, ok := targets.([]any)
			if !ok {
				continue
			}
			for _, tgt := range list {
				// Divergence note: CPython crashes on unhashable link
				// targets; Go skips non-strings (schemas require strings).
				ts, ok := tgt.(string)
				if !ok || known[ts] {
					if !ok {
						continue
					}
					continue
				}
				msg := fmt.Sprintf("%s: dangling link %s -> %s", aid, rel, pyRepr(ts))
				if strict {
					errors = append(errors, Entry{Path: apath, Message: msg, Rule: "links.dangling"})
				} else {
					warnings = append(warnings, Entry{Path: apath, Message: msg, Rule: "links.dangling"})
				}
			}
		}
	}
	for _, path := range files {
		if filepath.Base(path) != "trace.json" || !isDir(root) {
			continue
		}
		text, err := readFileUTF8(path)
		if err != nil {
			errors = append(errors, Entry{Path: path,
				Message: fmt.Sprintf("invalid JSON: %s", readErrText(path, err))})
			continue
		}
		doc, err := decodeJSON(text)
		if err != nil {
			errors = append(errors, Entry{Path: path,
				Message: fmt.Sprintf("invalid JSON: %s", err)})
			continue
		}
		schema, _ := schemas.Load("trace")
		for _, msg := range ValidateAgainstSchema(doc, schema, "$") {
			errors = append(errors, Entry{Path: path, Message: msg, Rule: "trace.schema"})
		}
	}
	sortEntries(errors)
	sortEntries(warnings)
	return errors, warnings, artifacts
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func sortEntries(entries []Entry) {
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0; j-- {
			a, b := entries[j-1], entries[j]
			if b.Path < a.Path || (b.Path == a.Path && b.Message < a.Message) {
				entries[j], entries[j-1] = entries[j-1], entries[j]
			} else {
				break
			}
		}
	}
}

// DefaultSchemasDir locates the schemas/ directory: explicit override,
// CWD-relative (repo-root runs), then executable-relative layouts.
func DefaultSchemasDir() (string, error) {
	if env := os.Getenv("SHIPLOOM_SCHEMAS"); env != "" {
		return env, nil
	}
	candidates := []string{
		"schemas",
		exeDir() + "/schemas",
		exeDir() + "/../schemas",
	}
	tried := []string{}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if isDir(c) {
			return c, nil
		}
		tried = append(tried, c)
	}
	return "", fmt.Errorf("schemas directory not found (tried %s)", strings.Join(tried, ", "))
}

func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(exe))
	if err != nil {
		return filepath.Dir(exe)
	}
	return dir
}

// RunValidate implements `validate [--strict] [path]`. Returns exit code.
func RunValidate(args []string) int {
	strict := false
	var positional []string
	for _, arg := range args {
		switch arg {
		case "--strict":
			strict = true
		case "--help", "-h":
			fmt.Println("usage: shiploom validate [--strict] [path]")
			fmt.Println()
			fmt.Println("Schemas + artifact frontmatter + links. Exit 0 pass, 2 validation fail.")
			return 0
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "shiploom: error: unrecognized arguments: %s\n", arg)
				return 2
			}
			positional = append(positional, arg)
		}
	}
	if len(positional) > 1 {
		fmt.Fprintf(os.Stderr, "shiploom: error: unrecognized arguments: %s\n",
			strings.Join(positional[1:], " "))
		return 2
	}
	target := "."
	if len(positional) == 1 {
		target = positional[0]
	}
	schemasDir, err := DefaultSchemasDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "shiploom: error: %s\n", err)
		return 2
	}
	schemas := NewSchemas(schemasDir)
	errors, warnings, _ := ValidatePath(schemas, target, strict)
	fmt.Println(string(mustMarshalResult(errors, warnings)))
	return codeFor(errors)
}

// mustMarshalResult renders {"errors","ok","warnings"} like Python
// json.dump(sort_keys=True, indent=2). Entry rule keys omit when empty.
func mustMarshalResult(errors, warnings []Entry) []byte {
	toAny := func(entries []Entry) []any {
		out := make([]any, 0, len(entries))
		for _, e := range entries {
			m := map[string]any{"path": e.Path, "message": e.Message}
			if e.Rule != "" {
				m["rule"] = e.Rule
			}
			out = append(out, m)
		}
		return out
	}
	doc := map[string]any{
		"errors":   toAny(errors),
		"ok":       len(errors) == 0,
		"warnings": toAny(warnings),
	}
	out, err := jsoncanon.Marshal(doc)
	if err != nil {
		return []byte("{}")
	}
	return out
}

func codeFor(errors []Entry) int {
	if len(errors) > 0 {
		return 2
	}
	return 0
}
