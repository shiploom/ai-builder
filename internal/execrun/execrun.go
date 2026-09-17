// Package execrun runs shell-less commands with timeouts (stdlib only).
//
// Shared by gates (gate commands, scanners) and characterize (snapshot
// commands) so both observe identical execution semantics.
package execrun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/shiploom/ai-builder/internal/jsoncanon"
)

// DefaultTimeoutS mirrors gates.DEFAULT_TIMEOUT_S.
const DefaultTimeoutS = 600

// TailLimit mirrors gates.TAIL_LIMIT (runes, like Python string slices).
const TailLimit = 4000

// Result mirrors _run_command() dicts.
type Result struct {
	Status    string
	Command   string
	Exit      int
	DurationS float64
	Detail    string
	Tail      string
}

// ToMap renders the result mapping (key order handled by the writer).
func (r Result) ToMap() map[string]any {
	return map[string]any{
		"status": r.Status, "command": r.Command, "exit": int64(r.Exit),
		"durationS": r.DurationS, "detail": r.Detail, "tail": r.Tail,
	}
}

// Split mirrors shlex.split(s) posix defaults (no comments): single and
// double quotes, backslash escapes, whitespace separation. Verified
// against CPython, including: backslash-newline vanishes outside quotes
// but is kept inside double quotes; inside double quotes only " and \
// unescape; trailing lone backslash errors before unclosed quotes.
// Returns an error like "No closing quotation".
func Split(command string) ([]string, error) {
	const (
		stSpace = iota
		stWord
		stSingle
		stDouble
	)
	var out []string
	var buf strings.Builder
	state := stSpace
	escaped := false
	hasToken := false
	flush := func() {
		// Empty quoted strings ("") still produce a token.
		out = append(out, buf.String())
		buf.Reset()
		hasToken = false
	}
	for _, r := range command {
		if escaped {
			escaped = false
			hasToken = true
			if state == stDouble && r != '"' && r != '\\' {
				// Inside double quotes only " and \ unescape;
				// everything else (incl. newline) keeps the backslash.
				buf.WriteByte('\\')
			}
			buf.WriteRune(r)
			continue
		}
		switch state {
		case stSingle:
			if r == '\'' {
				state = stWord
			} else {
				buf.WriteRune(r)
			}
		case stDouble:
			switch r {
			case '"':
				state = stWord
			case '\\':
				escaped = true
			default:
				buf.WriteRune(r)
			}
			hasToken = true
		default: // stSpace, stWord
			switch {
			case r == '\'':
				state = stSingle
				hasToken = true
			case r == '"':
				state = stDouble
				hasToken = true
			case r == '\\':
				escaped = true
				hasToken = true
			case r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f':
				if state == stWord {
					flush()
					state = stSpace
				}
			default:
				state = stWord
				buf.WriteRune(r)
				hasToken = true
			}
		}
	}
	if escaped {
		return nil, fmt.Errorf("No escaped character")
	}
	if state == stSingle || state == stDouble {
		return nil, fmt.Errorf("No closing quotation")
	}
	if hasToken {
		flush()
	}
	if out == nil {
		out = []string{}
	}
	return out, nil
}

// RunCommand mirrors _run_command(): shell-less execution with timeout.
// An empty/unparseable command fails clean (Python tracebacks instead;
// documented divergence).
func RunCommand(projectDir, command string, timeoutS float64) Result {
	started := time.Now()
	argv, err := Split(command)
	if err != nil {
		return Result{Status: "fail", Command: command, Exit: 127,
			DurationS: elapsed(started), Detail: fmt.Sprintf("cannot parse command: %s", err)}
	}
	if len(argv) == 0 {
		return Result{Status: "fail", Command: command, Exit: 127,
			DurationS: elapsed(started), Detail: "empty command"}
	}
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(timeoutS*float64(time.Second)))
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = projectDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	out := stdout.String() + stderr.String()
	tail := tailRunes(out, TailLimit)
	duration := elapsed(started)
	if ctx.Err() == context.DeadlineExceeded {
		return Result{Status: "fail", Command: command, Exit: 124,
			DurationS: duration, Detail: fmt.Sprintf("timeout after %ds", int(timeoutS)),
			Tail: tail}
	}
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			return Result{Status: "fail", Command: command, Exit: code,
				DurationS: duration, Detail: fmt.Sprintf("exit %d", code), Tail: tail}
		}
		// Start failures (missing binary, permission): Python reports
		// "command not found" for FileNotFoundError and crashes otherwise;
		// Go reports cleanly either way (documented divergence).
		return Result{Status: "fail", Command: command, Exit: 127,
			DurationS: duration, Detail: "command not found", Tail: ""}
	}
	return Result{Status: "pass", Command: command, Exit: 0,
		DurationS: duration, Detail: "exit 0", Tail: tail}
}

func elapsed(started time.Time) float64 {
	// Rounding differs from CPython round(x, 2) on exact halves;
	// durations are normalized away in parity comparisons.
	return float64(int(time.Since(started).Seconds()*100+0.5)) / 100
}

func tailRunes(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[len(runes)-limit:])
}

// LoadGateConfig mirrors gates._configured(): the .shiploom/config.json
// "gates" mapping, or empty when absent/unreadable.
func LoadGateConfig(projectDir string) map[string]any {
	raw, err := os.ReadFile(filepath.Join(projectDir, ".shiploom", "config.json"))
	if err != nil {
		return map[string]any{}
	}
	doc, err := jsoncanon.Decode(raw)
	if err != nil {
		return map[string]any{}
	}
	obj, ok := doc.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	gates, ok := obj["gates"].(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return gates
}

// ResolveTestCommand mirrors the test branch of _gate_command().
// Divergence note: CPython runs its own interpreter (`sys.executable -m
// pytest`); Go probes PATH for pytest, then python3. Fixtures pin explicit
// commands so parity never depends on this difference.
func ResolveTestCommand(projectDir string) (command string, timeout float64, reason string, ok bool) {
	configured := LoadGateConfig(projectDir)
	if entry, present := configured["test"]; present {
		if obj, isMap := entry.(map[string]any); isMap {
			if cmd, _ := obj["command"].(string); cmd != "" {
				return cmd, gateTimeout(obj), "configured", true
			}
		} else if cmd, isStr := entry.(string); isStr && cmd != "" {
			return cmd, DefaultTimeoutS, "configured", true
		}
	}
	if _, err := os.Stat(filepath.Join(projectDir, "tests")); err == nil {
		if _, err := exec.LookPath("pytest"); err == nil {
			return "pytest -q", DefaultTimeoutS, "auto-detected pytest", true
		}
		if _, err := exec.LookPath("python3"); err == nil {
			return "python3 -m pytest -q", DefaultTimeoutS, "auto-detected pytest", true
		}
		return "", 0, "pytest not on PATH", false
	}
	return "", 0, "no tests/ directory", false
}

func gateTimeout(entry map[string]any) float64 {
	if v, ok := entry["timeoutS"]; ok {
		if f, ok := ToFloat(v); ok {
			return f
		}
	}
	return DefaultTimeoutS
}

// ToFloat coerces JSON numbers (plus Go ints/floats) for budget/
// timeout math. Non-numeric values report false.
func ToFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case int64:
		return float64(t), true
	case int:
		return float64(t), true
	case float64:
		return t, true
	case json.Number:
		if f, err := strconv.ParseFloat(string(t), 64); err == nil {
			return f, true
		}
		return 0, false
	default:
		// Non-numeric timeouts crash CPython (TypeError); Go falls back
		// to the default (documented divergence, malformed configs only).
		return 0, false
	}
}
