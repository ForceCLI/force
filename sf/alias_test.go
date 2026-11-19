package sf

import (
	"os"
	"path/filepath"
	"testing"
)

// The expected output in this file was captured from sf 2.138.6.

func TestAliasCommands(t *testing.T) {
	home := useTempHome(t)

	expectRun(t, 0, "No results\n", "alias", "list")
	expectRun(t, 0, `{
  "status": 0,
  "result": [],
  "warnings": []
}
`, "alias", "list", "--json")

	expectRun(t, 0, `Alias Set
┌───────┬───────────────┬─────────┐
│ Alias │ Value         │ Success │
├───────┼───────────────┼─────────┤
│ a     │ x@example.com │ true    │
│ b     │ y@example.com │ true    │
└───────┴───────────────┴─────────┘

`, "alias", "set", "a=x@example.com", "b=y@example.com")

	expectRun(t, 0, `{
  "status": 0,
  "result": [
    {
      "alias": "c",
      "success": true,
      "value": "z@example.com"
    }
  ],
  "warnings": []
}
`, "alias", "set", "c=z@example.com", "--json")

	expectRun(t, 0, `Alias List
┌───────┬───────────────┐
│ Alias │ Value         │
├───────┼───────────────┤
│ a     │ x@example.com │
│ b     │ y@example.com │
│ c     │ z@example.com │
└───────┴───────────────┘

`, "alias", "list")

	expectRun(t, 0, `Alias Unset
┌───────┬───────────────┬─────────┐
│ Alias │ Value         │ Success │
├───────┼───────────────┼─────────┤
│ a     │ x@example.com │ true    │
└───────┴───────────────┴─────────┘

`, "alias", "unset", "a")

	expectRun(t, 0, `{
  "status": 0,
  "result": [
    {
      "alias": "nosuch",
      "success": true
    },
    {
      "alias": "b",
      "value": "y@example.com",
      "success": true
    }
  ],
  "warnings": []
}
`, "alias", "unset", "nosuch", "b", "--json")

	expectRun(t, 0, `Alias Unset
┌───────┬───────────────┬─────────┐
│ Alias │ Value         │ Success │
├───────┼───────────────┼─────────┤
│ c     │ z@example.com │ true    │
└───────┴───────────────┴─────────┘

`, "alias", "unset", "--all", "--no-prompt")

	data, err := os.ReadFile(filepath.Join(home, ".sfdx", "alias.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"orgs":{}}` {
		t.Errorf("alias.json = %s", data)
	}
}

func TestAliasErrors(t *testing.T) {
	useTempHome(t)

	_, errOut, status := run("alias", "set", "bad")
	want := "Error (InvalidArgumentFormatError): Error in the following argument\nbad\nSet varargs with this format: key=value or key=\"value with spaces\"\n\n\n"
	if status != 1 || errOut != want {
		t.Errorf("alias set bad: status %d, stderr %q", status, errOut)
	}

	_, errOut, status = run("alias", "unset")
	want = "Error (NameRequiredError): You must provide an alias name when unsetting an alias.\n\n\n"
	if status != 1 || errOut != want {
		t.Errorf("alias unset: status %d, stderr %q", status, errOut)
	}

	_, errOut, status = run("alias", "set")
	want = "Error (ArgumentsRequiredError): You must provide one or more aliases to set. Use the --help flag to see examples.\n\n\n"
	if status != 1 || errOut != want {
		t.Errorf("alias set: status %d, stderr %q", status, errOut)
	}
}

func TestLegacyAliasCommandId(t *testing.T) {
	useTempHome(t)
	if _, errOut, status := run("force:alias:set", "dev=me@example.com"); status != 0 {
		t.Fatalf("status %d, stderr %s", status, errOut)
	}
	expectRun(t, 0, `{
  "status": 0,
  "result": [
    {
      "alias": "dev",
      "value": "me@example.com"
    }
  ],
  "warnings": []
}
`, "alias:list", "--json")
}
