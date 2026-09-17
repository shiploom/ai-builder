// Package validate ports validators/validate.py (stdlib only).
//
// Fidelity contract: same file dispatch, same error strings, same exit
// codes. Documented divergences from CPython live in GO_MIGRATION_PLAN.md
// under "port notes" (engine-specific regexp/JSON error suffixes, exotic
// numerics, non-UTF8 .md handling, unhashable workflow ids).
package validate

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/shiploom/ai-builder/internal/jsoncanon"
)

// Entry mirrors Python {"path","message","rule?"} dicts.
type Entry struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Rule    string `json:"rule,omitempty"`
}

// EntryMap renders an Entry exactly like the Python validators do:
// {"path","message"} plus "rule" only when set.
func EntryMap(e Entry) map[string]any {
	m := map[string]any{"path": e.Path, "message": e.Message}
	if e.Rule != "" {
		m["rule"] = e.Rule
	}
	return m
}

// ----------------------------------------------------------------------------
// Minimal JSON Schema (draft 2020-12) subset evaluator
// ----------------------------------------------------------------------------

// SchemaFiles maps short names to normative schema files (mirrors Python).
var SchemaFiles = map[string]string{
	"artifact-frontmatter": "artifact-frontmatter.schema.json",
	"acceptance":           "acceptance.schema.json",
	"skill":                "skill.schema.json",
	"workflow":             "workflow.schema.json",
	"hook":                 "hook.schema.json",
	"policy":               "policy.schema.json",
	"mcp-registry":         "mcp-registry.schema.json",
	"verification-report":  "verification-report.schema.json",
	"trace":                "trace-link.schema.json",
}

// Schemas loads and caches normative schemas from a directory.
type Schemas struct {
	dir   string
	cache map[string]map[string]any
}

// NewSchemas returns a loader rooted at dir (the schemas/ directory).
func NewSchemas(dir string) *Schemas {
	return &Schemas{dir: dir, cache: map[string]map[string]any{}}
}

// Load returns the parsed schema for a short name.
func (s *Schemas) Load(name string) (map[string]any, error) {
	if doc, ok := s.cache[name]; ok {
		return doc, nil
	}
	file, ok := SchemaFiles[name]
	if !ok {
		return nil, fmt.Errorf("unknown schema %q", name)
	}
	raw, err := readFileUTF8(s.dir + "/" + file)
	if err != nil {
		return nil, err
	}
	doc, err := decodeJSON(raw)
	if err != nil {
		return nil, err
	}
	obj, ok := doc.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema %s is not an object", file)
	}
	s.cache[name] = obj
	return obj, nil
}

func typeOk(value any, expected string) bool {
	switch expected {
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "integer":
		return isIntValue(value)
	case "number":
		return isNumValue(value)
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "null":
		return value == nil
	default:
		return false
	}
}

// isIntValue mirrors Python isinstance int (bools excluded). json.Number
// keeps its literal, so "800000" is integral while "25.0" is not.
func isIntValue(v any) bool {
	switch t := v.(type) {
	case bool:
		return false
	case int64, int, int32:
		return true
	case json.Number:
		return isIntegralLiteral(string(t))
	case float64:
		return false
	default:
		return false
	}
}

func isNumValue(v any) bool {
	switch t := v.(type) {
	case bool:
		return false
	case int64, int, int32, float64:
		return true
	case json.Number:
		_, err := strconv.ParseFloat(string(t), 64)
		return err == nil
	default:
		return false
	}
}

