package gates

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// secretsTruth mirrors CPython SECRET_RULES, generated 2026-09-17:
// line -> first matching rule ("-" means clean).
var secretsTruth = []struct {
	line string
	rule string
}{
	{"-----BEGIN RSA PRIVATE KEY-----", "private-key"},
	{"-----BEGIN OPENSSH PRIVATE KEY-----", "private-key"},
	{"-----BEGIN CERTIFICATE-----", "-"},
	{"AKIAIOSFODNN7EXAMPLE", "aws-key"},
	{"AKIAIOSFODNN7EXAMPL", "-"},
	{"token ghp_abcdefgh12345678 here", "token-prefix"},
	{"token short ghp_abc here", "-"},
	{"xoxb-123456789012-ab", "token-prefix"},
	{`password = "hunter2hunter"`, "secret-assign"},
	{"password = hunter2", "-"},
	{"api_key='AK'", "-"},
	{`api_key = "abcdef12345"`, "secret-assign"},
	{"SECRET: 's3cr3t!x'", "secret-assign"},
	{"the password is hunter2", "-"},
	{"postgres://bob:s3cret@db:5432/app", "conn-string"},
	{"postgres://bob@db/app", "-"},
	{"http://example.com", "-"},
	{"mysql://u:p@h/db", "conn-string"},
}

func TestSecretsCorpus(t *testing.T) {
	rules := SecretRules()
	for _, tc := range secretsTruth {
		var got string = "-"
		for _, rule := range rules {
			if rule.Re.MatchString(tc.line) {
				got = rule.Name
				break
			}
		}
		if got != tc.rule {
			t.Errorf("line %q: got %s want %s", tc.line, got, tc.rule)
		}
	}
}

func TestScanSecretsFindings(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app.py"), "password = \"hunter2hunter\"\n")
	writeFile(t, filepath.Join(dir, "key.pem"), "-----BEGIN RSA PRIVATE KEY-----\nabc\n")
	writeFile(t, filepath.Join(dir, ".venv", "app.py"), "password = \"hunter2hunter\"\n")
	if err := os.WriteFile(filepath.Join(dir, "blob.bin"),
		[]byte("\x00password = \"hunter2hunter\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	findings := ScanSecrets(dir)
	if len(findings) != 2 {
		t.Fatalf("want 2 findings, got %v", findings)
	}
	if findings[0].Path != "app.py" || findings[0].Rule != "secret-assign" {
		t.Fatalf("first finding wrong: %+v", findings[0])
	}
	if findings[1].Path != "key.pem" || findings[1].Rule != "private-key" {
		t.Fatalf("second finding wrong: %+v", findings[1])
	}
	for _, f := range findings {
		if f.Path == "blob.bin" || len(f.Path) >= 5 && f.Path[:5] == ".venv" {
			t.Fatalf("excluded paths must not appear: %+v", f)
		}
	}
}

func TestConfiguredGates(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `{"sast": {"command": "true", "timeoutS": 30}}}`)
	report := RunGates(dir, []string{"sast"})
	if report.Gates["sast"].Status != "pass" {
		t.Fatalf("sast must pass: %+v", report.Gates["sast"])
	}
	writeConfig(t, dir, `{"sast": {"command": "false", "timeoutS": 30}}}`)
	report = RunGates(dir, []string{"sast"})
	if report.Gates["sast"].Status != "fail" {
		t.Fatalf("sast must fail: %+v", report.Gates["sast"])
	}
	report = RunGates(dir, []string{"license", "mutation"})
	if _, ok := report.Gates["sast"]; ok {
		t.Fatal("unselected gate must be absent")
	}
	report = RunGates(dir, []string{"nope"})
	if report.Ok || len(report.Errors) == 0 {
		t.Fatalf("unknown gate must fail: %+v", report)
	}
}

func writeConfig(t *testing.T, dir, gatesJSON string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, ".shiploom", "config.json"),
		`{"coreVersion": "1.1.0", "harness": "auto", "gates": `+gatesJSON+`}`)
}

func TestLicenseInventory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package-lock.json"), `{"name": "x", "packages": {
		"": {"name": "x"},
		"node_modules/left-pad": {"version": "1.3.0", "license": "MIT"},
		"node_modules/evil": {"version": "9.9.9", "license": "GPL-3.0-only"},
		"node_modules/mystery": {"version": "1.0.0"}}}`)
	writeFile(t, filepath.Join(dir, "package.json"),
		`{"dependencies": {"left-pad": "1.3.0"}}`)
	writeFile(t, filepath.Join(dir, "requirements.txt"), "foo\n")
	declared, unknown, nonAllowlisted := LicenseInventory(dir)
	if declared["npm:left-pad"] != "MIT" || declared["npm:evil"] != "GPL-3.0-only" {
		t.Fatalf("declared wrong: %v", declared)
	}
	if len(unknown) != 1 || unknown[0] != "pip:foo" {
		t.Fatalf("unknown wrong: %v", unknown)
	}
	if len(nonAllowlisted) != 1 || nonAllowlisted[0] != "npm:evil" {
		t.Fatalf("non-allowlisted wrong: %v", nonAllowlisted)
	}
	report := RunGates(dir, []string{"license"})
	if report.Gates["license"].Status != "pass" {
		t.Fatal("license is report-only, must pass")
	}
	if _, ok := report.Warnings["license"]; !ok {
		t.Fatal("non-allowlisted must warn")
	}
}

func TestMutationSampler(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not on PATH")
	}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mod.py"),
		"def f(a, b):\n    if a == b and b > 0:\n        return True\n    return a + b\n")
	candidates, sample, err := MutationSample(dir, 20)
	if err != nil {
		t.Fatal(err)
	}
	if candidates != 5 {
		t.Fatalf("want 5 candidates, got %d (%v)", candidates, sample)
	}
	kinds := map[string]bool{}
	for _, s := range sample {
		kinds[s["kind"].(string)] = true
	}
	for _, want := range []string{"comparison", "boolean-op", "boolean-return", "arithmetic"} {
		if !kinds[want] {
			t.Fatalf("missing kind %s in %v", want, sample)
		}
	}
}

func TestQualityAndDeterminism(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `{"test": {"command": "true", "timeoutS": 30}}`)
	report := RunGates(dir, nil)
	if !report.Ok || report.Verdict != "pass" {
		t.Fatalf("passing tree must pass: %+v", report)
	}
	if report.Quality["determinism"] != "pass (2 identical runs)" {
		t.Fatalf("determinism wrong with passing tests: %v", report.Quality)
	}
	if report.Quality["tests"] != "pass" || report.Quality["license"] != "pass" {
		t.Fatalf("quality wrong: %v", report.Quality)
	}
}
