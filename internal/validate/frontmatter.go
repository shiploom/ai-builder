package validate

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// fmLine is one preprocessed frontmatter line: indent width, text, or blank.
type fmLine struct {
	indent int
	text   string
	blank  bool
}

func parseScalar(raw string) (any, error) {
	s := strings.TrimSpace(raw)
	if s == "" || s == "~" || strings.ToLower(s) == "null" {
		return nil, nil
	}
	if len(s) >= 2 && s[0] == s[len(s)-1] && (s[0] == '\'' || s[0] == '"') {
		return s[1 : len(s)-1], nil
	}
	if low := strings.ToLower(s); low == "true" {
		return true, nil
	} else if low == "false" {
		return false, nil
	}
	if strings.HasPrefix(s, "[") && !strings.HasSuffix(s, "]") {
		return nil, fmt.Errorf("unclosed flow list: %s", pyRepr(raw))
	}
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		inner := strings.TrimSpace(s[1 : len(s)-1])
		if inner == "" {
			return []any{}, nil
		}
		var parts []any
		var buf strings.Builder
		var quote rune
		inQuote := false
		for _, ch := range inner {
			switch {
			case inQuote:
				buf.WriteRune(ch)
				if ch == quote {
					inQuote = false
				}
			case ch == '\'' || ch == '"':
				inQuote = true
				quote = ch
				buf.WriteRune(ch)
			case ch == ',':
				part, err := parseScalar(buf.String())
				if err != nil {
					return nil, err
				}
				parts = append(parts, part)
				buf.Reset()
			default:
				buf.WriteRune(ch)
			}
		}
		if strings.TrimSpace(buf.String()) != "" {
			part, err := parseScalar(buf.String())
			if err != nil {
				return nil, err
			}
			parts = append(parts, part)
		}
		if parts == nil {
			parts = []any{}
		}
		return parts, nil
	}
	if n, ok := parsePyInt(s); ok {
		return n, nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		// Mirror CPython float(): reject hex floats and underscores,
		// which Go would otherwise accept.
		lower := strings.ToLower(s)
		if strings.ContainsAny(lower, "x") || strings.Contains(s, "_") {
			return s, nil
		}
		_ = lower
		return f, nil
	}
	return s, nil
}

// parsePyInt mirrors CPython int(s) for frontmatter scalars: optional
// sign, digits, single underscores between digits.
func parsePyInt(s string) (any, bool) {
	t := s
	if strings.HasPrefix(t, "+") || strings.HasPrefix(t, "-") {
		t = t[1:]
	}
	if t == "" {
		return nil, false
	}
	digits := 0
	prevUnderscore := false
	for i, r := range t {
		switch {
		case r >= '0' && r <= '9':
			digits++
			prevUnderscore = false
		case r == '_' && i > 0 && !prevUnderscore:
			prevUnderscore = true
		default:
			return nil, false
		}
		_ = i
	}
	if digits == 0 || prevUnderscore {
		return nil, false
	}
	clean := strings.ReplaceAll(t, "_", "")
	n, err := strconv.ParseInt(clean, 10, 64)
	if err != nil {
		// Beyond int64: keep the literal as an integer-typed number
		// (mirrors Python bigints for type checks and %r output).
		if _, ferr := strconv.ParseFloat(clean, 64); ferr == nil {
			return json.Number(clean), true
		}
		return nil, false
	}
	if strings.HasPrefix(s, "-") {
		return -n, true
	}
	return n, true
}

func parseMapping(lines []fmLine, pos, indent int) (map[string]any, int, error) {
	out := map[string]any{}
	n := len(lines)
	for pos < n {
		line := lines[pos]
		if line.blank {
			// Blank/comment lines carry no indent; the indexed list
			// this port builds drops them, matching Python's filter.
			pos++
			continue
		}
		if line.indent != indent {
			break
		}
		text := line.text
		if strings.HasPrefix(text, "- ") || text == "-" {
			break
		}
		if !strings.Contains(text, ":") {
			break
		}
		key, rest, _ := strings.Cut(text, ":")
		key = strings.TrimSpace(key)
		rest = strings.TrimSpace(rest)
		pos++
		if rest != "" {
			val, err := parseScalar(rest)
			if err != nil {
				return nil, pos, err
			}
			out[key] = val
		} else {
			for pos < n && lines[pos].blank {
				pos++
			}
			if pos >= n || lines[pos].indent <= indent {
				out[key] = nil
			} else if strings.HasPrefix(lines[pos].text, "- ") || lines[pos].text == "-" {
				val, next, err := parseList(lines, pos, lines[pos].indent)
				if err != nil {
					return nil, pos, err
				}
				pos = next
				out[key] = val
			} else {
				val, next, err := parseMapping(lines, pos, lines[pos].indent)
				if err != nil {
					return nil, pos, err
				}
				pos = next
				out[key] = val
			}
		}
	}
	return out, pos, nil
}

