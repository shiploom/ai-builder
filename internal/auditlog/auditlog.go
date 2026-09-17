// Package auditlog ports cli/auditlog.py (stdlib only).
//
// Hash chaining is byte-compatible with Python: canonical JSON uses
// compact separators + sorted keys, and the digest input is
// prev + "\n" + canonical. File lines use spaced separators, like
// CPython json.dump defaults.
package auditlog

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shiploom/ai-builder/internal/jsoncanon"
	"github.com/shiploom/ai-builder/internal/manifest"
)

// AuditName is the append-only log file.
const AuditName = "audit.jsonl"

// GenesisPrev is the prev pointer of the first entry.
const GenesisPrev = "GENESIS"

// Entry mirrors the Python entry dict.
type Entry struct {
	Ts     string `json:"ts"`
	Actor  string `json:"actor"`
	Action string `json:"action"`
	Target string `json:"target"`
	Policy string `json:"policy"`
	Prev   string `json:"prev"`
	Hash   string `json:"hash"`
}

// AuditPath mirrors audit_path().
func AuditPath(projectDir string) string {
	return filepath.Join(projectDir, ".shiploom", AuditName)
}

func entryHash(prev string, body map[string]any) (string, error) {
	canonical, err := jsoncanon.MarshalCompact(body)
	if err != nil {
		return "", err
	}
	digest := sha256.New()
	digest.Write([]byte(prev))
	digest.Write([]byte("\n"))
	digest.Write(canonical)
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func makeEntry(prev, actor, action, target, policy, ts string) (map[string]any, error) {
	body := map[string]any{
		"ts": ts, "actor": actor, "action": action,
		"target": target, "policy": policy, "prev": prev,
	}
	hash, err := entryHash(prev, body)
	if err != nil {
		return nil, err
	}
	body["hash"] = "sha256:" + hash
	return body, nil
}

// InitLog creates a fresh audit log with a genesis entry (overwrites).
func InitLog(projectDir string) (map[string]any, error) {
	path := AuditPath(projectDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	genesis, err := makeEntry(GenesisPrev, "system", "log.genesis", ".", "-", manifest.Utcnow())
	if err != nil {
		return nil, err
	}
	line, err := jsoncanon.MarshalLine(genesis)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, append(line, '\n'), 0o644); err != nil {
		return nil, err
	}
	return genesis, nil
}

// ReadAll mirrors read_all().
func ReadAll(projectDir string) ([]map[string]any, error) {
	path := AuditPath(projectDir)
	raw, err := os.ReadFile(path)
	if err != nil {
		// Byte-parity with FileNotFoundError str() for the missing-file
		// case (same synthesis as run.LoadManifest); other OSError
		// texts differ (documented divergence).
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("[Errno 2] No such file or directory: '%s'", path)
		}
		return nil, err
	}
	var entries []map[string]any
	for i, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		doc, err := jsoncanon.Decode([]byte(line))
		if err != nil {
			return nil, fmt.Errorf("audit log corrupt at line %d: not JSON", i+1)
		}
		obj, ok := doc.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("audit log corrupt at line %d: not JSON", i+1)
		}
		entries = append(entries, obj)
	}
	return entries, nil
}

// Append mirrors append(): auto-inits a missing log, chains to the tail.
func Append(projectDir, actor, action, target, policy string) (map[string]any, error) {
	path := AuditPath(projectDir)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if _, err := InitLog(projectDir); err != nil {
			return nil, err
		}
	}
	entries, err := ReadAll(projectDir)
	if err != nil {
		return nil, err
	}
	prev := GenesisPrev
	if len(entries) > 0 {
		prev, _ = entries[len(entries)-1]["hash"].(string)
	}
	entry, err := makeEntry(prev, actor, action, target, policy, manifest.Utcnow())
	if err != nil {
		return nil, err
	}
	line, err := jsoncanon.MarshalLine(entry)
	if err != nil {
		return nil, err
	}
	fh, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	if _, err := fh.Write(append(line, '\n')); err != nil {
		return nil, err
	}
	return entry, nil
}

// Verify mirrors verify(): returns (ok, errors[]).
func Verify(projectDir string) (bool, []string) {
	entries, err := ReadAll(projectDir)
	if err != nil {
		return false, []string{fmt.Sprintf("unreadable audit log: %s", err)}
	}
	if len(entries) == 0 {
		return false, []string{"audit log is empty (missing genesis)"}
	}
	var errors []string
	prev := GenesisPrev
	keys := []string{"ts", "actor", "action", "target", "policy", "prev", "hash"}
	for i, entry := range entries {
		for _, key := range keys {
			if _, ok := entry[key]; !ok {
				errors = append(errors, fmt.Sprintf("line %d: missing key %s", i+1,
					quotePy(key)))
			}
		}
		eprev, _ := entry["prev"].(string)
		if eprev != prev {
			errors = append(errors, fmt.Sprintf(
				"line %d: prev-link broken (reordered or deleted entries?)", i+1))
			break
		}
		body := map[string]any{}
		for _, key := range []string{"ts", "actor", "action", "target", "policy", "prev"} {
			if v, ok := entry[key]; ok {
				body[key] = v
			}
		}
		want, err := entryHash(eprev, body)
		if err != nil {
			errors = append(errors, fmt.Sprintf("line %d: hash mismatch (tampered entry?)", i+1))
			break
		}
		if ehash, _ := entry["hash"].(string); ehash != "sha256:"+want {
			errors = append(errors, fmt.Sprintf("line %d: hash mismatch (tampered entry?)", i+1))
			break
		}
		if ehash, ok := entry["hash"].(string); ok {
			prev = ehash
		} else {
			prev = ""
		}
	}
	return len(errors) == 0, errors
}

// ExportMd mirrors export_md().
func ExportMd(projectDir string) (string, error) {
	entries, err := ReadAll(projectDir)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Audit log (%d entries)\n\n", len(entries))
	b.WriteString("| ts | actor | action | target | hash |\n")
	b.WriteString("|---|---|---|---|---|\n")
	for _, entry := range entries {
		hash, _ := entry["hash"].(string)
		short := hash
		if len(short) > 12 {
			short = short[len(short)-12:]
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | `…%s` |\n",
			strVal(entry["ts"]), strVal(entry["actor"]), strVal(entry["action"]),
			strVal(entry["target"]), short)
	}
	return b.String(), nil
}

// strVal mirrors Python "%s" over audit values (missing keys print None).
func strVal(v any) string {
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
		return fmt.Sprintf("%v", v)
	}
}

func quotePy(s string) string {
	return "'" + strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "'", "\\'") + "'"
}