func isIntegralLiteral(s string) bool {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "+")
	s = strings.TrimPrefix(s, "-")
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func asFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case bool:
		return 0, false
	case int64:
		return float64(t), true
	case int:
		return float64(t), true
	case int32:
		return float64(t), true
	case float64:
		return t, true
	case json.Number:
		f, err := strconv.ParseFloat(string(t), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

// pyEqual mirrors Python == for JSON scalars (True==1, 1==1.0).
func pyEqual(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if as, ok := a.(string); ok {
		bs, ok := b.(string)
		return ok && as == bs
	}
	if _, ok := a.(string); ok {
		return false
	}
	if _, ok := b.(string); ok {
		return false
	}
	ab, aok := a.(bool)
	bb, bok := b.(bool)
	if aok && bok {
		return ab == bb
	}
	af, aok := asFloat(a)
	bf, bok := asFloat(b)
	if aok && bok {
		if aok && (isBoolNum(a) || isBoolNum(b)) {
			return af == bf
		}
		return af == bf
	}
	// bool vs number handled above via asFloat exclusion; compare directly.
	if aok != bok {
		// One side numeric, the other bool (asFloat rejects bools).
		if ab2, ok2 := a.(bool); ok2 {
			if bf2, ok3 := asFloat(b); ok3 {
				return (bf2 == 1 && ab2) || (bf2 == 0 && !ab2)
			}
		}
		if bb2, ok2 := b.(bool); ok2 {
			if af2, ok3 := asFloat(a); ok3 {
				return (af2 == 1 && bb2) || (af2 == 0 && !bb2)
			}
		}
		return false
	}
	asl, aok := a.([]any)
	bsl, bok := b.([]any)
	if aok && bok {
		if len(asl) != len(bsl) {
			return false
		}
		for i := range asl {
			if !pyEqual(asl[i], bsl[i]) {
				return false
			}
		}
		return true
	}
	am, aok := a.(map[string]any)
	bm, bok := b.(map[string]any)
	if aok && bok {
		if len(am) != len(bm) {
			return false
		}
		for k, v := range am {
			w, ok := bm[k]
			if !ok || !pyEqual(v, w) {
				return false
			}
		}
		return true
	}
	return false
}

func isBoolNum(v any) bool {
	_, ok := v.(bool)
	return ok
}

// PyStr mirrors Python "%s" / str() for JSON-shaped values.
func PyStr(v any) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case string:
		return t
	case bool:
		if t {
			return "True"
		}
		return "False"
	default:
		return pyRepr(v)
	}
}

// PyRepr is the exported shared CPython-repr helper.
func PyRepr(v any) string {
	return pyRepr(v)
}

func pyRepr(v any) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case bool:
		if t {
			return "True"
		}
		return "False"
	case string:
		return pyReprString(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case int:
		return strconv.Itoa(t)
	case int32:
		return strconv.FormatInt(int64(t), 10)
	case float64:
		return pyFloatRepr(t)
	case json.Number:
		return string(t)
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			parts = append(parts, pyRepr(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		// Divergence note: CPython preserves insertion order; Go sorts.
		// Only reachable when a mapping fails an enum check.
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, pyReprString(k)+": "+pyRepr(t[k]))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// pyFloatRepr mirrors CPython repr() for floats.
func pyFloatRepr(f float64) string {
	if math.IsNaN(f) {
		return "nan"
	}
	if math.IsInf(f, 1) {
		return "inf"
	}
	if math.IsInf(f, -1) {
		return "-inf"
	}
	neg := ""
	if math.Signbit(f) {
		neg = "-"
		f = -f
	}
	if f == math.Trunc(f) && f < 1e16 {
		return neg + strconv.FormatInt(int64(f), 10) + ".0"
	}
	sci := strconv.FormatFloat(f, 'e', -1, 64)
	ePos := strings.LastIndexByte(sci, 'e')
	mant, exp := sci[:ePos], sci[ePos+1:]
	expVal, _ := strconv.Atoi(exp)
	digits := strings.Replace(mant, ".", "", 1)
	if expVal >= -4 && expVal < 16 {
		point := expVal + 1
		var out string
		switch {
		case point <= 0:
			out = "0." + strings.Repeat("0", -point) + digits
		case point >= len(digits):
			out = digits + strings.Repeat("0", point-len(digits))
		default:
			out = digits[:point] + "." + digits[point:]
		}
		if !strings.Contains(out, ".") {
			out += ".0"
		}
		return neg + out
	}
	intPart := digits[:1]
	fracPart := strings.TrimRight(digits[1:], "0")
	m := intPart
	if fracPart != "" {
		m += "." + fracPart
	}
	return neg + m + "e" + exp
}

func pyReprString(s string) string {
	useDouble := strings.Contains(s, "'") && !strings.Contains(s, `"`)
	var b strings.Builder
	if useDouble {
		b.WriteByte('"')
	} else {
		b.WriteByte('\'')
	}
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString("\\\\")
		case r == '\'' && !useDouble:
			b.WriteString("\\'")
		case r == '"' && useDouble:
			b.WriteString("\\\"")
		case r == '\n':
			b.WriteString("\\n")
		case r == '\r':
			b.WriteString("\\r")
		case r == '\t':
			b.WriteString("\\t")
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, "\\x%02x", r)
		case unicodeIsPrint(r):
			b.WriteRune(r)
		case r < 0x100:
			fmt.Fprintf(&b, "\\x%02x", r)
		case r <= 0xffff:
			fmt.Fprintf(&b, "\\u%04x", r)
		default:
			fmt.Fprintf(&b, "\\U%08x", r)
		}
	}
	if useDouble {
		b.WriteByte('"')
	} else {
		b.WriteByte('\'')
	}
	return b.String()
}

