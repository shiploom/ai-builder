package validate

import (
	"encoding/json"
	"strings"
	"testing"
)

func mustDoc(t *testing.T, text string) map[string]any {
	t.Helper()
	v, err := decodeJSON(text)
	if err != nil {
		t.Fatal(err)
	}
	obj, ok := v.(map[string]any)
	if !ok {
		t.Fatal("not an object")
	}
	return obj
}

func TestSchemaSubsetTable(t *testing.T) {
	cases := []struct {
		name   string
		data   string
		schema string
		ok     bool
	}{
		{"required-present", `{"a": 1}`, `{"type": "object", "required": ["a"]}`, true},
		{"required-missing", `{"b": 1}`, `{"type": "object", "required": ["a"]}`, false},
		{"minLength-fail", `"x"`, `{"type": "string", "minLength": 2}`, false},
		{"maxLength-fail", `"xyz"`, `{"type": "string", "maxLength": 2}`, false},
		{"pattern-pass", `"abc-123"`, `{"type": "string", "pattern": "^[a-z]+-[0-9]+$"}`, true},
		{"pattern-fail", `"ABC"`, `{"type": "string", "pattern": "^[a-z]+$"}`, false},
		{"enum-pass", `"b"`, `{"type": "string", "enum": ["a", "b"]}`, true},
		{"enum-fail", `"c"`, `{"type": "string", "enum": ["a", "b"]}`, false},
		{"bool-not-integer", `true`, `{"type": "integer"}`, false},
		{"bool-is-boolean", `true`, `{"type": "boolean"}`, true},
		{"minimum-fail", `1.5`, `{"type": "number", "minimum": 2}`, false},
		{"minItems-fail", `[1]`, `{"type": "array", "minItems": 2}`, false},
		{"prop-type-fail", `{"x": 1}`, `{"type": "object", "properties": {"x": {"type": "string"}}}`, false},
		{"additional-false", `{"z": 1}`, `{"type": "object", "properties": {"a": {"type": "string"}}, "additionalProperties": false}`, false},
		{"patternProps", `{"cap.x": {"ttlS": 1}}`, `{"type": "object", "patternProperties": {"^cap\\.": {"type": "object"}}}`, true},
		{"int-literal-is-integer", `800000`, `{"type": "integer"}`, true},
		{"float-literal-not-integer", `25.0`, `{"type": "integer"}`, false},
		{"float-literal-is-number", `25.0`, `{"type": "number"}`, true},
		{"null-type", `null`, `{"type": "null"}`, true},
		{"type-list", `1`, `{"type": ["string", "integer"]}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := decodeJSON(tc.data)
			if err != nil {
				t.Fatal(err)
			}
			errs := ValidateAgainstSchema(data, mustDoc(t, tc.schema), "$")
			if (len(errs) == 0) != tc.ok {
				t.Fatalf("errors=%v, want ok=%v", errs, tc.ok)
			}
		})
	}
}

func TestUnknownKeywordsIgnored(t *testing.T) {
	data, _ := decodeJSON(`1`)
	errs := ValidateAgainstSchema(data, mustDoc(t, `{"type": "integer", "default": 5, "format": "x"}`), "$")
	if len(errs) != 0 {
		t.Fatalf("unknown keywords must be ignored: %v", errs)
	}
}

func TestAllNormativeSchemasLoad(t *testing.T) {
	schemas := NewSchemas("../../schemas")
	for name := range SchemaFiles {
		doc, err := schemas.Load(name)
		if err != nil {
			t.Fatalf("%s: %s", name, err)
		}
		if doc["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
			t.Fatalf("%s: bad $schema", name)
		}
		_ = doc
	}
}

func TestPyReprTable(t *testing.T) {
	cases := map[string]string{
		`"a"`:       `'a'`,
		`"it's"`:    `"it's"`,
		`"a\"b"`:    `'a"b'`,
		`"a'b\"c"`:  `'a\'b"c'`,
		`"line\nx"`: `'line\nx'`,
		`"t\there"`: `'t\there'`,
		`"q\\q"`:    `'q\\q'`,
		`"é☃"`:      `'é☃'`,
		`" "`:       `'\xa0'`,
		`"\u200b"`:  `'\u200b'`,
	}
	for in, want := range cases {
		var s string
		if err := json.Unmarshal([]byte(in), &s); err != nil {
			t.Fatal(err)
		}
		if got := pyRepr(s); got != want {
			t.Errorf("pyRepr(%q) = %s, want %s", s, got, want)
		}
	}
	if pyRepr(true) != "True" || pyRepr(false) != "False" || pyRepr(nil) != "None" {
		t.Fatal("bool/none repr")
	}
	if pyRepr(int64(800000)) != "800000" || pyRepr(json.Number("25.0")) != "25.0" {
		t.Fatal("number repr")
	}
	if pyRepr([]any{int64(1), "x"}) != "[1, 'x']" {
		t.Fatal("list repr")
	}
}

func TestFrontmatterCases(t *testing.T) {
	fm, body, err := ParseFrontmatter("# Just markdown\n\ntext\n")
	if fm != nil || err != "" || !strings.Contains(body, "text") {
		t.Fatalf("no-frontmatter must skip: %v %q %q", fm, body, err)
	}
	_, _, err = ParseFrontmatter("---\nid: REQ-001\n")
	if err == "" || !strings.Contains(err, "unterminated") {
		t.Fatalf("want unterminated error, got %q", err)
	}
	_, _, err = ParseFrontmatter("---\n\tid: REQ-001\n---\nbody\n")
	if err == "" || !strings.Contains(err, "tabs") {
		t.Fatalf("want tabs error, got %q", err)
	}
	_, _, err = ParseFrontmatter("---\nid: [unclosed\n---\nbody\n")
	if err == "" || !strings.Contains(err, "unclosed flow list") {
		t.Fatalf("want unclosed error, got %q", err)
	}
	fm, _, err = ParseFrontmatter("---\na: [X-001, 'Y-002']\nb: \"quoted\"\nc: 3\nd: 1.5\ne: true\nf:\n---\nbody\n")
	if err != "" {
		t.Fatal(err)
	}
	if fm["a"].([]any)[1] != "Y-002" || fm["b"] != "quoted" || fm["c"] != int64(3) {
		t.Fatalf("scalar shapes wrong: %v", fm)
	}
	if fm["d"] != 1.5 || fm["e"] != true || fm["f"] != nil {
		t.Fatalf("scalar shapes wrong: %v", fm)
	}
}

func TestFrontmatterArtifactShape(t *testing.T) {
	text := "---\nid: REQ-001\nkind: requirement\ntitle: T\nstatus: proposed\n" +
		"provenance:\n  - type: human\n    ref: \"idea.md:1\"\n    confidence: high\n" +
		"    date: 2026-09-17\nlinks:\n  requires: []\nowner: specifier\nversion: 1\n---\n\nBody.\n"
	fm, body, err := ParseFrontmatter(text)
	if err != "" {
		t.Fatal(err)
	}
	if fm["id"] != "REQ-001" || fm["version"] != int64(1) {
		t.Fatalf("top-level wrong: %v", fm)
	}
	prov := fm["provenance"].([]any)
	first := prov[0].(map[string]any)
	if first["confidence"] != "high" || first["ref"] != "idea.md:1" {
		t.Fatalf("nested list-of-maps wrong: %v", first)
	}
	if links := fm["links"].(map[string]any); len(links["requires"].([]any)) != 0 {
		t.Fatalf("flow list wrong: %v", links)
	}
	if !strings.Contains(body, "Body.") {
		t.Fatalf("body wrong: %q", body)
	}
}

func TestVagueTerms(t *testing.T) {
	if len(vagueTermsIn("Login should be fast and secure")) != 2 {
		t.Fatal("want fast+secure")
	}
	if len(vagueTermsIn("arrives within 60 seconds")) != 0 {
		t.Fatal("measurable statement must be clean")
	}
}

func TestAcceptanceSemantics(t *testing.T) {
	good := map[string]any{
		"id": "ACC-001", "statement": "X arrives within 60 seconds",
		"howToVerify": map[string]any{"type": "script", "command": "pytest t.py -q",
			"expect": "exit 0 under 60s"},
		"oracleRef": "oracle/ACC-001.sh",
	}
	if es, ws := checkAcceptanceSemantics(good, true); len(es)+len(ws) != 0 {
		t.Fatalf("good acceptance must be clean: %v %v", es, ws)
	}
	bad := map[string]any{
		"id": "ACC-002", "statement": "Login should be fast",
		"howToVerify": map[string]any{"type": "human", "expect": "looks fine"},
		"oracleRef":   "oracle/ACC-002.sh",
	}
	if es, _ := checkAcceptanceSemantics(bad, true); len(es) == 0 {
		t.Fatal("strict must fail vague acceptance")
	}
	if es, ws := checkAcceptanceSemantics(bad, false); len(es) != 0 || len(ws) == 0 {
		t.Fatalf("non-strict must warn: %v %v", es, ws)
	}
	noCmd := map[string]any{
		"id": "ACC-003", "statement": "Reset tokens expire",
		"howToVerify": map[string]any{"type": "script", "expect": "works"},
		"oracleRef":   "oracle/ACC-003.sh",
	}
	if es, _ := checkAcceptanceSemantics(noCmd, false); len(es) == 0 {
		t.Fatal("missing command must error")
	}
	abs := map[string]any{
		"id": "ACC-004", "statement": "Reset tokens expire",
		"howToVerify": map[string]any{"type": "script", "command": "t", "expect": "0123456789"},
		"oracleRef":   "/etc/oracle.sh",
	}
	if es, _ := checkAcceptanceSemantics(abs, false); len(es) == 0 {
		t.Fatal("absolute oracleRef must error")
	}
}
