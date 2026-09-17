// Command bodies for `pin` and `upgrade` (stdlib only, P4).
package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/shiploom/ai-builder/internal/auditlog"
	"github.com/shiploom/ai-builder/internal/execrun"
	"github.com/shiploom/ai-builder/internal/jsoncanon"
	"github.com/shiploom/ai-builder/internal/manifest"
	"github.com/shiploom/ai-builder/internal/mcp"
	"github.com/shiploom/ai-builder/internal/run"
	"github.com/shiploom/ai-builder/internal/validate"
)

func init() {
	Register("pin", RunPin)
	Register("upgrade", RunUpgrade)
}

// osErrText mirrors FileNotFoundError str() for missing files (same
// synthesis as run.LoadManifest); other OSError texts differ.
func osErrText(path string, err error) string {
	if os.IsNotExist(err) {
		return fmt.Sprintf("[Errno 2] No such file or directory: '%s'", path)
	}
	return err.Error()
}

// RequiredManifestKeys mirrors REQUIRED_MANIFEST_KEYS (fixed order).
var RequiredManifestKeys = []string{"coreVersion", "workflow", "workflowVersion",
	"artifacts", "gates", "budgets", "retries", "checkpoints"}

// UpgradeBackup mirrors UPGRADE_BACKUP.
const UpgradeBackup = "upgrade-backup.json"

// toolCoreVersion reads the live tool core version (mirrors
// core_version(): TOOL_ROOT/core/VERSION via the schemas dir parent).
func toolCoreVersion(schemasDir string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(schemasDir), "core", "VERSION"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
}

// pythonDotVersion reports "X.Y.Z" for the operator python3.
func pythonDotVersion() string {
	for _, binary := range []string{"python3", "python"} {
		path, err := exec.LookPath(binary)
		if err != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		out, err := exec.CommandContext(ctx, path, "--version").Output()
		cancel()
		if err != nil {
			continue
		}
		text := strings.TrimSpace(string(out))
		if rest, ok := strings.CutPrefix(text, "Python "); ok {
			return strings.TrimSpace(rest)
		}
		if text != "" {
			return text
		}
	}
	return "unknown"
}

// toolProbe mirrors _tool_probe(): first output line of `<binary>
// --version`, else nil (10s cap).
func toolProbe(binary string) any {
	path, err := exec.LookPath(binary)
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--version")
	var stdout, stderr strings.Builder
	// Capture via buffers (no shell).
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Mirror Python: any run fault still yields combined output when
		// present; a total failure yields None.
		if stdout.Len() == 0 && stderr.Len() == 0 {
			return nil
		}
	}
	combined := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
	lines := strings.Split(combined, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return nil
	}
	first := strings.TrimSpace(lines[0])
	if len(first) > 120 {
		first = first[:120]
	}
	return first
}

// livePin builds the live reproducibility pin (mirrors cmd_pin's lock).
func livePin(schemasDir string, data map[string]any) map[string]any {
	core, err := toolCoreVersion(schemasDir)
	if err != nil {
		core = "unknown"
	}
	mcpVersions := map[string]any{}
	if registry, err := mcp.LoadRegistry("."); err == nil {
		if servers, ok := registry["servers"].(map[string]any); ok {
			for name, raw := range servers {
				entry, _ := raw.(map[string]any)
				if entry == nil {
					mcpVersions[name] = nil
				} else {
					mcpVersions[name] = entry["version"]
				}
			}
		}
	}
	return map[string]any{
		"coreVersion":  core,
		"manifestCore": data["coreVersion"],
		"python":       pythonDotVersion(),
		"platform":     runtime.GOOS,
		"harness":      map[string]any{"claude": toolProbe("claude"), "opencode": toolProbe("opencode")},
		"mcp":          mcpVersions,
		"models":       "BYO (harness-owned, never recorded)",
		"pinnedAt":     manifest.Utcnow(),
	}
}

