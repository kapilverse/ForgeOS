package builder

import (
	"strconv"
	"strings"
	"unicode"
)

// extractJSONObjectField is a tiny non-strict JSON value extractor that pulls
// the string value for a top-level (or shallowly nested) key. It avoids a full
// encoding/json round-trip for robustness against trailing commas / comments
// in package.json. Returns "" if not found.
func extractJSONObjectField(raw, key string) string {
	// Find `"key"` then the following `:` then the string value.
	needle := `"` + key + `"`
	idx := strings.Index(raw, needle)
	if idx < 0 {
		return ""
	}
	rest := raw[idx+len(needle):]
	// Skip whitespace and a colon.
	for i := 0; i < len(rest); i++ {
		if rest[i] == ':' {
			rest = rest[i+1:]
			break
		}
	}
	rest = strings.TrimLeftFunc(rest, unicode.IsSpace)
	if len(rest) == 0 || rest[0] != '"' {
		return ""
	}
	// Scan until the closing unescaped quote.
	var b strings.Builder
	for i := 1; i < len(rest); i++ {
		c := rest[i]
		if c == '\\' && i+1 < len(rest) {
			b.WriteByte(rest[i+1])
			i++
			continue
		}
		if c == '"' {
			break
		}
		b.WriteByte(c)
	}
	return b.String()
}

// tokenizeLines splits a file's contents into whitespace-delimited tokens,
// ignoring comments. Used by the Dockerfile EXPOSE scanner.
func tokenizeLines(raw string) []string {
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		// Strip comments.
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		for _, t := range strings.Fields(line) {
			out = append(out, t)
		}
	}
	return out
}

// atoiSafe parses an int, returning 0 on failure.
func atoiSafe(s string) int {
	// Docker EXPOSE may include "/tcp" suffix; strip it.
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// equalFold reports whether two ASCII strings are equal, ignoring case.
func equalFold(a, b string) bool {
	return strings.EqualFold(a, b)
}
