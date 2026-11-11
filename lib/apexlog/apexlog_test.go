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

func TestStepBackIsTheReverseOfStepOver(t *testing.T) {
	s := newSession(t)
	position := func() string { return fmt.Sprintf("%s:%d", s.Top().SourceKey(), s.Top().Line) }
	var forward []string
	for s.Resume(StepIn) != StoppedAtEnd {
		forward = append(forward, position())
	}
	want := []string{"anonymous:1", calcID + ":5", calcID + ":6", calcID + ":15", calcID + ":8", "anonymous:2"}
	if fmt.Sprint(forward) != fmt.Sprint(want) {
		t.Fatalf("forward = %v", forward)
	}

	// From the end, back to the last statement, then over the call to
	// Calc.add back to line 1.
	for _, w := range []string{"anonymous:2", "anonymous:1"} {
		if reason := s.StepBack(); reason != StoppedOnStep || position() != w {
			t.Errorf("reason=%v at %s, want %s", reason, position(), w)
		}
	}
	if reason := s.StepBack(); reason != StoppedAtStart || position() != "anonymous:1" {
		t.Errorf("at first statement: reason=%v at %s", reason, position())
	}

	// From line 8, back over the constructor's line 15 to line 6.
	s.SetBreakpoint(Breakpoint{Source: calcID, Line: 8}, true)
	s.Restart()
	s.Resume(Continue)
	s.StepBack()
	if position() != calcID+":6" || s.Statement() != 3 {
		t.Errorf("at %s (statement %d), want %s:6", position(), s.Statement(), calcID)
	}
	// From the first statement of a method, back to the caller.
	s.StepBack()
	s.StepBack()
	if position() != "anonymous:1" {
		t.Errorf("at %s, want anonymous:1", position())
	}
}

func TestStepBackRestoresVariablesAndOutput(t *testing.T) {
	s := newSession(t)
	for s.Resume(StepIn) != StoppedAtEnd {
	}
	output := len(s.Output())
	s.StepBack()
	s.StepBack()
	if len(s.Output()) >= output {
		t.Errorf("output after stepping back = %d, at end = %d", len(s.Output()), output)
	}
	s.SetBreakpoint(Breakpoint{Source: calcID, Line: 6}, true)
	s.Restart()
	s.Resume(Continue)
	s.Resume(StepIn)
	s.StepBack()
	if s.Top().Line != 6 || !s.HasBreakpoint(Breakpoint{Source: calcID, Line: 6}) {
		t.Errorf("line = %d", s.Top().Line)
	}
	if _, ok := s.Top().Locals.Get("a"); !ok {
		t.Error("locals not restored")
	}
}

func TestVisualforceControllerCallsAreFrames(t *testing.T) {
	s := newTestSession(t, strings.Join([]string{
		"62.0 APEX_CODE,FINEST",
		"12:00:00.1 (1)|CODE_UNIT_STARTED|[EXTERNAL]|066000000000001|VF: /apex/MyPage",
		"12:00:00.1 (2)|VF_APEX_CALL_START|[EXTERNAL]|MyController <init>|MyController",
		"12:00:00.1 (3)|CONSTRUCTOR_ENTRY|[1]|01p5g00000ABCDE|<init>()|MyController",
		"12:00:00.1 (4)|STATEMENT_EXECUTE|[3]",
		"12:00:00.1 (5)|CONSTRUCTOR_EXIT|[1]|01p5g00000ABCDE|<init>()|MyController",
		"12:00:00.1 (6)|VF_APEX_CALL_END|MyController <init>|MyController",
		"12:00:00.1 (7)|VF_APEX_CALL_START|[EXTERNAL]|MyController get(message)|MyController",
		"12:00:00.1 (8)|VF_APEX_CALL_START|[EXTERNAL]|MyController invoke(getmessage)|MyController",
		"12:00:00.1 (9)|STATEMENT_EXECUTE|[5]",
		"12:00:00.1 (10)|VF_APEX_CALL_END|MyController invoke(getmessage)|MyController",
		"12:00:00.1 (11)|VF_APEX_CALL_END|MyController get(message)|MyController",
		"12:00:00.1 (12)|VF_APEX_CALL_START|[EXTERNAL]|MyController invoke(save)|MyController",
		"12:00:00.1 (13)|STATEMENT_EXECUTE|[8]",
		"12:00:00.1 (14)|VF_APEX_CALL_END|MyController invoke(save)|MyController",
		"12:00:00.1 (15)|CODE_UNIT_FINISHED|VF: /apex/MyPage",
	}, "\n"))
	var stops []string
	for s.Resume(StepIn) != StoppedAtEnd {
		var names []string
		for _, f := range s.Frames() {
			names = append(names, f.Name)
		}
		top := s.Top()
		stops = append(stops, fmt.Sprintf("%s:%d %s", top.SourceKey(), top.Line, strings.Join(names, " < ")))
	}
	want := []string{
		"01p5g00000ABCDE:3 VF: /apex/MyPage < MyController <init> < new MyController()",
		"01p5g00000ABCDE:5 VF: /apex/MyPage < MyController.getmessage()",
		"01p5g00000ABCDE:8 VF: /apex/MyPage < MyController.save()",
	}
	if strings.Join(stops, "\n") != strings.Join(want, "\n") {
		t.Errorf("stops:\n%s\nwant:\n%s", strings.Join(stops, "\n"), strings.Join(want, "\n"))
	}
}

