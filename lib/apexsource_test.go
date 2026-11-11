package lib

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestApexSourceByIDQueriesClassesAndTriggers(t *testing.T) {
	var mu sync.Mutex
	var queries []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		mu.Lock()
		queries = append(queries, q)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(q, "FROM ApexClass"):
			fmt.Fprint(w, `{"totalSize":2,"done":true,"records":[
				{"attributes":{"type":"ApexClass"},"Id":"01p5g00000ABCDEAAA","Name":"Calc","NamespacePrefix":null,"Body":"public class Calc {}"},
				{"attributes":{"type":"ApexClass"},"Id":"01p5g00000MANAGAAA","Name":"Util","NamespacePrefix":"pkg","Body":"(hidden)"}
			]}`)
		case strings.Contains(q, "FROM ApexTrigger"):
			fmt.Fprint(w, `{"totalSize":1,"done":true,"records":[
				{"attributes":{"type":"ApexTrigger"},"Id":"01q5g00000ZZZZZAAA","Name":"AccountTrigger","NamespacePrefix":null,"Body":"trigger AccountTrigger on Account (before insert) {}"}
			]}`)
		default:
			http.Error(w, "unexpected query", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	f := &Force{Credentials: &ForceSession{InstanceUrl: server.URL, AccessToken: "token"}}
	sources, err := f.ApexSourceByID([]string{"01p5g00000ABCDE", "01p5g00000ABCDEAAA", "01p5g00000MANAG", "01q5g00000ZZZZZ", "0015g00000XXXXX"})
	if err != nil {
		t.Fatal(err)
	}

	if len(queries) != 2 {
		t.Fatalf("queries = %q", queries)
	}
	if !strings.Contains(queries[0], "WHERE Id IN ('01p5g00000ABCDE','01p5g00000MANAG')") {
		t.Errorf("class query = %q", queries[0])
	}
	if !strings.Contains(queries[1], "WHERE Id IN ('01q5g00000ZZZZZ')") {
		t.Errorf("trigger query = %q", queries[1])
	}

	calc := sources["01p5g00000ABCDE"]
	if calc.Name != "Calc" || calc.Body != "public class Calc {}" || calc.Hidden || calc.Trigger {
		t.Errorf("Calc = %+v", calc)
	}
	util := sources["01p5g00000MANAG"]
	if !util.Hidden || util.Body != "" || util.NamespacePrefix != "pkg" {
		t.Errorf("Util = %+v", util)
	}
	trigger := sources["01q5g00000ZZZZZ"]
	if !trigger.Trigger || trigger.Name != "AccountTrigger" {
		t.Errorf("AccountTrigger = %+v", trigger)
	}
	if len(sources) != 3 {
		t.Errorf("sources = %d", len(sources))
	}
}

func TestApexSourceByIDMakesNoRequestWithoutIDs(t *testing.T) {
	f := &Force{Credentials: &ForceSession{InstanceUrl: "http://127.0.0.1:0"}}
	sources, err := f.ApexSourceByID([]string{"0015g00000XXXXX", "short"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 0 {
		t.Errorf("sources = %v", sources)
	}
}
