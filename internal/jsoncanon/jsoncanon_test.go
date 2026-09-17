package jsoncanon

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// Goldens captured from CPython json on 2026-09-17 (see golden capture
// note in GO_MIGRATION_PLAN.md). Any change here means drift from Python.
func TestIndentGolden(t *testing.T) {
	doc := map[string]any{
		"a": "héllo\nworld\"q\" ☃ \x01\x7f",
		"b": json.Number("25.0"),
		"c": int64(800000),
		"d": true,
		"e": nil,
		"f": []any{int64(1), "x"},
		"g": map[string]any{},
	}
	got, err := Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n" +
		"  \"a\": \"h\\u00e9llo\\nworld\\\"q\\\" \\u2603 \\u0001\\u007f\",\n" +
		"  \"b\": 25.0,\n" +
		"  \"c\": 800000,\n" +
		"  \"d\": true,\n" +
		"  \"e\": null,\n" +
		"  \"f\": [\n" +
		"    1,\n" +
		"    \"x\"\n" +
		"  ],\n" +
		"  \"g\": {}\n" +
		"}"
	if string(got) != want {
		t.Fatalf("indent drift:\n got: %q\nwant: %q", got, want)
	}
}

func TestCompactGolden(t *testing.T) {
	doc := map[string]any{
		"a": "héllo\nworld\"q\" ☃ \x01\x7f",
		"b": json.Number("25.0"),
		"c": int64(800000),
	}
	got, err := MarshalCompact(doc)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":"h\u00e9llo\nworld\"q\" \u2603 \u0001\u007f","b":25.0,"c":800000}`
	if string(got) != want {
		t.Fatalf("compact drift:\n got: %q\nwant: %q", got, want)
	}
}

func TestManifestGenesisGolden(t *testing.T) {
	// Byte-exact genesis manifest written by Python cli/manifest.py
	// (initializedAt normalized out; key order and spacing are the test).
	doc := map[string]any{
		"coreVersion": "1.1.0", "workflow": "greenfield-full-lite",
		"workflowVersion": "1.0.0",
		"artifacts":       map[string]any{}, "gates": map[string]any{},
		"budgets": map[string]any{
			"spendUSD": map[string]any{"limit": json.Number("25.0"), "used": json.Number("0.0")},
			"tokens":   map[string]any{"limit": json.Number("800000"), "used": int64(0)},
		},
		"retries": map[string]any{}, "checkpoints": []any{},
		"initializedAt": "TS",
	}
	got, err := Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n" +
		"  \"artifacts\": {},\n" +
		"  \"budgets\": {\n" +
		"    \"spendUSD\": {\n" +
		"      \"limit\": 25.0,\n" +
		"      \"used\": 0.0\n" +
		"    },\n" +
		"    \"tokens\": {\n" +
		"      \"limit\": 800000,\n" +
		"      \"used\": 0\n" +
		"    }\n" +
		"  },\n" +
		"  \"checkpoints\": [],\n" +
		"  \"coreVersion\": \"1.1.0\",\n" +
		"  \"gates\": {},\n" +
		"  \"initializedAt\": \"TS\",\n" +
		"  \"retries\": {},\n" +
		"  \"workflow\": \"greenfield-full-lite\",\n" +
		"  \"workflowVersion\": \"1.0.0\"\n" +
		"}"
	if string(got) != want {
		t.Fatalf("genesis drift:\n got: %q\nwant: %q", got, want)
	}
}

func TestPyFloatEdges(t *testing.T) {
	cases := map[float64]string{
		0.1:                "0.1",
		25.0:               "25.0",
		1e16:               "1e+16",
		1e-5:               "1e-05",
		0.0001:             "0.0001",
		1234567890123456.0: "1234567890123456.0",
		1.5e30:             "1.5e+30",
	}
	for f, want := range cases {
		if got := pyFloat(f); got != want {
			t.Errorf("pyFloat(%v) = %q, want %q", f, got, want)
		}
	}
	if pyFloat(math.Copysign(0, -1)) != "-0.0" {
		t.Errorf("negative zero must keep its sign")
	}
}

func TestDecodePreservesLiterals(t *testing.T) {
	v, err := Decode([]byte(`{"a": 25.0, "b": 800000, "c": 1e3}`))
	if err != nil {
		t.Fatal(err)
	}
	m := v.(map[string]any)
	if m["a"].(json.Number) != "25.0" || m["b"].(json.Number) != "800000" {
		t.Fatalf("literals not preserved: %v", m)
	}
	if _, err := Decode([]byte(`{"a": 1} trailing`)); err == nil {
		t.Fatal("trailing data must error (Python json.loads parity)")
	}
}

func TestDecodeRejectsBadNumbers(t *testing.T) {
	if _, err := Marshal(map[string]any{"x": json.Number("abc")}); err == nil {
		t.Fatal("bad number literal must error")
	}
}

func TestUnsupportedType(t *testing.T) {
	if _, err := Marshal(map[string]any{"x": strings.Builder{}}); err == nil {
		t.Fatal("unsupported type must error")
	}
}