// testHeapDump has the structure of the HeapDump field of an
// ApexExecutionOverlayResult.
const testHeapDump = `{
  "className": "Calc",
  "namespace": "none",
  "heapDumpDate": "2026-11-11T21:57:43.685+0000",
  "extents": [
    {"typeName": "Account", "collectionType": null, "count": 2, "definition": [], "totalSize": 0, "extent": [
      {"address": "0x71ecde6d", "isStatic": false, "symbols": null, "size": 0, "value": {"entry": [
        {"keyDisplayValue": "Name", "value": {"value": "Acme 0"}},
        {"keyDisplayValue": "NumberOfEmployees", "value": {"value": 10}}
      ]}},
      {"address": "0x6c313b45", "isStatic": false, "symbols": null, "size": 0, "value": {"entry": [
        {"keyDisplayValue": "Name", "value": {"value": "Acme 1"}},
        {"keyDisplayValue": "NumberOfEmployees", "value": {"value": null}}
      ]}}
    ]},
    {"typeName": "List<Account>", "collectionType": "Account", "count": 1, "definition": [], "totalSize": 0, "extent": [
      {"address": "0x6c3d5e81", "isStatic": false, "symbols": ["accounts"], "size": 0, "value": {"value": [{"value": "0x71ecde6d"}, {"value": "0x6c313b45"}]}}
    ]},
    {"typeName": "List<String>", "collectionType": "String", "count": 1, "definition": [], "totalSize": 0, "extent": [
      {"address": "0x11111111", "isStatic": false, "symbols": ["names"], "size": 0, "value": {"value": [{"value": "0x22222222"}]}}
    ]},
    {"typeName": "Integer", "collectionType": null, "count": 1, "definition": [{"name": "value", "type": "Double"}], "totalSize": 0, "extent": [
      {"address": "0x7428ab", "isStatic": false, "symbols": ["count", "Calc.LIMIT"], "size": 0, "value": {"value": 2}}
    ]},
    {"typeName": "String", "collectionType": null, "count": 2, "definition": [{"name": "stringValue", "type": "char[]"}], "totalSize": 0, "extent": [
      {"address": "0x22222222", "isStatic": false, "symbols": null, "size": 0, "value": {"value": "x|y"}},
      {"address": "0x33333333", "isStatic": true, "symbols": ["Calc.label"], "size": 0, "value": {"value": "sum"}}
    ]}
  ]
}`