// unicodeIsPrint approximates Python str.isprintable for repr purposes:
// printable graphic incl. space, excluding format/control characters.
func unicodeIsPrint(r rune) bool {
	if r == ' ' {
		return true
	}
	if r < 0xa0 {
		return r >= 0x20 && r != 0x7f
	}
	// U+00AD soft hyphen and format chars (Cf) are unprintable in Python.
	if unicodeIn(r, '\u00ad', '\u200b', '\u200c', '\u200d', '\ufeff') {
		return false
	}
	return unicode.IsPrint(r)
}

func unicodeIn(r rune, set ...rune) bool {
	for _, s := range set {
		if r == s {
			return true
		}
	}
	return false
}

// readFileUTF8 reads a file, rejecting non-UTF8 bytes (Python read_text
// strictness parity).
func readFileUTF8(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(raw) {
		return "", fmt.Errorf("invalid UTF-8 in %s", path)
	}
	return string(raw), nil
}

// decodeJSON parses with number literals preserved (no float64 drift)
// and rejects trailing data like Python json.loads.
func decodeJSON(data string) (any, error) {
	return jsoncanon.Decode([]byte(data))
}

// DecodeJSON is the exported shared JSON decoder (number literals
// preserved, trailing data rejected).
func DecodeJSON(data []byte) (any, error) {
	return jsoncanon.Decode(data)
}

