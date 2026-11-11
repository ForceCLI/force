package bubbles

import (
	"strings"
	"testing"

	"github.com/ForceCLI/force/lib/apexlog"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const debugTestClassID = "01p5g00000ABCDE"

var debugTestLog = strings.Join([]string{
	"62.0 APEX_CODE,FINEST",
	"Execute Anonymous: Integer total = Calc.add(2, 3);",
	"Execute Anonymous: System.debug(total);",
	"12:00:00.1 (1)|CODE_UNIT_STARTED|[EXTERNAL]|execute_anonymous_apex",
	"12:00:00.1 (2)|VARIABLE_SCOPE_BEGIN|[1]|total|Integer|false|false",
	"12:00:00.1 (3)|STATEMENT_EXECUTE|[1]",
	"12:00:00.1 (4)|METHOD_ENTRY|[1]|01p5g00000ABCDE|Calc.add(Integer, Integer)",
	"12:00:00.1 (5)|VARIABLE_SCOPE_BEGIN|[2]|a|Integer|false|false",
	"12:00:00.1 (6)|VARIABLE_ASSIGNMENT|[2]|a|2",
	"12:00:00.1 (7)|VARIABLE_ASSIGNMENT|[2]|pair|{\"left\":2,\"right\":3}|0x10",
	"12:00:00.1 (8)|STATEMENT_EXECUTE|[3]",
	"12:00:00.1 (9)|STATEMENT_EXECUTE|[4]",
	"12:00:00.1 (10)|METHOD_EXIT|[1]|01p5g00000ABCDE|Calc.add(Integer, Integer)",
	"12:00:00.1 (11)|VARIABLE_ASSIGNMENT|[1]|total|5",
	"12:00:00.1 (12)|STATEMENT_EXECUTE|[2]",
	"12:00:00.1 (13)|USER_DEBUG|[2]|DEBUG|5",
	"12:00:00.1 (14)|CODE_UNIT_FINISHED|execute_anonymous_apex",
}, "\n")

func newDebugModel(t *testing.T) LogDebugModel {
	t.Helper()
	log, err := apexlog.Parse(strings.NewReader(debugTestLog))
	if err != nil {
		t.Fatal(err)
	}
	session, err := apexlog.NewSession(log)
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]DebugSource{
		apexlog.AnonymousKey: {Name: "anonymous Apex", Lines: log.Header.AnonymousSource},
		debugTestClassID: {Name: "Calc", Lines: []string{
			"public class Calc {",
			"\tpublic static Integer add(Integer a, Integer b) {",
			"\t\tInteger sum = a + b;",
			"\t\treturn sum;",
			"\t}",
			"}",
		}},
	}
	m := NewLogDebugModel(session, sources)
	return update(m, tea.WindowSizeMsg{Width: 120, Height: 40})
}

func update(m LogDebugModel, msg tea.Msg) LogDebugModel {
	next, _ := m.Update(msg)
	return next.(LogDebugModel)
}

func press(m LogDebugModel, keys ...string) LogDebugModel {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		m = update(m, msg)
	}
	return m
}

func TestLogDebugModelStartsAtFirstStatement(t *testing.T) {
	m := newDebugModel(t)
	if m.viewKey != apexlog.AnonymousKey || m.cursor != 1 {
		t.Errorf("viewKey=%q cursor=%d", m.viewKey, m.cursor)
	}
	view := m.View()
	for _, want := range []string{"anonymous Apex  line 1", "Integer total = Calc.add(2, 3);", "Call Stack", "anonymous Apex:1", "total = null"} {
		if !strings.Contains(view, want) {
			t.Errorf("view does not contain %q", want)
		}
	}
}

func TestLogDebugModelStepsIntoAndOver(t *testing.T) {
	m := newDebugModel(t)
	m = press(m, "s")
	if m.viewKey != debugTestClassID || m.cursor != 3 {
		t.Fatalf("after step in: viewKey=%q cursor=%d", m.viewKey, m.cursor)
	}
	if view := m.View(); !strings.Contains(view, "Calc.add(Integer, Integer):3") || !strings.Contains(view, "Integer sum = a + b;") {
		t.Errorf("view after step in:\n%s", view)
	}
	m = press(m, "n")
	if m.cursor != 4 {
		t.Errorf("after next: cursor=%d", m.cursor)
	}
	m = press(m, "o")
	if m.viewKey != apexlog.AnonymousKey || m.cursor != 2 {
		t.Errorf("after step out: viewKey=%q cursor=%d", m.viewKey, m.cursor)
	}
	if strings.Contains(m.View(), "DEBUG 5") {
		t.Error("output shown before the debug statement ran")
	}
	m = press(m, "s")
	view := m.View()
	if !strings.Contains(view, "[2] DEBUG 5") || !strings.Contains(view, "End of log") {
		t.Errorf("view at end of log:\n%s", view)
	}
}

func TestLogDebugModelContinuesToBreakpointSetFromClassPicker(t *testing.T) {
	m := newDebugModel(t)
	m = press(m, "f", "down", "enter")
	if m.viewKey != debugTestClassID || m.picker {
		t.Fatalf("picker did not open Calc: viewKey=%q picker=%v", m.viewKey, m.picker)
	}
	m = press(m, "down", "down", "down", "b")
	if !m.session.HasBreakpoint(apexlog.Breakpoint{Source: debugTestClassID, Line: 4}) {
		t.Fatalf("breakpoint not set; status=%q", m.status)
	}
	if !strings.Contains(m.View(), "●") {
		t.Error("breakpoint marker not shown")
	}
	m = press(m, "c")
	if m.cursor != 4 || !strings.HasPrefix(m.status, "Breakpoint at Calc line 4") {
		t.Errorf("cursor=%d status=%q", m.cursor, m.status)
	}
	m = press(m, "c")
	if !strings.HasPrefix(m.status, "End of log") {
		t.Errorf("status at end = %q", m.status)
	}
	m = press(m, "r")
	if m.viewKey != apexlog.AnonymousKey || m.cursor != 1 {
		t.Errorf("after restart: viewKey=%q cursor=%d", m.viewKey, m.cursor)
	}
}

