package apexlog

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

const calcID = "01p5g00000ABCDE"

func loadLog(t *testing.T, name string) *Log {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	log, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	return log
}

func newTestSession(t *testing.T, input string) *Session {
	t.Helper()
	log, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewSession(log)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func newSession(t *testing.T) *Session {
	t.Helper()
	s, err := NewSession(loadLog(t, "anonymous.log"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

type position struct {
	source string
	line   int
	depth  int
}

func current(s *Session) position {
	top := s.Top()
	if top == nil {
		return position{}
	}
	return position{source: top.SourceKey(), line: top.Line, depth: len(s.Frames())}
}

func TestParseReadsHeader(t *testing.T) {
	log := loadLog(t, "anonymous.log")
	if log.Header.APIVersion != "62.0" {
		t.Errorf("APIVersion = %q", log.Header.APIVersion)
	}
	if got := log.Header.Level("APEX_CODE"); got != "FINEST" {
		t.Errorf("APEX_CODE level = %q", got)
	}
	if got := log.Header.Level("VISUALFORCE"); got != "FINER" {
		t.Errorf("VISUALFORCE level = %q", got)
	}
	want := []string{"Integer total = Calc.add(2, 3);", "System.debug('total: ' + total);"}
	if strings.Join(log.Header.AnonymousSource, "\n") != strings.Join(want, "\n") {
		t.Errorf("AnonymousSource = %q", log.Header.AnonymousSource)
	}
}

func TestParseReadsEvents(t *testing.T) {
	log := loadLog(t, "anonymous.log")
	first := log.Events[0]
	if first.Type != "USER_INFO" || first.LogLine != 4 || first.Nanos != 1000 {
		t.Errorf("first event = %+v", first)
	}
	var entry Event
	for _, e := range log.Events {
		if e.Type == "METHOD_ENTRY" {
			entry = e
			break
		}
	}
	if entry.SourceLine != 1 {
		t.Errorf("METHOD_ENTRY SourceLine = %d", entry.SourceLine)
	}
	if lastField(entry) != "Calc.add(Integer, Integer)" {
		t.Errorf("METHOD_ENTRY signature = %q", lastField(entry))
	}
}

func TestParseAppendsContinuationLinesToPreviousEvent(t *testing.T) {
	log := loadLog(t, "anonymous.log")
	for _, e := range log.Events {
		if e.Type == "USER_DEBUG" {
			message := strings.Join(e.Fields[2:], "|")
			if message != "total: 5\nsecond line|with pipe" {
				t.Errorf("message = %q", message)
			}
			return
		}
	}
	t.Fatal("no USER_DEBUG event")
}

func TestParseHandlesCRLFAndMissingTrailingNewline(t *testing.T) {
	input := "62.0 APEX_CODE,FINEST\r\n12:00:00.1 (1)|EXECUTION_STARTED\r\n12:00:00.1 (2)|USER_DEBUG|[1]|DEBUG|hi"
	log, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(log.Events) != 2 {
		t.Fatalf("events = %d", len(log.Events))
	}
	if got := lastField(log.Events[1]); got != "hi" {
		t.Errorf("last field = %q", got)
	}
}

func TestNewSessionRejectsLogWithoutFinestApexCode(t *testing.T) {
	log, err := Parse(strings.NewReader("62.0 APEX_CODE,FINE;DB,INFO\n12:00:00.1 (1)|EXECUTION_STARTED\n"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewSession(log)
	levelErr, ok := err.(ErrLogLevel)
	if !ok {
		t.Fatalf("err = %v", err)
	}
	if levelErr.Level != "FINE" {
		t.Errorf("Level = %q", levelErr.Level)
	}
}

func TestStepInStopsAtEveryStatement(t *testing.T) {
	s := newSession(t)
	want := []position{
		{AnonymousKey, 1, 1},
		{calcID, 5, 2},
		{calcID, 6, 2},
		{calcID, 15, 3},
		{calcID, 8, 2},
		{AnonymousKey, 2, 1},
	}
	for i, w := range want {
		if reason := s.Resume(StepIn); reason != StoppedOnStep {
			t.Fatalf("step %d: reason = %v", i, reason)
		}
		if got := current(s); got != w {
			t.Fatalf("step %d: position = %+v, want %+v", i, got, w)
		}
	}
	if reason := s.Resume(StepIn); reason != StoppedAtEnd {
		t.Errorf("final reason = %v", reason)
	}
	if !s.Done() {
		t.Error("Done = false at end of log")
	}
}

func TestStepOverSkipsCalledMethods(t *testing.T) {
	s := newSession(t)
	s.Resume(StepIn)
	s.Resume(StepOver)
	if got := current(s); got != (position{AnonymousKey, 2, 1}) {
		t.Errorf("position = %+v", got)
	}
}

func TestStepOverSkipsConstructors(t *testing.T) {
	s := newSession(t)
	for range 3 {
		s.Resume(StepIn)
	}
	if got := current(s); got != (position{calcID, 6, 2}) {
		t.Fatalf("setup position = %+v", got)
	}
	s.Resume(StepOver)
	if got := current(s); got != (position{calcID, 8, 2}) {
		t.Errorf("position = %+v", got)
	}
}

func TestStepOutStopsInCaller(t *testing.T) {
	s := newSession(t)
	for range 4 {
		s.Resume(StepIn)
	}
	s.Resume(StepOut)
	if got := current(s); got != (position{calcID, 8, 2}) {
		t.Errorf("position = %+v", got)
	}
	s.Resume(StepOut)
	if got := current(s); got != (position{AnonymousKey, 2, 1}) {
		t.Errorf("position = %+v", got)
	}
}

func TestContinueStopsAtBreakpoint(t *testing.T) {
	s := newSession(t)
	s.SetBreakpoint(Breakpoint{Source: calcID, Line: 8}, true)
	if reason := s.Resume(Continue); reason != StoppedOnBreakpoint {
		t.Fatalf("reason = %v", reason)
	}
	if got := current(s); got != (position{calcID, 8, 2}) {
		t.Errorf("position = %+v", got)
	}
	if reason := s.Resume(Continue); reason != StoppedAtEnd {
		t.Errorf("second reason = %v", reason)
	}
}

func TestStepOverStopsAtBreakpointInCalledMethod(t *testing.T) {
	s := newSession(t)
	s.Resume(StepIn)
	s.SetBreakpoint(Breakpoint{Source: calcID, Line: 6}, true)
	if reason := s.Resume(StepOver); reason != StoppedOnBreakpoint {
		t.Fatalf("reason = %v", reason)
	}
	if got := current(s); got != (position{calcID, 6, 2}) {
		t.Errorf("position = %+v", got)
	}
}

func TestRemovedBreakpointIsNotHit(t *testing.T) {
	s := newSession(t)
	b := Breakpoint{Source: calcID, Line: 8}
	s.SetBreakpoint(b, true)
	s.SetBreakpoint(b, false)
	if reason := s.Resume(Continue); reason != StoppedAtEnd {
		t.Errorf("reason = %v", reason)
	}
}

func TestRestartReturnsToStartAndKeepsBreakpoints(t *testing.T) {
	s := newSession(t)
	s.SetBreakpoint(Breakpoint{Source: calcID, Line: 8}, true)
	s.Resume(Continue)
	s.Resume(Continue)
	s.Restart()
	if len(s.Frames()) != 0 || len(s.Output()) != 0 {
		t.Fatalf("state after restart: frames=%d output=%d", len(s.Frames()), len(s.Output()))
	}
	if reason := s.Resume(Continue); reason != StoppedOnBreakpoint {
		t.Errorf("reason = %v", reason)
	}
}

func variableValues(vars []*Variable) map[string]string {
	result := map[string]string{}
	for _, v := range vars {
		result[v.Name] = v.Value
	}
	return result
}

func TestLocalsAreTrackedPerFrame(t *testing.T) {
	s := newSession(t)
	s.SetBreakpoint(Breakpoint{Source: calcID, Line: 8}, true)
	s.Resume(Continue)
	locals := variableValues(s.Top().Locals.Variables())
	if locals["a"] != "2" || locals["b"] != "3" {
		t.Errorf("locals = %v", locals)
	}
	caller := s.Frames()[0]
	if got := variableValues(caller.Locals.Variables())["total"]; got != "null" {
		t.Errorf("caller total = %q", got)
	}
}

func TestStaticsAreTrackedPerClass(t *testing.T) {
	s := newSession(t)
	s.Resume(StepIn)
	s.Resume(StepIn)
	statics := variableValues(s.Statics(s.Top()).Variables())
	if statics["label"] != "'sum'" {
		t.Errorf("statics = %v", statics)
	}
}

func TestObjectFieldsComeFromReferences(t *testing.T) {
	s := newSession(t)
	s.SetBreakpoint(Breakpoint{Source: calcID, Line: 8}, true)
	s.Resume(Continue)
	result, ok := s.Top().Locals.Get("result")
	if !ok {
		t.Fatal("no result variable")
	}
	if result.Ref != "0x1a2b3c" {
		t.Errorf("Ref = %q", result.Ref)
	}
	if got := variableValues(s.Children(result)); got["value"] != "5" || len(got) != 1 {
		t.Errorf("children = %v", got)
	}
}

func TestCollectionValuesKeepPipesAndOrder(t *testing.T) {
	s := newSession(t)
	s.SetBreakpoint(Breakpoint{Source: calcID, Line: 8}, true)
	s.Resume(Continue)
	names, _ := s.Top().Locals.Get("names")
	if names.Value != `["x|y","z"]` || names.Ref != "0x4d5e6f" {
		t.Errorf("names = %+v", names)
	}
	children := s.Children(names)
	if len(children) != 2 || children[0].Name != "[0]" || children[0].Value != "'x|y'" || children[1].Value != "'z'" {
		t.Errorf("children = %v", variableValues(children))
	}
}

func TestConstructorFrameRecordsThis(t *testing.T) {
	s := newSession(t)
	for range 4 {
		s.Resume(StepIn)
	}
	top := s.Top()
	if top.Name != "new Calc.Result(Integer)" || top.Kind != ConstructorFrame {
		t.Errorf("frame = %+v", top)
	}
	if top.This != "0x1a2b3c" {
		t.Errorf("This = %q", top.This)
	}
	if _, ok := top.Locals.Get("this"); ok {
		t.Error("this is listed as a local")
	}
}

func TestReferenceStringsInJSONLinkToObjects(t *testing.T) {
	input := strings.Join([]string{
		"62.0 APEX_CODE,FINEST",
		"12:00:00.1 (1)|CODE_UNIT_STARTED|[EXTERNAL]|01p5g00000ABCDE|CalcTest.testAdd()",
		"12:00:00.1 (2)|VARIABLE_ASSIGNMENT|[3]|myB|{}|0x6ed02744",
		"12:00:00.1 (3)|VARIABLE_ASSIGNMENT|[3]|this.count|7|0x6ed02744",
		"12:00:00.1 (4)|VARIABLE_ASSIGNMENT|[4]|myA|{\"myB\":\"0x6ed02744\",\"label\":\"0x12 is text\"}|0x2f405ee0",
		"12:00:00.1 (5)|STATEMENT_EXECUTE|[5]",
	}, "\n")
	log, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewSession(log)
	if err != nil {
		t.Fatal(err)
	}
	s.Resume(StepIn)
	myA, _ := s.Top().Locals.Get("myA")
	fields := s.Children(myA)
	if len(fields) != 2 || fields[0].Name != "myB" || fields[0].Ref != "0x6ed02744" {
		t.Fatalf("fields = %+v", fields)
	}
	if got := variableValues(s.Children(fields[0])); got["count"] != "7" {
		t.Errorf("myA.myB fields = %v", got)
	}
	if fields[1].Value != "'0x12 is text'" || fields[1].Ref != "" {
		t.Errorf("label = %+v", fields[1])
	}
}

func TestOutputIncludesOnlyEventsReplayedSoFar(t *testing.T) {
	s := newSession(t)
	for range 6 {
		s.Resume(StepIn)
	}
	if len(s.Output()) != 0 {
		t.Fatalf("output before debug statement = %v", s.Output())
	}
	s.Resume(StepIn)
	out := s.Output()
	if len(out) != 1 || out[0].Kind != "DEBUG" || out[0].Level != "DEBUG" || out[0].SourceLine != 2 {
		t.Fatalf("output = %+v", out)
	}
	if out[0].Message != "total: 5\nsecond line|with pipe" {
		t.Errorf("message = %q", out[0].Message)
	}
}

func TestSourcesListsAnonymousApexAndClasses(t *testing.T) {
	s := newSession(t)
	got := s.Sources()
	want := []SourceRef{{Key: AnonymousKey, Name: "anonymous Apex"}, {Key: calcID, Name: "Calc"}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("Sources = %+v", got)
	}
}

func TestExceptionsAreCollectedAsOutput(t *testing.T) {
	input := strings.Join([]string{
		"62.0 APEX_CODE,FINEST",
		"12:00:00.1 (1)|CODE_UNIT_STARTED|[EXTERNAL]|01q5g00000ZZZZZ|Account on Account trigger event BeforeInsert|__sfdc_trigger/AccountTrigger",
		"12:00:00.1 (2)|STATEMENT_EXECUTE|[3]",
		"12:00:00.1 (3)|EXCEPTION_THROWN|[3]|System.NullPointerException: Attempt to de-reference a null object",
		"12:00:00.1 (4)|FATAL_ERROR|System.NullPointerException: Attempt to de-reference a null object",
		"",
		"Trigger.AccountTrigger: line 3, column 1",
	}, "\n")
	log, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewSession(log)
	if err != nil {
		t.Fatal(err)
	}
	s.Resume(StepIn)
	if top := s.Top(); top.Name != "AccountTrigger" || top.SourceKey() != "01q5g00000ZZZZZ" {
		t.Errorf("trigger frame = %+v", top)
	}
	s.Resume(StepIn)
	out := s.Output()
	if len(out) != 2 {
		t.Fatalf("output = %+v", out)
	}
	if out[0].Kind != "EXCEPTION" || out[0].Message != "System.NullPointerException: Attempt to de-reference a null object" {
		t.Errorf("exception = %+v", out[0])
	}
	if out[1].Kind != "FATAL_ERROR" || !strings.HasSuffix(out[1].Message, "Trigger.AccountTrigger: line 3, column 1") {
		t.Errorf("fatal error = %+v", out[1])
	}
	if refs := s.Sources(); len(refs) != 1 || refs[0].Name != "AccountTrigger" {
		t.Errorf("Sources = %+v", refs)
	}
}

func TestImplicitInitializerExitMatchesClassName(t *testing.T) {
	input := strings.Join([]string{
		"62.0 APEX_CODE,FINEST",
		"12:00:00.1 (1)|CODE_UNIT_STARTED|[EXTERNAL]|01p5g00000ABCDE|CalcTest.testAdd()",
		"12:00:00.1 (2)|METHOD_ENTRY|[2]|01p5g00000ABCDE|CalcTest.CalcTest()",
		"12:00:00.1 (3)|STATEMENT_EXECUTE|[1]",
		"12:00:00.1 (4)|METHOD_EXIT|[2]|CalcTest",
		"12:00:00.1 (5)|STATEMENT_EXECUTE|[4]",
		"12:00:00.1 (6)|METHOD_ENTRY|[4]||System.UserInfo.getUserId()",
		"12:00:00.1 (7)|METHOD_EXIT|[4]||System.UserInfo.getUserId()",
		"12:00:00.1 (8)|STATEMENT_EXECUTE|[5]",
		"12:00:00.1 (9)|METHOD_EXIT|[5]|Unmatched",
		"12:00:00.1 (10)|STATEMENT_EXECUTE|[6]",
	}, "\n")
	log, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewSession(log)
	if err != nil {
		t.Fatal(err)
	}
	// Stepping does not stop in the initializer, so a breakpoint stops there.
	s.SetBreakpoint(Breakpoint{Source: "01p5g00000ABCDE", Line: 1}, true)
	wantDepths := []int{2, 1, 1, 1}
	for i, want := range wantDepths {
		s.Resume(StepIn)
		if got := len(s.Frames()); got != want {
			t.Errorf("step %d: depth = %d, want %d", i, got, want)
		}
	}
	if s.Top().Class != "CalcTest" {
		t.Errorf("Class = %q", s.Top().Class)
	}
}

func TestStatementsInSystemMethodsAreSkipped(t *testing.T) {
	s := newTestSession(t, strings.Join([]string{
		"62.0 APEX_CODE,FINEST",
		"12:00:00.1 (1)|CODE_UNIT_STARTED|[EXTERNAL]|01p5g00000ABCDE|CalcTest.testAdd()",
		"12:00:00.1 (2)|STATEMENT_EXECUTE|[105]",
		"12:00:00.1 (3)|STATEMENT_EXECUTE|[106]",
		"12:00:00.1 (4)|SYSTEM_METHOD_ENTRY|[12]|DMLOptions.DMLOptions()",
		"12:00:00.1 (5)|STATEMENT_EXECUTE|[12]",
		"12:00:00.1 (6)|SYSTEM_METHOD_EXIT|[12]|DMLOptions",
		"12:00:00.1 (7)|STATEMENT_EXECUTE|[107]",
		"12:00:00.1 (8)|CODE_UNIT_FINISHED|CalcTest.testAdd()",
	}, "\n"))
	s.SetBreakpoint(Breakpoint{Source: "01p5g00000ABCDE", Line: 12}, true)
	var lines []int
	for s.Resume(StepIn) != StoppedAtEnd {
		lines = append(lines, s.Top().Line)
	}
	if fmt.Sprint(lines) != "[105 106 107]" {
		t.Errorf("lines = %v", lines)
	}
	if s.Executed("01p5g00000ABCDE", 12) {
		t.Error("line 12 of the system method recorded as executed")
	}
}

func TestSteppingSkipsStaticInitialization(t *testing.T) {
	s := newTestSession(t, strings.Join([]string{
		"62.0 APEX_CODE,FINEST",
		"12:00:00.1 (1)|CODE_UNIT_STARTED|[EXTERNAL]|01p5g00000TESTX|CalcTest.testAdd()",
		"12:00:00.1 (2)|METHOD_ENTRY|[14]|01p5g00000TESTX|CalcTest.CalcTest()",
		"12:00:00.1 (3)|STATEMENT_EXECUTE|[14]",
		"12:00:00.1 (4)|METHOD_EXIT|[14]|CalcTest",
		"12:00:00.1 (5)|STATEMENT_EXECUTE|[309]",
		"12:00:00.1 (6)|METHOD_ENTRY|[23]|01p5g00000ABCDE|Calc.Calc()",
		"12:00:00.1 (7)|STATEMENT_EXECUTE|[23]",
		"12:00:00.1 (8)|STATEMENT_EXECUTE|[35]",
		"12:00:00.1 (9)|METHOD_EXIT|[23]|Calc",
		"12:00:00.1 (10)|METHOD_ENTRY|[309]|01p5g00000TESTX|CalcTest.make(Integer)",
		"12:00:00.1 (11)|STATEMENT_EXECUTE|[105]",
		"12:00:00.1 (12)|METHOD_EXIT|[309]|CalcTest.make(Integer)",
		"12:00:00.1 (13)|CODE_UNIT_FINISHED|CalcTest.testAdd()",
	}, "\n"))
	var stops []string
	for s.Resume(StepIn) != StoppedAtEnd {
		stops = append(stops, fmt.Sprintf("%s:%d", s.Top().Name, s.Top().Line))
	}
	if want := "[CalcTest.testAdd():309 CalcTest.make(Integer):105]"; fmt.Sprint(stops) != want {
		t.Errorf("stops = %v, want %s", stops, want)
	}

	s.Restart()
	s.SetBreakpoint(Breakpoint{Source: "01p5g00000ABCDE", Line: 35}, true)
	if reason := s.Resume(Continue); reason != StoppedOnBreakpoint {
		t.Fatalf("reason = %v", reason)
	}
	top := s.Top()
	if !top.Initializer || top.Name != "Calc static initialization" || top.Line != 35 {
		t.Errorf("top = %+v", top)
	}
	if !s.Executed("01p5g00000ABCDE", 35) || s.Executed("01p5g00000ABCDE", 28) {
		t.Error("executed lines of Calc")
	}
}

func TestCollectionsRecordTheLineOfTheirLastAssignment(t *testing.T) {
	s := newTestSession(t, strings.Join([]string{
		"62.0 APEX_CODE,FINEST",
		"12:00:00.1 (1)|CODE_UNIT_STARTED|[EXTERNAL]|01p5g00000ABCDE|CalcTest.testAdd()",
		"12:00:00.1 (2)|STATEMENT_EXECUTE|[109]",
		"12:00:00.1 (3)|VARIABLE_SCOPE_BEGIN|[109]|accounts|List<Account>|true|false",
		"12:00:00.1 (4)|VARIABLE_ASSIGNMENT|[109]|accounts|[]|0x3f28b980",
		"12:00:00.1 (5)|VARIABLE_SCOPE_BEGIN|[110]|count|Integer|false|false",
		"12:00:00.1 (6)|VARIABLE_ASSIGNMENT|[110]|count|50",
		"12:00:00.1 (7)|VARIABLE_ASSIGNMENT|[111]|big|\"List of size 50 too large to display\"|0x6e5d197",
		"12:00:00.1 (8)|STATEMENT_EXECUTE|[130]",
	}, "\n"))
	s.SetBreakpoint(Breakpoint{Source: "01p5g00000ABCDE", Line: 130}, true)
	s.Resume(Continue)
	locals := s.Top().Locals
	accounts, _ := locals.Get("accounts")
	count, _ := locals.Get("count")
	big, _ := locals.Get("big")
	if !accounts.Collection() || accounts.Line != 109 {
		t.Errorf("accounts = %+v", accounts)
	}
	if count.Collection() || count.Line != 110 {
		t.Errorf("count = %+v", count)
	}
	if !big.Collection() || big.Line != 111 {
		t.Errorf("big = %+v", big)
	}
}
