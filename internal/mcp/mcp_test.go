package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func testRegistry() map[string]any {
	return map[string]any{
		"capabilities": map[string]any{
			"cap.web.search": map[string]any{
				"providers": []any{"tavily"}, "trust": "community",
				"ttlS": int64(3600), "fallback": "fetch"},
			"cap.repo.symbols": map[string]any{
				"providers": []any{"local-lsp"}, "trust": "first-party", "ttlS": int64(30)},
		},
		"servers": map[string]any{
			"tavily": map[string]any{"transport": "http", "version": "1.2.0",
				"attested": false, "scopes": []any{"search:read"}, "auth": "env:TAVILY_KEY"},
			"fetch": map[string]any{"transport": "stdio", "version": "1.0.0",
				"attested": true, "scopes": []any{"fetch:read"}, "auth": "env:ALLOWLIST_ONLY"},
			"local-lsp": map[string]any{"transport": "stdio", "version": "1.0.0",
				"attested": true, "scopes": []any{"symbols:read"}, "auth": "env:EMPTY"},
		},
	}
}

func TestResolveChain(t *testing.T) {
	chain := Resolve(testRegistry(), "cap.web.search")
	if chain == nil {
		t.Fatal("want chain")
	}
	if len(chain.Providers) != 1 || chain.Providers[0] != "tavily" {
		t.Fatalf("providers wrong: %v", chain.Providers)
	}
	if !chain.HasFallback || chain.Fallback != "fetch" {
		t.Fatal("fallback missing")
	}
	if chain.Trust != "community" {
		t.Fatalf("trust wrong: %s", chain.Trust)
	}
	if !chain.Quarantine {
		t.Fatal("unattested provider must quarantine")
	}
	if chain.Attestation["tavily"] != "unattested" {
		t.Fatalf("attestation wrong: %v", chain.Attestation)
	}
	if Resolve(testRegistry(), "cap.nope") != nil || Resolve(nil, "cap.web.search") != nil {
		t.Fatal("unknown capability/doc must return nil")
	}
}

func TestResolveAttested(t *testing.T) {
	chain := Resolve(testRegistry(), "cap.repo.symbols")
	if chain == nil || chain.Quarantine {
		t.Fatalf("attested provider must not quarantine: %+v", chain)
	}
	if chain.Fallback != "" || chain.HasFallback {
		t.Fatal("no fallback expected")
	}
}

func TestAttestationSummary(t *testing.T) {
	summary := AttestationStatus(testRegistry())
	want := map[string]string{"tavily": "unattested", "fetch": "unverified", "local-lsp": "unverified"}
	for name, status := range want {
		if summary.Servers[name] != status {
			t.Fatalf("%s: got %s want %s", name, summary.Servers[name], status)
		}
	}
	if len(summary.UnverifiedAttested) != 2 {
		t.Fatalf("unverified list wrong: %v", summary.UnverifiedAttested)
	}
	if summary.Counts["unattested"] != 1 || summary.Counts["unverified"] != 2 {
		t.Fatalf("counts wrong: %v", summary.Counts)
	}
	if got := AttestationStatus(nil); len(got.Counts) != 0 {
		t.Fatalf("nil doc must yield empty summary: %v", got)
	}
}

func TestLoadRegistryErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadRegistry(dir); err == nil {
		t.Fatal("missing registry must error")
	}
	bad := filepath.Join(dir, ".shiploom", "mcp-registry.json")
	if err := os.MkdirAll(filepath.Dir(bad), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRegistry(dir); err == nil {
		t.Fatal("corrupt registry must error")
	}
}