// ValidateAgainstSchema validates data against a schema subset.
// Returns error strings (same shapes as Python; regexp/JSON suffixes are
// engine-specific — see port notes).
func ValidateAgainstSchema(data any, schema map[string]any, path string) []string {
	var errors []string
	if schema == nil {
		return errors
	}
	if expected, ok := schema["type"]; ok {
		var types []string
		switch t := expected.(type) {
		case string:
			types = []string{t}
		case []any:
			for _, item := range t {
				if s, ok := item.(string); ok {
					types = append(types, s)
				}
			}
		}
		matched := false
		for _, t := range types {
			if typeOk(data, t) {
				matched = true
				break
			}
		}
		if !matched {
			errors = append(errors, fmt.Sprintf("%s: expected type %v, got %s",
				path, expected, goTypeName(data)))
			return errors
		}
	}
	if enum, ok := schema["enum"].([]any); ok {
		hit := false
		for _, e := range enum {
			if pyEqual(data, e) {
				hit = true
				break
			}
		}
		if !hit {
			errors = append(errors, fmt.Sprintf("%s: %s not in enum %s",
				path, pyRepr(data), pyRepr(enum)))
		}
	}
	if s, ok := data.(string); ok {
		if pattern, ok := schema["pattern"].(string); ok {
			matched, err := regexp.MatchString(pattern, s)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: invalid schema pattern: %s", path, err))
			} else if !matched {
				errors = append(errors, fmt.Sprintf("%s: %s does not match pattern %s",
					path, pyRepr(s), pyRepr(pattern)))
			}
		}
		if minLength(data, schema, "minLength") {
			errors = append(errors, fmt.Sprintf("%s: string shorter than minLength %d",
				path, asInt(schema["minLength"])))
		}
		if maxLengthExceeded(data, schema) {
			errors = append(errors, fmt.Sprintf("%s: string longer than maxLength %d",
				path, asInt(schema["maxLength"])))
		}
	}
	if _, isBool := data.(bool); !isBool {
		if f, ok := asFloat(data); ok {
			if minimum, ok := schemaNum(schema, "minimum"); ok && f < minimum {
				errors = append(errors, fmt.Sprintf("%s: %s below minimum %s",
					path, pyRepr(data), pyRepr(schema["minimum"])))
			}
			if maximum, ok := schemaNum(schema, "maximum"); ok && f > maximum {
				errors = append(errors, fmt.Sprintf("%s: %s above maximum %s",
					path, pyRepr(data), pyRepr(schema["maximum"])))
			}
		}
	}
	if list, ok := data.([]any); ok {
		if n, ok := schemaNum(schema, "minItems"); ok && float64(len(list)) < n {
			errors = append(errors, fmt.Sprintf("%s: fewer than minItems %d",
				path, asInt(schema["minItems"])))
		}
		if n, ok := schemaNum(schema, "maxItems"); ok && float64(len(list)) > n {
			errors = append(errors, fmt.Sprintf("%s: more than maxItems %d",
				path, asInt(schema["maxItems"])))
		}
		if items, ok := schema["items"].(map[string]any); ok {
			for i, item := range list {
				errors = append(errors, ValidateAgainstSchema(
					item, items, fmt.Sprintf("%s[%d]", path, i))...)
			}
		}
	}
	if obj, ok := data.(map[string]any); ok {
		if required, ok := schema["required"].([]any); ok {
			for _, key := range required {
				if ks, ok := key.(string); ok {
					if _, present := obj[ks]; !present {
						errors = append(errors, fmt.Sprintf(
							"%s: missing required property %s", path, pyRepr(ks)))
					}
				}
			}
		}
		props, _ := schema["properties"].(map[string]any)
		if props == nil {
			props = map[string]any{}
		}
		patProps, _ := schema["patternProperties"].(map[string]any)
		// Sorted for determinism (Python follows file order; our schemas
		// carry a single pattern each, so this never changes outcomes).
		patterns := make([]string, 0, len(patProps))
		for pat := range patProps {
			patterns = append(patterns, pat)
		}
		sort.Strings(patterns)
		for key, value := range obj {
			var sub map[string]any
			subPath := path + "." + key
			subSet := false
			if ps, present := props[key]; present {
				// A non-dict subschema validates nothing (mirrors Python,
				// where a non-dict schema returns no errors).
				if m, ok := ps.(map[string]any); ok {
					sub, subSet = m, true
				} else {
					continue
				}
			}
			if !subSet {
				for _, pat := range patterns {
					matched, err := regexp.MatchString(pat, key)
					if err != nil {
						continue
					}
					if matched {
						if m, ok := patProps[pat].(map[string]any); ok {
							sub = m
						}
						break
					}
				}
				if sub == nil {
					if extra, ok := schema["additionalProperties"]; ok {
						if eb, ok := extra.(bool); ok && !eb {
							errors = append(errors, fmt.Sprintf(
								"%s: unexpected property %s", path, pyRepr(key)))
							continue
						}
						if m, ok := extra.(map[string]any); ok {
							sub = m
						}
					}
				}
			}
			if sub != nil {
				errors = append(errors, ValidateAgainstSchema(value, sub, subPath)...)
			}
		}
	}
	return errors
}

// goTypeName mirrors Python type(x).__name__ for JSON-shaped values.
func goTypeName(v any) string {
	switch v.(type) {
	case map[string]any:
		return "dict"
	case []any:
		return "list"
	case string:
		return "str"
	case bool:
		return "bool"
	case nil:
		return "NoneType"
	case int64, int, int32, json.Number:
		if isIntValue(v) {
			return "int"
		}
		return "float"
	case float64:
		return "float"
	default:
		return "object"
	}
}

func asInt(v any) int {
	if f, ok := asFloat(v); ok {
		return int(f)
	}
	return 0
}

func minLength(data any, schema map[string]any, key string) bool {
	s, ok := data.(string)
	if !ok {
		return false
	}
	n, ok := schemaNum(schema, key)
	if !ok {
		return false
	}
	return float64(len([]rune(s))) < n
}

func maxLengthExceeded(data any, schema map[string]any) bool {
	s, ok := data.(string)
	if !ok {
		return false
	}
	n, ok := schemaNum(schema, "maxLength")
	if !ok {
		return false
	}
	return float64(len([]rune(s))) > n
}

func schemaNum(schema map[string]any, key string) (float64, bool) {
	v, ok := schema[key]
	if !ok {
		return 0, false
	}
	return asFloat(v)
}
