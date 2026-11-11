package command

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/ForceCLI/force/lib"
	"github.com/ForceCLI/force/lib/apexlog"
)

func TestDebugSourcesCombinesLogAndOrgSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if !strings.Contains(q, "FROM ApexClass") {
			fmt.Fprint(w, `{"totalSize":0,"done":true,"records":[]}`)
			return
		}
		fmt.Fprint(w, `{"totalSize":2,"done":true,"records":[
			{"attributes":{"type":"ApexClass"},"Id":"01p5g00000ABCDEAAA","Name":"Calc","NamespacePrefix":null,"Body":"public class Calc {\r\n}"},
			{"attributes":{"type":"ApexClass"},"Id":"01p5g00000MANAGAAA","Name":"Util","NamespacePrefix":"pkg","Body":"(hidden)"}
		]}`)
	}))
	t.Cleanup(server.Close)
	origForce := force
	t.Cleanup(func() { force = origForce })
	force = NewForce(&ForceSession{AccessToken: "token", InstanceUrl: server.URL})

	log, err := apexlog.Parse(strings.NewReader(strings.Join([]string{
		"62.0 APEX_CODE,FINEST",
		"Execute Anonymous: Calc.run();",
		"12:00:00.1 (1)|CODE_UNIT_STARTED|[EXTERNAL]|execute_anonymous_apex",
		"12:00:00.1 (2)|METHOD_ENTRY|[1]|01p5g00000ABCDE|Calc.run()",
		"12:00:00.1 (3)|METHOD_ENTRY|[1]|01p5g00000MANAG|pkg.Util.go()",
		"12:00:00.1 (4)|METHOD_ENTRY|[1]|01p5g00000GONEX|Gone.go()",
	}, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	session, err := apexlog.NewSession(log)
	if err != nil {
		t.Fatal(err)
	}

	sources, err := debugSources(session)
	if err != nil {
		t.Fatal(err)
	}

	anonymous := sources[apexlog.AnonymousKey]
	if len(anonymous.Lines) != 1 || anonymous.Lines[0] != "Calc.run();" {
		t.Errorf("anonymous = %+v", anonymous)
	}
	calc := sources["01p5g00000ABCDE"]
	if calc.Name != "Calc" || len(calc.Lines) != 2 || calc.Lines[0] != "public class Calc {" {
		t.Errorf("Calc = %+v", calc)
	}
	util := sources["01p5g00000MANAG"]
	if util.Name != "pkg.Util" || len(util.Lines) != 0 || !strings.Contains(util.Missing, "managed package") {
		t.Errorf("Util = %+v", util)
	}
	gone := sources["01p5g00000GONEX"]
	if gone.Name != "Gone" || !strings.Contains(gone.Missing, "not found") {
		t.Errorf("Gone = %+v", gone)
	}
}

func TestReplayLogRejectsAnEmptyLog(t *testing.T) {
	if err := replayLog("  \n", false); err == nil || err.Error() != "no debug log to replay" {
		t.Errorf("err = %v", err)
	}
}

func TestReplayLogRejectsALogWithoutFinestApexCode(t *testing.T) {
	err := replayLog("62.0 APEX_CODE,DEBUG\n12:00:00.1 (1)|CODE_UNIT_STARTED|[EXTERNAL]|execute_anonymous_apex\n", false)
	var levelErr apexlog.ErrLogLevel
	if !errors.As(err, &levelErr) || levelErr.Level != "DEBUG" {
		t.Errorf("err = %v", err)
	}
}

func TestApexDebugCannotBeUsedWithTest(t *testing.T) {
	t.Cleanup(func() {
		apexCmd.Flags().Set("debug", "false")
		apexCmd.Flags().Set("test", "false")
	})
	apexCmd.Flags().Set("debug", "true")
	if err := apexCmd.Args(apexCmd, []string{"code.apex"}); err != nil {
		t.Errorf("--debug alone: %v", err)
	}
	apexCmd.Flags().Set("test", "true")
	if err := apexCmd.Args(apexCmd, []string{"code.apex"}); err == nil {
		t.Error("expected an error for --debug with --test")
	}
}

func TestTestDebugCannotBeUsedWithIntegration(t *testing.T) {
	origDebug, origIntegration := debugTestFlag, integrationTestFlag
	t.Cleanup(func() { debugTestFlag, integrationTestFlag = origDebug, origIntegration })
	debugTestFlag, integrationTestFlag = true, false
	if err := testCmd.Args(testCmd, []string{"CalcTest"}); err != nil {
		t.Errorf("--debug alone: %v", err)
	}
	integrationTestFlag = true
	if err := testCmd.Args(testCmd, []string{"CalcTest"}); err == nil {
		t.Error("expected an error for --debug with --integration")
	}
}

func TestRunTestsForReplaySetsTraceFlagAndFetchesTheLog(t *testing.T) {
	var events []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		switch {
		case strings.Contains(q, "FROM DebugLevel"):
			fmt.Fprint(w, `{"totalSize":1,"done":true,"records":[{"Id":"7dl000000000RPL"}]}`)
		case strings.Contains(q, "FROM TraceFlag"):
			fmt.Fprint(w, `{"totalSize":1,"done":true,"records":[{"Id":"7tf000000000OLD","DebugLevelId":"7dl000000000USR","StartDate":null,"ExpirationDate":"2999-01-01T00:00:00.000+0000"}]}`)
		case r.Method == "PATCH":
			body, _ := io.ReadAll(r.Body)
			events = append(events, "patch "+string(body))
			w.WriteHeader(204)
		case strings.Contains(r.URL.Path, "/services/Soap/s/"):
			events = append(events, "runTests")
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns="http://soap.sforce.com/2006/08/apex">
	<soapenv:Body><runTestsResponse><result><apexLogId>07L000000000001</apexLogId><numFailures>0</numFailures><numTestsRun>1</numTestsRun>
	<successes><methodName>adds</methodName><name>CalcTest</name></successes></result></runTestsResponse></soapenv:Body>
</soapenv:Envelope>`)
		case strings.HasSuffix(r.URL.Path, "/ApexLog/07L000000000001/Body"):
			events = append(events, "log")
			fmt.Fprint(w, "62.0 APEX_CODE,FINEST")
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(server.Close)
	origForce := force
	t.Cleanup(func() { force = origForce })
	force = NewForce(&ForceSession{
		AccessToken:    "token",
		InstanceUrl:    server.URL,
		UserInfo:       &UserInfo{OrgId: "00D000000000001", UserId: "005000000000001"},
		SessionOptions: &SessionOptions{ApiVersion: "62.0"},
	})

	result, err := runTestsForReplay([]string{"CalcTest"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Log != "62.0 APEX_CODE,FINEST" || len(result.SMethodNames) != 1 {
		t.Errorf("result = %+v", result)
	}
	if len(events) != 4 ||
		!strings.Contains(events[0], `"DebugLevelId":"7dl000000000RPL"`) ||
		events[1] != "runTests" ||
		!strings.Contains(events[2], `"DebugLevelId":"7dl000000000USR"`) ||
		events[3] != "log" {
		t.Errorf("events = %q", events)
	}
}

func TestParseCheckpoints(t *testing.T) {
	checkpoints, err := parseCheckpoints([]string{"Calc:9", "CalcTest:120"})
	if err != nil {
		t.Fatal(err)
	}
	if len(checkpoints) != 2 || checkpoints[0] != (Checkpoint{Name: "Calc", Line: 9}) || checkpoints[1] != (Checkpoint{Name: "CalcTest", Line: 120}) {
		t.Errorf("checkpoints = %+v", checkpoints)
	}
	for _, bad := range [][]string{{"Calc"}, {"Calc:"}, {":9"}, {"Calc:0"}, {"Calc:x"}, {"A:1", "B:2", "C:3", "D:4", "E:5", "F:6"}} {
		if _, err := parseCheckpoints(bad); err == nil {
			t.Errorf("%v: expected an error", bad)
		}
	}
}

func TestCheckpointRequiresDebug(t *testing.T) {
	origDebug, origCheckpoints := debugTestFlag, checkpointFlag
	t.Cleanup(func() { debugTestFlag, checkpointFlag = origDebug, origCheckpoints })
	debugTestFlag, checkpointFlag = false, []string{"Calc:9"}
	if err := testCmd.Args(testCmd, []string{"CalcTest"}); err == nil {
		t.Error("expected an error for force test --checkpoint without --debug")
	}
	debugTestFlag = true
	if err := testCmd.Args(testCmd, []string{"CalcTest"}); err != nil {
		t.Errorf("force test --debug --checkpoint: %v", err)
	}
	checkpointFlag = []string{"Calc"}
	if err := testCmd.Args(testCmd, []string{"CalcTest"}); err == nil {
		t.Error("expected an error for a malformed checkpoint")
	}

	t.Cleanup(func() {
		apexCmd.Flags().Set("debug", "false")
		apexCmd.Flags().Lookup("checkpoint").Value.(interface{ Replace([]string) error }).Replace(nil)
	})
	apexCmd.Flags().Set("checkpoint", "Calc:9")
	if err := apexCmd.Args(apexCmd, nil); err == nil {
		t.Error("expected an error for force apex --checkpoint without --debug")
	}
	apexCmd.Flags().Set("debug", "true")
	if err := apexCmd.Args(apexCmd, nil); err != nil {
		t.Errorf("force apex --debug --checkpoint: %v", err)
	}
}
