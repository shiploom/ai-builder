// Package mcp ports cli/mcp.py (stdlib only): capability resolution and
// attestation summaries over the project MCP registry.
package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/shiploom/ai-builder/internal/jsoncanon"
)

// RegistryName is the project registry file.
const RegistryName = "mcp-registry.json"

// RegistryPath mirrors registry_path().
func RegistryPath(projectDir string) string {
	return filepath.Join(projectDir, ".shiploom", RegistryName)
}

// LoadRegistry mirrors load_registry(). Raises as error on missing/bad.
func LoadRegistry(projectDir string) (map[string]any, error) {
	path := RegistryPath(projectDir)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unreadable MCP registry %s: %s", path, fsErrText(err))
	}
	doc, err := jsoncanon.Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("unreadable MCP registry %s: %s", path, err)
	}
	obj, ok := doc.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unreadable MCP registry %s: not an object", path)
	}
	caps, ok := obj["capabilities"].(map[string]any)
	if !ok || caps == nil {
		return nil, fmt.Errorf("MCP registry %s misses capabilities{}", path)
	}
	return obj, nil
}

// Chain mirrors resolve()'s return mapping.
type Chain struct {
	Capability  string
	Providers   []string
	TtlS        any
	Trust       string
	Fallback    string
	HasFallback bool
	Servers     map[string]any
	Attestation map[string]string
	Quarantine  bool
}

// Resolve mirrors resolve(doc, capability). Returns nil when unknown.
func Resolve(doc map[string]any, capability string) *Chain {
	if doc == nil {
		return nil
	}
	caps, _ := doc["capabilities"].(map[string]any)
	if caps == nil {
		return nil
	}
	capRaw, ok := caps[capability]
	if !ok {
		return nil
	}
	cap, ok := capRaw.(map[string]any)
	if !ok {
		return nil
	}
	chain := &Chain{Capability: capability, Servers: map[string]any{},
		Attestation: map[string]string{}}
	if providers, ok := cap["providers"].([]any); ok {
		for _, p := range providers {
			if name, ok := p.(string); ok {
				chain.Providers = append(chain.Providers, name)
			}
		}
	}
	if chain.Providers == nil {
		chain.Providers = []string{}
	}
	if ttl, ok := cap["ttlS"]; ok {
		chain.TtlS = ttl
	} else {
		chain.TtlS = int64(0)
	}
	if trust, ok := cap["trust"].(string); ok {
		chain.Trust = trust
	} else {
		chain.Trust = "community"
	}
	if fallback, ok := cap["fallback"].(string); ok {
		chain.Fallback = fallback
		chain.HasFallback = true
	}
	servers, _ := doc["servers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	involved := []map[string]any{}
	for _, name := range chain.Providers {
		entry, _ := servers[name].(map[string]any)
		chain.Servers[name] = servers[name]
		chain.Attestation[name] = serverStatus(entry)
		involved = append(involved, entry)
	}
	if chain.HasFallback {
		seen := false
		for _, name := range chain.Providers {
			if name == chain.Fallback {
				seen = true
			}
		}
		if !seen {
			entry, _ := servers[chain.Fallback].(map[string]any)
			involved = append(involved, entry)
		}
	}
	chain.Quarantine = false
	for _, entry := range involved {
		attested, _ := entry["attested"].(bool)
		if !attested {
			chain.Quarantine = true
			break
		}
	}
	return chain
}

func serverStatus(entry map[string]any) string {
	if entry == nil {
		return "unknown"
	}
	if attested, _ := entry["attested"].(bool); !attested {
		return "unattested"
	}
	if att, ok := entry["attestation"].(map[string]any); ok {
		if method, _ := att["method"].(string); method != "" {
			return "attested"
		}
	}
	return "unverified"
}

// AttestationSummary mirrors attestation_status().
type AttestationSummary struct {
	Servers            map[string]string
	UnverifiedAttested []string
	Counts             map[string]int
}

// AttestationStatus summarizes attestation across a registry document.
// Never raises on malformed input.
func AttestationStatus(doc map[string]any) AttestationSummary {
	summary := AttestationSummary{
		Servers: map[string]string{}, UnverifiedAttested: []string{}, Counts: map[string]int{},
	}
	var servers map[string]any
	if doc != nil {
		servers, _ = doc["servers"].(map[string]any)
	}
	if servers == nil {
		servers = map[string]any{}
	}
	for name, raw := range servers {
		entry, _ := raw.(map[string]any)
		status := serverStatus(entry)
		summary.Servers[name] = status
		summary.Counts[status]++
		if status == "unverified" {
			summary.UnverifiedAttested = append(summary.UnverifiedAttested, name)
		}
	}
	sort.Strings(summary.UnverifiedAttested)
	return summary
}

func fsErrText(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
