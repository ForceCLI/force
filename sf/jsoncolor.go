package sf

import (
	"io"
	"os"
	"regexp"
	"strings"
)

// The colors of JSON on a terminal, from the theme of sf: bright white
// braces and brackets, green commas, bright blue keys, bright green
// strings, blue numbers, bright red booleans and bright black nulls.
var jsonTokens = []struct {
	pattern *regexp.Regexp
	style   *style
}{
	{regexp.MustCompile(`^\s+`), nil},
	{regexp.MustCompile(`^[{}\[\]]`), &style{"\x1b[97m", "\x1b[39m"}},
	{regexp.MustCompile(`^:`), nil},
	{regexp.MustCompile(`^,`), &style{"\x1b[32m", "\x1b[39m"}},
	{regexp.MustCompile(`(?i)^-?\d+(?:\.\d+)?(?:e[+-]?\d+)?`), &style{"\x1b[34m", "\x1b[39m"}},
	{regexp.MustCompile(`^"(?:\\.|[^"\\])*"\s*:`), &style{"\x1b[94m", "\x1b[39m"}},
	{regexp.MustCompile(`^"(?:\\.|[^"\\])*"`), &style{"\x1b[92m", "\x1b[39m"}},
	{regexp.MustCompile(`^true|^false`), &style{"\x1b[91m", "\x1b[39m"}},
	{regexp.MustCompile(`^null`), &style{"\x1b[90m", "\x1b[39m"}},
}

// colorizeJSON adds the colors sf gives to JSON it prints on a terminal.
func colorizeJSON(text string) string {
	var b strings.Builder
	for len(text) > 0 {
		matched := false
		for i, token := range jsonTokens {
			found := token.pattern.FindString(text)
			if found == "" {
				continue
			}
			// The pattern of a key includes the colon after it so that a
			// key can be told from a string; the colon is not colored.
			if i == 5 {
				found = found[:strings.LastIndex(found, `"`)+1]
			}
			if token.style == nil {
				b.WriteString(found)
			} else {
				b.WriteString(token.style.open + found + token.style.close)
			}
			text = text[len(found):]
			matched = true
			break
		}
		if !matched {
			b.WriteString(text)
			break
		}
	}
	return b.String()
}

// jsonColorEnabled reports whether JSON written to w is colored.  sf colors
// JSON on a terminal and when FORCE_COLOR is set; unlike its tables, it does
// not color JSON written to a pipe when COLORTERM=truecolor.
func jsonColorEnabled(w io.Writer) bool {
	if force, ok := os.LookupEnv("FORCE_COLOR"); ok && force != "" {
		return force != "0" && force != "false"
	}
	if os.Getenv("COLORTERM") == "truecolor" {
		previous := os.Getenv("COLORTERM")
		os.Unsetenv("COLORTERM")
		defer os.Setenv("COLORTERM", previous)
	}
	return colorEnabled(w)
}
