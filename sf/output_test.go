package sf

import (
	"encoding/json"
	"testing"
)

func TestColorizeJSONUsesTheColorsOfSf(t *testing.T) {
	got := colorizeJSON("{\n  \"a\": \"b:\",\n  \"n\": [1.5, true, null]\n}\n")
	want := "\x1b[97m{\x1b[39m\n  \x1b[94m\"a\"\x1b[39m: \x1b[92m\"b:\"\x1b[39m\x1b[32m,\x1b[39m\n  \x1b[94m\"n\"\x1b[39m: \x1b[97m[\x1b[39m\x1b[34m1.5\x1b[39m\x1b[32m,\x1b[39m \x1b[91mtrue\x1b[39m\x1b[32m,\x1b[39m \x1b[90mnull\x1b[39m\x1b[97m]\x1b[39m\n\x1b[97m}\x1b[39m\n"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestParseOrderedJSONPrintsWholeDecimalsAsIntegers(t *testing.T) {
	parsed, err := parseOrderedJSON([]byte(`{"a": 68.0, "b": 1.50, "c": "68.0"}`))
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(parsed)
	if string(encoded) != `{"a":68,"b":1.50,"c":"68.0"}` {
		t.Errorf("encoded %s", encoded)
	}
}
