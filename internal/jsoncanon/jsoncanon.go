// Package jsoncanon renders JSON byte-identical to Python's json module
// with sort_keys=True (stdlib only).
//
// Two shapes are needed because CPython uses different separators:
//   - File:  json.dump(sort_keys=True, indent=2) — spaced, indented.
//   - Hash:  json.dumps(sort_keys=True, separators=(",", ":")) — compact.
//
// Both use ensure_ascii=True string escaping.
//
// Supported values: map[string]any, []any, string, bool, nil,
// json.Number (emitted verbatim), int/int64, float64 (Python repr).
// Anything else is an error — callers decode with UseNumber so floats
// arrive as json.Number and no precision is lost.
package jsoncanon

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Marshal renders v like json.dump(sort_keys=True, indent=2).
func Marshal(v any) ([]byte, error) {
	var b strings.Builder
	if err := writeValue(&b, v, 0); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

// MarshalCompact renders v like json.dumps(sort_keys=True,
// separators=(",", ":")).
func MarshalCompact(v any) ([]byte, error) {
	var b strings.Builder
	if err := writeCompact(&b, v); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

// Decode parses JSON preserving number literals (no float64 drift).
func Decode(data []byte) (any, error) {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, fmt.Errorf("extra data after JSON document")
	}
	return v, nil
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func indent(b *strings.Builder, level int) {
	for i := 0; i < level; i++ {
		b.WriteString("  ")
	}
}

func writeValue(b *strings.Builder, v any, level int) error {
	switch t := v.(type) {
	case map[string]any:
		if len(t) == 0 {
			b.WriteString("{}")
			return nil
		}
		b.WriteString("{\n")
		keys := sortedKeys(t)
		for i, k := range keys {
			indent(b, level+1)
			appendJSONString(b, k)
			b.WriteString(": ")
			if err := writeValue(b, t[k], level+1); err != nil {
				return err
			}
			if i < len(keys)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		indent(b, level)
		b.WriteString("}")
		return nil
	case []any:
		if len(t) == 0 {
			b.WriteString("[]")
			return nil
		}
		b.WriteString("[\n")
		for i, item := range t {
			indent(b, level+1)
			if err := writeValue(b, item, level+1); err != nil {
				return err
			}
			if i < len(t)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		indent(b, level)
		b.WriteString("]")
		return nil
	case string:
		appendJSONString(b, t)
		return nil
	case bool:
		if t {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
		return nil
	case nil:
		b.WriteString("null")
		return nil
	case json.Number:
		if !validNumberLiteral(string(t)) {
			return fmt.Errorf("invalid number literal %q", string(t))
		}
		b.WriteString(string(t))
		return nil
	case int:
		b.WriteString(strconv.Itoa(t))
		return nil
	case int64:
		b.WriteString(strconv.FormatInt(t, 10))
		return nil
	case float64:
		b.WriteString(pyFloat(t))
		return nil
	default:
		return fmt.Errorf("unsupported type %T", v)
	}
}

func writeCompact(b *strings.Builder, v any) error {
	switch t := v.(type) {
	case map[string]any:
		if len(t) == 0 {
			b.WriteString("{}")
			return nil
		}
		b.WriteString("{")
		keys := sortedKeys(t)
		for i, k := range keys {
			if i > 0 {
				b.WriteString(",")
			}
			appendJSONString(b, k)
			b.WriteString(":")
			if err := writeCompact(b, t[k]); err != nil {
				return err
			}
		}
		b.WriteString("}")
		return nil
	case []any:
		if len(t) == 0 {
			b.WriteString("[]")
			return nil
		}
		b.WriteString("[")
		for i, item := range t {
			if i > 0 {
				b.WriteString(",")
			}
			if err := writeCompact(b, item); err != nil {
				return err
			}
		}
		b.WriteString("]")
		return nil
	case string:
		appendJSONString(b, t)
		return nil
	case bool:
		if t {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
		return nil
	case nil:
		b.WriteString("null")
		return nil
	case json.Number:
		if !validNumberLiteral(string(t)) {
			return fmt.Errorf("invalid number literal %q", string(t))
		}
		b.WriteString(string(t))
		return nil
	case int:
		b.WriteString(strconv.Itoa(t))
		return nil
	case int64:
		b.WriteString(strconv.FormatInt(t, 10))
		return nil
	case float64:
		b.WriteString(pyFloat(t))
		return nil
	default:
		return fmt.Errorf("unsupported type %T", v)
	}
}

func validNumberLiteral(s string) bool {
	if s == "" {
		return false
	}
	// Accept whatever encoding/json itself would emit.
	var v any
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return false
	}
	return !dec.More()
}

// pyFloat formats f like CPython repr(): shortest round-trip, ".0"
// suffix on integral values below 1e16, scientific outside [-4, 16).
func pyFloat(f float64) string {
	if math.IsNaN(f) {
		return "NaN"
	}
	if math.IsInf(f, 1) {
		return "Infinity"
	}
	if math.IsInf(f, -1) {
		return "-Infinity"
	}
	neg := ""
	if math.Signbit(f) {
		neg = "-"
		f = -f
	}
	if f == math.Trunc(f) && f < 1e16 {
		return neg + strconv.FormatInt(int64(f), 10) + ".0"
	}
	sci := strconv.FormatFloat(f, 'e', -1, 64) // d[.ddd]e±XX
	ePos := strings.LastIndexByte(sci, 'e')
	mant, exp := sci[:ePos], sci[ePos+1:]
	expVal, _ := strconv.Atoi(exp)
	digits := strings.Replace(mant, ".", "", 1)
	if expVal >= -4 && expVal < 16 {
		point := expVal + 1
		var out string
		switch {
		case point <= 0:
			out = "0." + strings.Repeat("0", -point) + digits
		case point >= len(digits):
			out = digits + strings.Repeat("0", point-len(digits))
		default:
			out = digits[:point] + "." + digits[point:]
		}
		if !strings.Contains(out, ".") {
			out += ".0"
		}
		return neg + out
	}
	intPart := digits[:1]
	fracPart := strings.TrimRight(digits[1:], "0")
	m := intPart
	if fracPart != "" {
		m += "." + fracPart
	}
	return neg + m + "e" + exp
}

// appendJSONString escapes like json.dumps(ensure_ascii=True): short
// escapes where CPython defines them, \u00xx for other controls and
// DEL, \uXXXX (lowercase, surrogate pairs) above ASCII.
func appendJSONString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		case '\b':
			b.WriteString("\\b")
		case '\f':
			b.WriteString("\\f")
		default:
			switch {
			case r < 0x20 || r == 0x7f:
				fmt.Fprintf(b, "\\u%04x", r)
			case r < 0x80:
				b.WriteRune(r)
			case r <= 0xffff:
				fmt.Fprintf(b, "\\u%04x", r)
			default:
				r -= 0x10000
				fmt.Fprintf(b, "\\u%04x\\u%04x", 0xd800+(r>>10), 0xdc00+(r&0x3ff))
			}
		}
	}
	b.WriteByte('"')
}