func parseList(lines []fmLine, pos, indent int) ([]any, int, error) {
	var out []any
	n := len(lines)
	for pos < n {
		line := lines[pos]
		if line.blank {
			pos++
			continue
		}
		text := line.text
		if line.indent != indent || !(strings.HasPrefix(text, "- ") || text == "-") {
			break
		}
		itemText := ""
		if text != "-" {
			itemText = strings.TrimSpace(text[1:])
		}
		pos++
		if itemText == "" {
			for pos < n && lines[pos].blank {
				pos++
			}
			if pos < n && lines[pos].indent > indent {
				if strings.HasPrefix(lines[pos].text, "- ") || lines[pos].text == "-" {
					val, next, err := parseList(lines, pos, lines[pos].indent)
					if err != nil {
						return nil, pos, err
					}
					pos = next
					out = append(out, val)
				} else {
					val, next, err := parseMapping(lines, pos, lines[pos].indent)
					if err != nil {
						return nil, pos, err
					}
					pos = next
					out = append(out, val)
				}
			} else {
				out = append(out, nil)
			}
		} else if strings.Contains(itemText, ":") && !strings.HasPrefix(itemText, "[") {
			key, rest, _ := strings.Cut(itemText, ":")
			first, err := parseScalar(strings.TrimSpace(rest))
			if err != nil {
				return nil, pos, err
			}
			item := map[string]any{strings.TrimSpace(key): first}
			for pos < n && lines[pos].blank {
				pos++
			}
			for pos < n && lines[pos].indent > indent {
				ctext := lines[pos].text
				if strings.HasPrefix(ctext, "- ") || ctext == "-" {
					break
				}
				if !strings.Contains(ctext, ":") {
					break
				}
				k2, r2, _ := strings.Cut(ctext, ":")
				v2, err := parseScalar(r2)
				if err != nil {
					return nil, pos, err
				}
				item[strings.TrimSpace(k2)] = v2
				pos++
				cind := lines[pos-1].indent
				if pos < n && !lines[pos].blank && lines[pos].indent > cind {
					// Deeper than one level: unsupported; stop list
					// parsing so the caller reports unparsed lines.
					break
				}
			}
			out = append(out, item)
		} else {
			val, err := parseScalar(itemText)
			if err != nil {
				return nil, pos, err
			}
			out = append(out, val)
		}
	}
	if out == nil {
		out = []any{}
	}
	return out, pos, nil
}

// ParseFrontmatter splits Markdown frontmatter.
// Returns (nil, text, "") when no block exists (skip, not error);
// (nil, body, err) on malformed input.
func ParseFrontmatter(text string) (map[string]any, string, string) {
	lines := splitLines(text)
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, text, ""
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if t := strings.TrimSpace(lines[i]); t == "---" || t == "..." {
			end = i
			break
		}
	}
	if end == -1 {
		return nil, strings.Join(lines[1:], "\n"), "unterminated frontmatter block"
	}
	fmLines, body := lines[1:end], strings.Join(lines[end+1:], "\n")

	var indexed []fmLine
	for _, raw := range fmLines {
		if strings.TrimSpace(raw) == "" || isCommentLine(raw) {
			continue
		}
		nospace := strings.TrimLeft(raw, " ")
		if nospace != trimLeftUnicode(raw) {
			return nil, body, "tabs not allowed in frontmatter indentation"
		}
		stripped := strings.TrimSpace(stripComment(nospace))
		indexed = append(indexed, fmLine{indent: len(raw) - len(nospace), text: stripped})
	}
	if len(indexed) == 0 {
		return nil, body, "empty frontmatter block"
	}
	data, pos, err := parseMapping(indexed, 0, indexed[0].indent)
	if err != nil {
		return nil, body, "frontmatter parse failure: " + err.Error()
	}
	if pos != len(indexed) {
		return nil, body, fmt.Sprintf("unparsed frontmatter near: %s", pyRepr(indexed[pos].text))
	}
	return data, body, ""
}

func splitLines(text string) []string {
	// Mirror str.splitlines for \n files; strip \r (Python drops it).
	raw := strings.Split(text, "\n")
	for i := range raw {
		raw[i] = strings.TrimSuffix(raw[i], "\r")
	}
	return raw
}

func isCommentLine(raw string) bool {
	return strings.HasPrefix(trimLeftUnicode(raw), "#")
}

func trimLeftUnicode(s string) string {
	return strings.TrimLeftFunc(s, unicodeIsSpace)
}

func unicodeIsSpace(r rune) bool {
	// Approximation of Python str.lstrip() whitespace set.
	switch {
	case r == ' ' || r == '\t' || r == '\n' || r == '\v' || r == '\f' || r == '\r':
		return true
	case r >= '\x1c' && r <= '\x1f', r == '\x85':
		return true
	default:
		return false
	}
}

func stripComment(line string) string {
	// Full-line comments only (caller guarantees); inline '#' is legal.
	return line
}
