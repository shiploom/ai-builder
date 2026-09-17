package execrun

import (
	"reflect"
	"testing"
)

// shlexTruth mirrors CPython shlex.split, generated 2026-09-17.
// Error entries expect failure; all others expect exact argv.
var shlexTruth = []struct {
	in    string
	out   []string
	isErr bool
}{
	{"pytest tests/unit -q", []string{"pytest", "tests/unit", "-q"}, false},
	{"python -m pytest -q", []string{"python", "-m", "pytest", "-q"}, false},
	{"echo 'a b' c", []string{"echo", "a b", "c"}, false},
	{"echo \"a b\" c", []string{"echo", "a b", "c"}, false},
	{"echo a\\ b", []string{"echo", "a b"}, false},
	{"echo \"a\\\"b\"", []string{"echo", `a"b`}, false},
	{"echo 'it'\\''s'", []string{"echo", "it's"}, false},
	{"", []string{}, false},
	{"   ", []string{}, false},
	{"cmd --flag=value --bool pos1 pos2",
		[]string{"cmd", "--flag=value", "--bool", "pos1", "pos2"}, false},
	{"echo unclosed 'quote", nil, true},
	{"echo \"dq\\\"q\"", []string{"echo", `dq"q`}, false},
	{"a#b", []string{"a#b"}, false},
	{"a #b", []string{"a", "#b"}, false},
	{"echo line1\\\nline2", []string{"echo", "line1\nline2"}, false},
	{"x '' y", []string{"x", "", "y"}, false},
	{"npm test -- --watch", []string{"npm", "test", "--", "--watch"}, false},
	{"echo a\\'b", []string{"echo", "a'b"}, false},
	{"echo '\\''", nil, true},
	{"echo x'y", nil, true},
	{"echo 'a'b'c'", []string{"echo", "abc"}, false},
	{"echo a'b", nil, true},
	{"echo 'it'\\''s' end", []string{"echo", "it's", "end"}, false},
	{"echo \\", nil, true},
	{"echo a\\", nil, true},
	{`echo "a\'b"`, []string{"echo", `a\'b`}, false},
	{`echo "a\bc"`, []string{"echo", `a\bc`}, false},
	{`echo "a\nb"`, []string{"echo", `a\nb`}, false},
	{`echo "a\$b"`, []string{"echo", `a\$b`}, false},
	{"echo a\\bc", []string{"echo", "abc"}, false},
	{"echo \"a\\\\b\"", []string{"echo", `a\b`}, false},
	{"echo 'a\"b'", []string{"echo", `a"b`}, false},
	{`echo "a\'b"`, []string{"echo", `a\'b`}, false},
	{"echo a#b#c", []string{"echo", "a#b#c"}, false},
	{`echo ""`, []string{"echo", ""}, false},
	{"echo -n", []string{"echo", "-n"}, false},
	{"echo --flag=x=y", []string{"echo", "--flag=x=y"}, false},
}

func TestSplitMatchesCPython(t *testing.T) {
	for _, tc := range shlexTruth {
		got, err := Split(tc.in)
		if tc.isErr {
			if err == nil {
				t.Errorf("Split(%q): want error, got %q", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("Split(%q): unexpected error %s", tc.in, err)
			continue
		}
		if !reflect.DeepEqual(got, tc.out) {
			t.Errorf("Split(%q) = %q, want %q", tc.in, got, tc.out)
		}
	}
}

func TestRunCommandPassFail(t *testing.T) {
	pass := RunCommand(t.TempDir(), "true", 30)
	if pass.Status != "pass" || pass.Exit != 0 {
		t.Fatalf("true must pass: %+v", pass)
	}
	fail := RunCommand(t.TempDir(), "false", 30)
	if fail.Status != "fail" || fail.Exit != 1 || fail.Detail != "exit 1" {
		t.Fatalf("false must fail exit 1: %+v", fail)
	}
	missing := RunCommand(t.TempDir(), "definitely-not-a-real-binary-xyz", 30)
	if missing.Status != "fail" || missing.Exit != 127 {
		t.Fatalf("missing binary must 127: %+v", missing)
	}
	empty := RunCommand(t.TempDir(), "   ", 30)
	if empty.Status != "fail" {
		t.Fatalf("empty command must fail: %+v", empty)
	}
}

func TestRunCommandTimeout(t *testing.T) {
	res := RunCommand(t.TempDir(), "sleep 30", 1)
	if res.Status != "fail" || res.Exit != 124 {
		t.Fatalf("timeout must fail 124: %+v", res)
	}
}

func TestResolveTestCommand(t *testing.T) {
	dir := t.TempDir()
	if _, _, _, ok := ResolveTestCommand(dir); ok {
		t.Fatal("no tests/ and no config must not resolve")
	}
}