// RunPin implements `pin [--check] [--json]`.
func RunPin(args []string) int {
	check := false
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--check":
			check = true
		case "--json":
			jsonOut = true
		case "--help", "-h":
			fmt.Println("usage: shiploom pin [--check] [--json]")
			fmt.Println()
			fmt.Println("Write/check ./.shiploom/lock.json reproducibility pin.")
			return ExitOK
		default:
			if strings.HasPrefix(arg, "-") {
				return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
			}
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		}
	}
	data, err := run.LoadManifest(".")
	if err != nil {
		return fail(err.Error(), ExitValidation)
	}
	schemasDir, serr := validate.DefaultSchemasDir()
	if serr != nil {
		return fail(serr.Error(), ExitValidation)
	}
	lock := livePin(schemasDir, data)
	lockPath := filepath.Join(".", ".shiploom", "lock.json")
	if check {
		raw, err := os.ReadFile(lockPath)
		if err != nil && os.IsNotExist(err) {
			return fail("not pinned (run shiploom pin)", ExitValidation)
		} else if err != nil {
			return fail(fmt.Sprintf("unreadable pin: %s", osErrText(lockPath, err)), ExitValidation)
		}
		pinned, err := jsoncanon.Decode(raw)
		if err != nil {
			return fail(fmt.Sprintf("unreadable pin: %s", err), ExitValidation)
		}
		pinnedObj, _ := pinned.(map[string]any)
		if pinnedObj == nil {
			return fail(fmt.Sprintf("unreadable pin: %s", "not an object"), ExitValidation)
		}
		var drifts []string
		for _, key := range []string{"coreVersion", "manifestCore", "python", "platform"} {
			if !jsonEqual(pinnedObj[key], lock[key]) {
				drifts = append(drifts, fmt.Sprintf("%s: pinned %s != live %s",
					key, validate.PyRepr(pinnedObj[key]), validate.PyRepr(lock[key])))
			}
		}
		liveHarness, _ := lock["harness"].(map[string]any)
		pinnedHarness, _ := pinnedObj["harness"].(map[string]any)
		if pinnedHarness == nil {
			pinnedHarness = map[string]any{}
		}
		for _, harness := range []string{"claude", "opencode"} {
			var liveV, pinnedV any
			if liveHarness != nil {
				liveV = liveHarness[harness]
			}
			pinnedV = pinnedHarness[harness]
			if !jsonEqual(pinnedV, liveV) {
				drifts = append(drifts, fmt.Sprintf("harness.%s: pinned %s != live %s",
					harness, validate.PyRepr(pinnedV), validate.PyRepr(liveV)))
			}
		}
		if !jsonEqual(pinnedObj["mcp"], lock["mcp"]) {
			drifts = append(drifts, "mcp registry changed")
		}
		if drifts == nil {
			drifts = []string{}
		}
		if jsonOut {
			items := make([]any, 0, len(drifts))
			for _, d := range drifts {
				items = append(items, d)
			}
			printJSON(map[string]any{"ok": len(drifts) == 0, "drifts": items, "live": lock})
			return codeFor(len(drifts) == 0)
		}
		if len(drifts) > 0 {
			for _, d := range drifts {
				fmt.Printf("  drift: %s\n", d)
			}
		} else {
			fmt.Printf("pin clean: %s\n", lockPath)
		}
		return codeFor(len(drifts) == 0)
	}
	raw, err := jsoncanon.Marshal(lock)
	if err != nil {
		return fail(err.Error(), ExitValidation)
	}
	if err := os.WriteFile(lockPath, append(raw, '\n'), 0o644); err != nil {
		return fail(err.Error(), ExitValidation)
	}
	if _, err := auditlog.Append(".", "human", "pin", "lock.json", "-"); err != nil {
		return fail(err.Error(), ExitValidation)
	}
	if jsonOut {
		printJSON(map[string]any{"ok": true, "lock": lock})
		return ExitOK
	}
	fmt.Printf("pinned %s\n", lockPath)
	return ExitOK
}