func TestLogDebugModelSelectsCallerFrame(t *testing.T) {
	m := newDebugModel(t)
	m = press(m, "s", "tab", "down")
	if m.focus != stackPane || m.frame != 1 {
		t.Fatalf("focus=%v frame=%d", m.focus, m.frame)
	}
	if m.viewKey != apexlog.AnonymousKey || m.cursor != 1 {
		t.Errorf("caller source: viewKey=%q cursor=%d", m.viewKey, m.cursor)
	}
	if view := m.View(); !strings.Contains(view, "total = null") {
		t.Errorf("caller variables not shown:\n%s", view)
	}
}

func TestLogDebugModelExpandsObjectVariables(t *testing.T) {
	m := newDebugModel(t)
	m = press(m, "s", "tab", "tab")
	rows := m.variableRows()
	pairRow := -1
	for i, r := range rows {
		if strings.HasPrefix(r.label, "pair = ") {
			pairRow = i
		}
	}
	if pairRow < 0 {
		t.Fatalf("no pair row in %+v", rows)
	}
	for m.varCursor < pairRow {
		m = press(m, "down")
	}
	m = press(m, "enter")
	view := m.View()
	if !strings.Contains(view, "left = 2") || !strings.Contains(view, "right = 3") {
		t.Errorf("fields not shown after expanding:\n%s", view)
	}
	m = press(m, "enter")
	if strings.Contains(m.View(), "left = 2") {
		t.Error("fields still shown after collapsing")
	}
}

func TestLogDebugModelReportsMissingSource(t *testing.T) {
	m := newDebugModel(t)
	m.sources[debugTestClassID] = DebugSource{Name: "Calc", Missing: "Source is hidden in a managed package"}
	m = press(m, "s")
	if !strings.Contains(m.View(), "Source is hidden in a managed package") {
		t.Errorf("missing-source message not shown:\n%s", m.View())
	}
	m = press(m, "b")
	if m.status != "No source to set a breakpoint in" {
		t.Errorf("status = %q", m.status)
	}
}

func TestLogDebugModelFitsWindow(t *testing.T) {
	m := newDebugModel(t)
	m = press(m, "s")
	for _, size := range []tea.WindowSizeMsg{{Width: 120, Height: 40}, {Width: 60, Height: 20}} {
		m = update(m, size)
		lines := strings.Split(m.View(), "\n")
		if len(lines) > size.Height {
			t.Errorf("%dx%d: %d lines", size.Width, size.Height, len(lines))
		}
		for i, line := range lines {
			if w := lipgloss.Width(line); w > size.Width {
				t.Errorf("%dx%d: line %d is %d columns", size.Width, size.Height, i, w)
			}
		}
	}
}

func TestLogDebugModelWarnsAboutBreakpointOnLineThatDidNotRun(t *testing.T) {
	m := newDebugModel(t)
	m = press(m, "f", "down", "enter", "down", "b")
	if m.cursor != 2 || !strings.Contains(m.status, "no statement on this line ran in the log") {
		t.Errorf("cursor=%d status=%q", m.cursor, m.status)
	}
	if !strings.Contains(m.View(), "○") {
		t.Error("breakpoint on a line that did not run is not shown as ○")
	}
	m = press(m, "b", "down", "b")
	if m.cursor != 3 || m.status != "Breakpoint set at Calc line 3" {
		t.Errorf("cursor=%d status=%q", m.cursor, m.status)
	}
}

func TestLogDebugModelShowsWhenACollectionWasAssigned(t *testing.T) {
	log, err := apexlog.Parse(strings.NewReader(strings.Join([]string{
		"62.0 APEX_CODE,FINEST",
		"Execute Anonymous: List<Integer> items = new List<Integer>();",
		"Execute Anonymous: items.add(1);",
		"12:00:00.1 (1)|CODE_UNIT_STARTED|[EXTERNAL]|execute_anonymous_apex",
		"12:00:00.1 (2)|STATEMENT_EXECUTE|[1]",
		"12:00:00.1 (3)|VARIABLE_SCOPE_BEGIN|[1]|items|List<Integer>|true|false",
		"12:00:00.1 (4)|VARIABLE_ASSIGNMENT|[1]|items|[]|0x10",
		"12:00:00.1 (5)|STATEMENT_EXECUTE|[2]",
		"12:00:00.1 (6)|CODE_UNIT_FINISHED|execute_anonymous_apex",
	}, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	session, err := apexlog.NewSession(log)
	if err != nil {
		t.Fatal(err)
	}
	m := NewLogDebugModel(session, map[string]DebugSource{
		apexlog.AnonymousKey: {Name: "anonymous Apex", Lines: log.Header.AnonymousSource},
	})
	m = update(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m = press(m, "n")
	if view := m.View(); !strings.Contains(view, "items = []  as of line 1") {
		t.Errorf("assignment line not shown:\n%s", view)
	}
}