func heapDumpSession(t *testing.T) *Session {
	t.Helper()
	s := newTestSession(t, strings.Join([]string{
		"62.0 APEX_CODE,FINEST",
		"12:00:00.1 (1)|CODE_UNIT_STARTED|[EXTERNAL]|01p5g00000ABCDE|Calc.run()",
		"12:00:00.1 (2)|VARIABLE_SCOPE_BEGIN|[3]|accounts|List<Account>|true|false",
		"12:00:00.1 (3)|VARIABLE_ASSIGNMENT|[3]|accounts|[]|0x6c3d5e81",
		"12:00:00.1 (4)|STATEMENT_EXECUTE|[3]",
		"12:00:00.1 (5)|VARIABLE_ASSIGNMENT|[4]|this.Name|\"Acme 0\"|0x71ecde6d",
		"12:00:00.1 (6)|HEAP_DUMP|[9]|07n000000000001|Calc|none|9",
		"12:00:00.1 (7)|STATEMENT_EXECUTE|[9]",
		"12:00:00.1 (8)|STATEMENT_EXECUTE|[10]",
		"12:00:00.1 (9)|CODE_UNIT_FINISHED|Calc.run()",
	}, "\n"))
	if ids := HeapDumpIDs(s.Log()); fmt.Sprint(ids) != "[07n000000000001]" {
		t.Fatalf("ids = %v", ids)
	}
	dump, err := ParseHeapDump([]byte(testHeapDump))
	if err != nil {
		t.Fatal(err)
	}
	s.AddHeapDump("07n000000000001", dump)
	return s
}

func TestHeapDumpReplacesVariablesAtItsStatement(t *testing.T) {
	s := heapDumpSession(t)
	s.Resume(StepIn)
	accounts, _ := s.Top().Locals.Get("accounts")
	if s.AtHeapDump() || len(s.Children(accounts)) != 0 {
		t.Fatalf("before the dump: at=%v children=%d", s.AtHeapDump(), len(s.Children(accounts)))
	}

	s.Resume(StepIn)
	if !s.AtHeapDump() || s.Top().Line != 9 {
		t.Fatalf("at=%v line=%d", s.AtHeapDump(), s.Top().Line)
	}
	locals := s.Top().Locals
	accounts, _ = locals.Get("accounts")
	if accounts.Ref != "0x6c3d5e81" || accounts.Line != 9 || s.ObjectType(accounts.Ref) != "List<Account>" {
		t.Errorf("accounts = %+v", accounts)
	}
	elements := s.Children(accounts)
	if len(elements) != 2 || elements[0].Name != "[0]" || elements[0].Ref != "0x71ecde6d" {
		t.Fatalf("elements = %+v", elements)
	}
	fields := s.Children(elements[1])
	if len(fields) != 2 || fields[0].Name != "Name" || fields[0].Value != "'Acme 1'" || fields[1].Value != "null" {
		t.Errorf("fields = %+v", fields)
	}
	names, _ := locals.Get("names")
	if c := s.Children(names); len(c) != 1 || c[0].Value != "'x|y'" || c[0].Ref != "" {
		t.Errorf("names = %+v", c)
	}
	if count, _ := locals.Get("count"); count.Value != "2" || count.Type != "Integer" {
		t.Errorf("count = %+v", count)
	}
	statics := s.Statics(s.Top())
	if v, _ := statics.Get("label"); v == nil || v.Value != "'sum'" {
		t.Errorf("label = %+v", v)
	}
	if v, _ := statics.Get("LIMIT"); v == nil || v.Value != "2" {
		t.Errorf("LIMIT = %+v", v)
	}

	s.Resume(StepIn)
	if s.AtHeapDump() {
		t.Error("AtHeapDump after the next statement")
	}
	if accounts, _ := s.Top().Locals.Get("accounts"); len(s.Children(accounts)) != 2 {
		t.Error("dump contents not kept after the dump's statement")
	}
}

func TestHeapDumpIsIgnoredWithoutItsContents(t *testing.T) {
	s := newTestSession(t, strings.Join([]string{
		"62.0 APEX_CODE,FINEST",
		"12:00:00.1 (1)|CODE_UNIT_STARTED|[EXTERNAL]|01p5g00000ABCDE|Calc.run()",
		"12:00:00.1 (2)|HEAP_DUMP|[9]|07n000000000001|Calc|none|9",
		"12:00:00.1 (3)|STATEMENT_EXECUTE|[9]",
		"12:00:00.1 (4)|CODE_UNIT_FINISHED|Calc.run()",
	}, "\n"))
	s.Resume(StepIn)
	if s.AtHeapDump() || s.Top().Locals.Len() != 0 {
		t.Errorf("at=%v locals=%d", s.AtHeapDump(), s.Top().Locals.Len())
	}
}
