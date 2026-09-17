package cli

import "testing"

func TestTableCoversAllCommands(t *testing.T) {
	want := []string{"install", "init", "validate", "status", "doctor",
		"audit", "lock", "run", "approve", "verify", "adapters", "trace",
		"budget", "resume", "approvals", "add", "conformance", "pin",
		"upgrade", "characterize"}
	if len(Table) != len(want) {
		t.Fatalf("table has %d commands, want %d", len(Table), len(want))
	}
	for i, name := range want {
		if Table[i].Name != name {
			t.Fatalf("table[%d] = %q, want %q", i, Table[i].Name, name)
		}
		if Table[i].Help == "" {
			t.Fatalf("command %q has empty help", name)
		}
		wantWired := name == "validate" || name == "run" || name == "status" || name == "approvals" || name == "lock" || name == "verify" || name == "trace" || name == "approve" || name == "budget" || name == "resume"
		if (Table[i].Run != nil) != wantWired {
			t.Fatalf("command %q wired=%v, want %v", name, Table[i].Run != nil, wantWired)
		}
	}
}

func TestVersion(t *testing.T) {
	if got := Main([]string{"--version"}); got != ExitOK {
		t.Fatalf("version exit = %d, want %d", got, ExitOK)
	}
}

func TestBareInvocationFails(t *testing.T) {
	if got := Main(nil); got != ExitValidation {
		t.Fatalf("bare exit = %d, want %d", got, ExitValidation)
	}
}

func TestUnknownCommandFails(t *testing.T) {
	if got := Main([]string{"bogus"}); got != ExitValidation {
		t.Fatalf("unknown exit = %d, want %d", got, ExitValidation)
	}
}

func TestHelp(t *testing.T) {
	if got := Main([]string{"--help"}); got != ExitOK {
		t.Fatalf("help exit = %d, want %d", got, ExitOK)
	}
}
