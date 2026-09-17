// Package workflow ports the workflow-file helpers of cli/run.py
// (stdlib only): find_workflow, load_workflow, resolve_uses, plus the
// fnmatch-style globbing _glob_hits needs.
package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/shiploom/ai-builder/internal/validate"
)

// ToolRoot, when set (CLI startup derives it from the schemas dir),
// is searched first — the Go equivalent of Python's TOOL_ROOT.
var ToolRoot string

// FindWorkflow mirrors find_workflow(): overlay shadows core. Accepts
// bare names, "name.md", or legacy "workflows/name" paths. The overlay
// lives at <project>/.shiploom/workflows/; core at <ToolRoot>/core/.
// Returns "" when absent.
func FindWorkflow(name, projectDir string) string {
	base := name
	if strings.HasSuffix(base, ".md") {
		base = base[:len(base)-3]
	}
	base = strings.TrimPrefix(base, "workflows/")
	candidates := []string{
		filepath.Join(projectDir, ".shiploom", "workflows", base+".md"),
	}
	if ToolRoot != "" {
		candidates = append(candidates,
			filepath.Join(ToolRoot, "core", "workflows", base+".md"))
	}
	for _, candidate := range candidates {
		if isFile(candidate) {
			return candidate
		}
	}
	return ""
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// LoadWorkflow mirrors load_workflow(): read + frontmatter + workflow
// schema. Returns (fm, errors); fm is nil on any failure.
func LoadWorkflow(schemas *validate.Schemas, path string) (map[string]any, []string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, []string{fmt.Sprintf("unreadable workflow %s: %s", path, fsErrText(err))}
	}
	if !utf8.Valid(raw) {
		// Robustness note: CPython read_text(strict) raises here, which
		// run_workflow does not catch; reported instead of crashing.
		return nil, []string{fmt.Sprintf("unreadable workflow %s: invalid UTF-8 in %s", path, path)}
	}
	fm, _, perr := validate.ParseFrontmatter(string(raw))
	if perr != "" {
		return nil, []string{perr}
	}
	if fm == nil {
		return nil, []string{fmt.Sprintf("workflow %s has no frontmatter", path)}
	}
	schema, serr := schemas.Load("workflow")
	if serr == nil {
		if schemaErrors := validate.ValidateAgainstSchema(fm, schema, "$"); len(schemaErrors) > 0 {
			return nil, schemaErrors
		}
	}
	return fm, nil
}

// ResolveUses mirrors resolve_uses(): overlay path, then tool core;
// directories resolve to their SKILL.md. Returns "" when unresolvable.
func ResolveUses(ref, projectDir string) string {
	candidates := []string{filepath.Join(projectDir, ".shiploom", ref)}
	if ToolRoot != "" {
		candidates = append(candidates, filepath.Join(ToolRoot, "core", ref))
	}
	for _, candidate := range candidates {
		if isFile(candidate) {
			return candidate
		}
		if isDir(candidate) {
			skill := filepath.Join(candidate, "SKILL.md")
			if isFile(skill) {
				return skill
			}
		}
	}
	return ""
}

// GlobHits mirrors _glob_hits(): literal existence check for plain
// patterns, else a sorted root.glob() equivalent (pathlib semantics:
// single * never crosses /, ** recurses, dangling links filtered).
func GlobHits(pattern, projectDir string) []string {
	hits := []string{}
	if !strings.ContainsAny(pattern, "*?[") {
		candidate := pattern
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(projectDir, pattern)
		}
		if entryExists(candidate) {
			hits = append(hits, candidate)
		}
		return hits
	}
	segments := strings.Split(filepath.ToSlash(pattern), "/")
	_ = filepath.WalkDir(projectDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(projectDir, path)
		if err != nil || rel == "." {
			return nil
		}
		if matchSegments(segments, strings.Split(filepath.ToSlash(rel), "/")) &&
			entryExists(path) {
			hits = append(hits, path)
		}
		return nil
	})
	sort.Strings(hits)
	return hits
}

// entryExists mirrors Path.exists(): follows symlinks.
func entryExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// matchSegments matches slash-split patterns; "**" spans separators.
func matchSegments(pat, parts []string) bool {
	if len(pat) == 0 {
		return len(parts) == 0
	}
	if pat[0] == "**" {
		for i := 0; i <= len(parts); i++ {
			if matchSegments(pat[1:], parts[i:]) {
				return true
			}
		}
		return false
	}
	if len(parts) == 0 {
		return false
	}
	if !fnmatchSegment(pat[0], parts[0]) {
		return false
	}
	return matchSegments(pat[1:], parts[1:])
}

// fnmatchSegment mirrors fnmatch.fnmatchcase for one path segment
// (*, ?, [seq], [!seq]; unclosed [ is literal; backslash is literal
// on POSIX outside classes).
func fnmatchSegment(pattern, name string) bool {
	px, nx := 0, 0
	starPx, starNx := -1, -1
	for nx < len(name) {
		if px < len(pattern) {
			c := pattern[px]
			switch {
			case c == '*':
				starPx, starNx = px, nx
				px++
				continue
			case c == '?':
				px++
				nx++
				continue
			case c == '[':
				if consumed, ok := matchClass(pattern[px:], name[nx]); ok {
					px += consumed
					nx++
					continue
				}
				// Unclosed [: literal compare (CPython parity).
				if c == name[nx] {
					px++
					nx++
					continue
				}
			case c == name[nx]:
				px++
				nx++
				continue
			}
		}
		if starPx != -1 {
			starNx++
			if starNx > len(name) {
				return false
			}
			nx = starNx
			px = starPx + 1
			continue
		}
		return false
	}
	for px < len(pattern) && pattern[px] == '*' {
		px++
	}
	return px == len(pattern)
}

// matchClass matches "[...]" at the head of pattern against one char.
// Returns bytes of pattern consumed (through ']') and whether it matched.
func matchClass(pattern string, ch byte) (int, bool) {
	i := 1
	negate := false
	if i < len(pattern) && (pattern[i] == '!' || pattern[i] == '^') {
		negate = true
		i++
	}
	matched := false
	closed := false
	first := true
	for i < len(pattern) {
		if pattern[i] == ']' && !first {
			closed = true
			i++
			break
		}
		if pattern[i] == '\\' && i+1 < len(pattern) {
			i++
			if pattern[i] == ch {
				matched = true
			}
			i++
			first = false
			continue
		}
		lo := pattern[i]
		if i+2 < len(pattern) && pattern[i+1] == '-' && pattern[i+2] != ']' {
			hi := pattern[i+2]
			if lo <= ch && ch <= hi {
				matched = true
			}
			i += 3
		} else {
			if lo == ch {
				matched = true
			}
			i++
		}
		first = false
	}
	if !closed {
		return 0, false
	}
	if negate {
		return i, !matched
	}
	return i, matched
}

func fsErrText(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

func validUTF8(raw []byte) bool {
	return utf8.Valid(raw)
}
