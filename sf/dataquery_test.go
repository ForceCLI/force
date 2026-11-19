package sf

import (
	"os"
	"strings"
	"testing"
)

// The layouts in these tests were compared with the output of sf 2.138.6
// for the same queries against a local server.

func column(name string) string {
	return `{"aggregate":false,"columnName":"` + name + `","displayName":"` + name + `","joinColumns":[]}`
}

func TestDataQueryLaysOutSubqueries(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.columnMetadata = map[string]string{"FROM Account": `{"columnMetadata":[` + column("Name") + `,` + column("Phone") + `,
		{"aggregate":true,"columnName":"Contacts","displayName":"Contacts","joinColumns":[` + column("LastName") + `,` + column("Email") + `]},
		{"aggregate":true,"columnName":"Opportunities","displayName":"Opportunities","joinColumns":[` + column("Name") + `]}]}`}
	// The records list their fields in another order than the query, as
	// some servers return them.  The columns follow the column metadata.
	org.rawQueries["FROM Account"] = `{"done":true,"totalSize":3,"records":[
		{"attributes":{"type":"Account"},"Contacts":{"done":true,"totalSize":2,"records":[
			{"attributes":{"type":"Contact"},"Email":null,"LastName":"Able"},
			{"attributes":{"type":"Contact"},"Email":"b@example.com","LastName":"Baker"}]},
		 "Name":"Sub One","Opportunities":{"done":true,"totalSize":1,"records":[{"attributes":{"type":"Opportunity"},"Name":"Opp1"}]},"Phone":"111"},
		{"attributes":{"type":"Account"},"Contacts":null,"Name":"Sub Three","Opportunities":null,"Phone":null},
		{"attributes":{"type":"Account"},"Contacts":{"done":true,"totalSize":1,"records":[{"attributes":{"type":"Contact"},"Email":null,"LastName":"Cole"}]},"Name":"Sub, Two","Opportunities":null,"Phone":null}]}`
	query := "SELECT Name, Phone, (SELECT LastName, Email FROM Contacts), (SELECT Name FROM Opportunities) FROM Account"
	out, _, status := run("data", "query", "-o", "dev", "-q", query)
	want := `┌───────────┬───────┬───────────────────┬────────────────┬────────────────────┐
│ NAME      │ PHONE │ CONTACTS.LASTNAME │ CONTACTS.EMAIL │ OPPORTUNITIES.NAME │
├───────────┼───────┼───────────────────┼────────────────┼────────────────────┤
│ Sub One   │ 111   │ Able              │ null           │ Opp1               │
│           │       │ Baker             │ b@example.com  │                    │
│ Sub Three │ null  │                   │                │                    │
│ Sub, Two  │ null  │ Cole              │ null           │                    │
└───────────┴───────┴───────────────────┴────────────────┴────────────────────┘

Total number of records retrieved: 3.
`
	if status != 0 || out != want {
		t.Errorf("status %d, stdout\n%s\nwant\n%s", status, out, want)
	}
	// sf writes the columns of a subquery once for each record of the
	// largest result, each time after a totalSize column.
	out, _, _ = run("data", "query", "-o", "dev", "-q", query, "-r", "csv")
	want = `Name,Phone,Contacts.totalSize,Contacts.records.0.LastName,Contacts.records.0.Email,Contacts.totalSize,Contacts.records.1.LastName,Contacts.records.1.Email,Opportunities.totalSize,Opportunities.records.0.Name
Sub One,111,2,Able,,2,Baker,b@example.com,1,Opp1
Sub Three,,,,,,,,,
"Sub, Two",,1,Cole,,1,,,,
`
	if out != want {
		t.Errorf("csv =\n%s\nwant\n%s", out, want)
	}
	if !strings.Contains(strings.Join(org.requests, "\n"), "&columns=true") {
		t.Errorf("no request for the columns of the query: %v", org.requests)
	}
}

func TestDataQueryNamesAggregatesAndParentFields(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.columnMetadata = map[string]string{"FROM Contact": `{"columnMetadata":[
		{"aggregate":false,"columnName":"Account","displayName":"Account","joinColumns":[` + column("Name") + `,
			{"aggregate":false,"columnName":"Owner","displayName":"Owner","joinColumns":[` + column("Alias") + `]}]},
		{"aggregate":true,"columnName":"total","displayName":"COUNT(Id)","joinColumns":[]},
		{"aggregate":true,"columnName":"expr0","displayName":"MAX(LastName)","joinColumns":[]}]}`}
	org.rawQueries["FROM Contact"] = `{"done":true,"totalSize":1,"records":[
		{"attributes":{"type":"AggregateResult"},"Account":{"attributes":{"type":"Account"},"Name":"Acme","Owner":{"attributes":{"type":"User"},"Alias":"ada"}},"total":2,"expr1":"Cole"}]}`
	query := "SELECT Account.Name, Account.Owner.Alias, COUNT(Id) total, MAX(LastName) FROM Contact GROUP BY Account.Name, Account.Owner.Alias"
	out, _, status := run("data", "query", "-o", "dev", "-q", query)
	// The second aggregate is expr1 in the record, and sf names an exprN
	// value after the aggregate at position N of the query.
	want := `┌──────────────┬─────────────────────┬───────┬───────────────┐
│ ACCOUNT.NAME │ ACCOUNT.OWNER.ALIAS │ TOTAL │ MAX(LASTNAME) │
├──────────────┼─────────────────────┼───────┼───────────────┤
│ Acme         │ ada                 │ 2     │ Cole          │
└──────────────┴─────────────────────┴───────┴───────────────┘

Total number of records retrieved: 1.
`
	if status != 0 || out != want {
		t.Errorf("status %d, stdout\n%s\nwant\n%s", status, out, want)
	}
	out, _, _ = run("data", "query", "-o", "dev", "-q", query, "-r", "csv")
	// Without subquery records the code of sf names a CSV column by the
	// display name of the column metadata and reads the value under that
	// name, so the value of an aggregate with another alias is empty.
	if want := "Account.Name,Account.Owner.Alias,COUNT(Id),MAX(LastName)\nAcme,ada,,Cole\n"; out != want {
		t.Errorf("csv = %q, want %q", out, want)
	}
}

func TestDataQueryCSVWithoutRecords(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.rawQueries["FROM Account"] = `{"done":true,"totalSize":0,"records":[]}`
	expectRun(t, 0, "\n", "data", "query", "-o", "dev", "-q", "SELECT Name FROM Account", "-r", "csv")
	for _, request := range org.requests {
		if strings.Contains(request, "columns=true") {
			t.Errorf("columns were requested for a query without records: %s", request)
		}
	}
	// A count has no records and one column.
	org.rawQueries["FROM Account"] = `{"done":true,"totalSize":3,"records":[]}`
	org.columnMetadata = map[string]string{"FROM Account": `{"columnMetadata":[{"aggregate":true,"columnName":"count()","displayName":"COUNT()","joinColumns":[]}]}`}
	if _, _, status := run("data", "query", "-o", "dev", "-q", "SELECT COUNT() FROM Account", "-r", "csv", "--output-file", "count.csv"); status != 0 {
		t.Fatalf("status %d", status)
	}
	if content, _ := os.ReadFile("count.csv"); string(content) != "COUNT()\n" {
		t.Errorf("count.csv = %q", content)
	}
}