// jsonEqual compares decoded JSON values with Python == semantics for
// the pin scalar shapes (None/str/numbers/dicts). Numbers compare by
// literal-preserving float value; True == 1 like Python.
func jsonEqual(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if ab, aok := a.(bool); aok {
		if bb, bok := b.(bool); bok {
			return ab == bb
		}
		if bf, ok := numFloatAny(b); ok {
			var af float64
			if ab {
				af = 1
			}
			return af == bf
		}
		return false
	}
	if _, bok := b.(bool); bok {
		return jsonEqual(b, a)
	}
	if af, ok := numFloatAny(a); ok {
		if bf, ok := numFloatAny(b); ok {
			return af == bf
		}
		return false
	}
	if as_, ok := a.(string); ok {
		bs, ok := b.(string)
		return ok && as_ == bs
	}
	if am, ok := a.(map[string]any); ok {
		bm, ok := b.(map[string]any)
		if !ok || len(am) != len(bm) {
			return false
		}
		for k, v := range am {
			w, present := bm[k]
			if !present || !jsonEqual(v, w) {
				return false
			}
		}
		return true
	}
	if al, ok := a.([]any); ok {
		bl, ok := b.([]any)
		if !ok || len(al) != len(bl) {
			return false
		}
		for i := range al {
			if !jsonEqual(al[i], bl[i]) {
				return false
			}
		}
		return true
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func numFloatAny(v any) (float64, bool) {
	return execrun.ToFloat(v)
}

// RunUpgrade implements `upgrade [--dry-run] [--rollback] [--actor A] [--json]`.
func RunUpgrade(args []string) int {
	dryRun := false
	rollback := false
	actor := "human"
	jsonOut := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--dry-run":
			dryRun = true
		case arg == "--rollback":
			rollback = true
		case arg == "--json":
			jsonOut = true
		case arg == "--actor":
			if i+1 >= len(args) {
				return fail("argument --actor: expected one argument", ExitValidation)
			}
			i++
			actor = args[i]
		case strings.HasPrefix(arg, "--actor="):
			actor = strings.TrimPrefix(arg, "--actor=")
		case arg == "--help" || arg == "-h":
			fmt.Println("usage: shiploom upgrade [--dry-run] [--rollback] [--actor A] [--json]")
			fmt.Println()
			fmt.Println("Move project to the tool core version (backup + rollback).")
			return ExitOK
		case strings.HasPrefix(arg, "-"):
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		default:
			return fail(fmt.Sprintf("unrecognized arguments: %s", arg), ExitValidation)
		}
	}
	data, err := run.LoadManifest(".")
	if err != nil {
		return fail(err.Error(), ExitValidation)
	}
	schemasDir, serr := validate.DefaultSchemasDir()
	if serr != nil {
		return fail(serr.Error(), ExitValidation)
	}
	dot := filepath.Join(".", ".shiploom")
	configPath := filepath.Join(dot, "config.json")
	configRaw, err := os.ReadFile(configPath)
	if err != nil {
		return fail(fmt.Sprintf("unreadable config: %s", osErrText(configPath, err)), ExitValidation)
	}
	configDoc, err := jsoncanon.Decode(configRaw)
	if err != nil {
		return fail(fmt.Sprintf("unreadable config: %s", err), ExitValidation)
	}
	config, _ := configDoc.(map[string]any)
	if config == nil {
		return fail("unreadable config: not an object", ExitValidation)
	}
	backupPath := filepath.Join(dot, UpgradeBackup)
	if rollback {
		raw, err := os.ReadFile(backupPath)
		if err != nil && os.IsNotExist(err) {
			return fail("no upgrade backup (nothing to roll back)", ExitValidation)
		} else if err != nil {
			return fail(fmt.Sprintf("unreadable backup: %s", osErrText(backupPath, err)), ExitValidation)
		}
		backupDoc, err := jsoncanon.Decode(raw)
		if err != nil {
			return fail(fmt.Sprintf("unreadable backup: %s", err), ExitValidation)
		}
		backup, _ := backupDoc.(map[string]any)
		backupConfig, _ := backup["config"].(map[string]any)
		backupManifest, _ := backup["manifest"].(map[string]any)
		if backup == nil || backupConfig == nil || backupManifest == nil {
			// Mirror KeyError str(): repr of the missing key.
			key := "config"
			if backup != nil && backupConfig != nil {
				key = "manifest"
			}
			return fail(fmt.Sprintf("unreadable backup: %s", validate.PyRepr(key)), ExitValidation)
		}
		writeJSONFile(configPath, backupConfig)
		if _, err := manifest.Save(".", backupManifest); err != nil {
			return fail(err.Error(), ExitValidation)
		}
		// Mirror backup.get("from", "?") exactly.
		fromDisplay := "?"
		if f, ok := backup["from"]; ok {
			fromDisplay = validate.PyStr(f)
		}
		if _, err := auditlog.Append(".", actor, "upgrade.rollback", fromDisplay, "-"); err != nil {
			return fail(err.Error(), ExitValidation)
		}
		os.Remove(backupPath)
		if jsonOut {
			restored := backup["from"]
			printJSON(map[string]any{"ok": true, "restored": restored})
			return ExitOK
		}
		fmt.Printf("rolled back to core %s\n", fromDisplay)
		return ExitOK
	}
	target, err := toolCoreVersion(schemasDir)
	if err != nil {
		return fail(fmt.Sprintf("unreadable tool version: %s", err), ExitValidation)
	}
	current := data["coreVersion"]
	var missing []string
	for _, k := range RequiredManifestKeys {
		if _, ok := data[k]; !ok {
			missing = append(missing, k)
		}
	}
	var changes []string
	if !jsonEqual(current, target) {
		changes = append(changes,
			fmt.Sprintf("./.shiploom/config.json: coreVersion %s -> %s",
				validate.PyRepr(current), validate.PyRepr(target)),
			fmt.Sprintf("./.shiploom/manifest.json: coreVersion %s -> %s",
				validate.PyRepr(current), validate.PyRepr(target)))
	}
	if dryRun {
		missingAny := make([]any, 0, len(missing))
		for _, k := range missing {
			missingAny = append(missingAny, k)
		}
		changesAny := make([]any, 0, len(changes))
		for _, c := range changes {
			changesAny = append(changesAny, c)
		}
		if jsonOut {
			printJSON(map[string]any{
				"ok": len(missing) == 0, "current": current, "target": target,
				"manifestCompat": len(missing) == 0, "missingKeys": missingAny,
				"changes": changesAny,
			})
			return codeFor(len(missing) == 0)
		}
		fmt.Printf("upgrade %s -> %s\n", validate.PyStr(current), validate.PyStr(target))
		if len(missing) > 0 {
			fmt.Printf("  incompatible manifest, missing: %s\n", strings.Join(missing, ", "))
		}
		if len(changes) > 0 {
			for _, c := range changes {
				fmt.Printf("  would change %s\n", c)
			}
		} else {
			fmt.Println("  already current, no changes")
		}
		return codeFor(len(missing) == 0)
	}
	if len(missing) > 0 {
		return fail(fmt.Sprintf("incompatible manifest, missing: %s (re-init or repair first)",
			strings.Join(missing, ", ")), ExitValidation)
	}
	if jsonEqual(current, target) {
		fmt.Printf("already at core %s\n", target)
		if jsonOut {
			printJSON(map[string]any{
				"ok": true, "current": current, "target": target, "changes": []any{},
			})
			return ExitOK
		}
		return ExitOK
	}
	backup := map[string]any{"from": current, "config": config, "manifest": data}
	if err := writeJSONFile(backupPath, backup); err != nil {
		return fail(err.Error(), ExitValidation)
	}
	config["coreVersion"] = target
	if err := writeJSONFile(configPath, config); err != nil {
		return fail(err.Error(), ExitValidation)
	}
	data["coreVersion"] = target
	if _, err := manifest.Save(".", data); err != nil {
		return fail(err.Error(), ExitValidation)
	}
	if _, err := auditlog.Append(".", actor, "upgrade",
		fmt.Sprintf("%s -> %s", validate.PyStr(current), validate.PyStr(target)), "-"); err != nil {
		return fail(err.Error(), ExitValidation)
	}
	schemas := validate.NewSchemas(schemasDir)
	verr, _, _ := validate.ValidatePath(schemas, ".", true)
	var filtered []validate.Entry
	for _, e := range verr {
		if !strings.Contains(e.Path, ".shiploom") {
			filtered = append(filtered, e)
		}
	}
	if len(filtered) > 0 {
		// Auto-rollback on failed validation.
		if brow, err := os.ReadFile(backupPath); err == nil {
			if bdoc, err := jsoncanon.Decode(brow); err == nil {
				if bobj, ok := bdoc.(map[string]any); ok {
					if bc, ok := bobj["config"].(map[string]any); ok {
						writeJSONFile(configPath, bc)
					}
					if bm, ok := bobj["manifest"].(map[string]any); ok {
						manifest.Save(".", bm)
					}
				}
			}
		}
		auditlog.Append(".", "system", "upgrade.rollback", "auto after failed validation", "-")
		fmt.Println("upgrade failed validation, rolled back:")
		top := filtered
		if len(top) > 5 {
			top = top[:5]
		}
		for _, e := range top {
			fmt.Printf("  fail: %s: %s\n", e.Path, e.Message)
		}
		return ExitValidation
	}
	if jsonOut {
		changesAny := make([]any, 0, len(changes))
		for _, c := range changes {
			changesAny = append(changesAny, c)
		}
		printJSON(map[string]any{
			"ok": true, "current": current, "target": target, "changes": changesAny,
		})
		return ExitOK
	}
	fmt.Printf("upgraded %s -> %s (backup at %s)\n",
		validate.PyStr(current), validate.PyStr(target), backupPath)
	return ExitOK
}

func writeJSONFile(path string, doc map[string]any) error {
	raw, err := jsoncanon.Marshal(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}
