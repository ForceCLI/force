package bubbles

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ForceCLI/force/lib/apexlog"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// DebugSource is the source shown for a class, trigger, or anonymous Apex.
type DebugSource struct {
	Name  string
	Lines []string
	// Missing explains why Lines is empty, e.g. because a managed package
	// hides the source.
	Missing string
}

type debugPane int

const (
	sourcePane debugPane = iota
	stackPane
	variablesPane
	paneCount
)

const maxVariableDepth = 20

var (
	focusedBorder   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#FFDD57"))
	unfocusedBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#4E4E4E"))
	paneTitleStyle  = lipgloss.NewStyle().Bold(true)
	currentStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFDD57"))
	cursorStyle     = lipgloss.NewStyle().Reverse(true)
	breakpointStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E88388"))
	lineNumberStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6C6C6C"))
	// executedStyle marks the numbers of lines on which a statement ran in
	// the log; only those lines can stop at a breakpoint.
	executedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#C6C6C6"))
	variableStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#89D5C9"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#6C6C6C"))
)

// LogDebugModel is a terminal debugger that replays an Apex debug log.
type LogDebugModel struct {
	session *apexlog.Session
	sources map[string]DebugSource
	focus   debugPane
	// frame is the selected stack frame, counted from the innermost frame.
	frame int
	// viewKey is the source shown in the source pane.
	viewKey string
	// cursor is the 1-based source line under the cursor.
	cursor int
	// offset is the 0-based index of the first source line shown.
	offset       int
	varCursor    int
	varOffset    int
	expanded     map[string]bool
	picker       bool
	pickerCursor int
	status       string
	width        int
	height       int
}

// NewLogDebugModel returns a model stopped at the first statement in the
// log. sources is keyed by apexlog.AnonymousKey or a 15-character class or
// trigger ID.
func NewLogDebugModel(session *apexlog.Session, sources map[string]DebugSource) LogDebugModel {
	m := LogDebugModel{
		session:  session,
		sources:  sources,
		expanded: map[string]bool{},
	}
	m.afterStop(session.Resume(apexlog.StepIn))
	return m
}

func (m LogDebugModel) Init() tea.Cmd {
	return nil
}

func (m LogDebugModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.centerSource(m.cursor)
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m LogDebugModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if m.picker {
		m.handlePickerKey(key)
		return m, nil
	}
	switch key {
	case "q":
		return m, tea.Quit
	case "n":
		m.afterStop(m.session.Resume(apexlog.StepOver))
	case "s":
		m.afterStop(m.session.Resume(apexlog.StepIn))
	case "o":
		m.afterStop(m.session.Resume(apexlog.StepOut))
	case "c":
		m.afterStop(m.session.Resume(apexlog.Continue))
	case "r":
		m.session.Restart()
		m.afterStop(m.session.Resume(apexlog.StepIn))
		if m.status == "Paused" {
			m.status = "Restarted"
		}
	case "b":
		m.toggleBreakpoint()
	case "f":
		m.picker = true
		m.pickerCursor = 0
	case "tab":
		m.focus = (m.focus + 1) % paneCount
	case "shift+tab":
		m.focus = (m.focus + paneCount - 1) % paneCount
	default:
		switch m.focus {
		case sourcePane:
			m.handleSourceKey(key)
		case stackPane:
			m.handleStackKey(key)
		case variablesPane:
			m.handleVariablesKey(key)
		}
	}
	return m, nil
}

func (m *LogDebugModel) handlePickerKey(key string) {
	refs := m.session.Sources()
	switch key {
	case "esc", "f", "q":
		m.picker = false
	case "up", "k":
		if m.pickerCursor > 0 {
			m.pickerCursor--
		}
	case "down", "j":
		if m.pickerCursor < len(refs)-1 {
			m.pickerCursor++
		}
	case "enter":
		if m.pickerCursor < len(refs) {
			m.showSource(refs[m.pickerCursor].Key, 1)
			m.focus = sourcePane
		}
		m.picker = false
	}
}

func (m *LogDebugModel) handleSourceKey(key string) {
	lines := len(m.sources[m.viewKey].Lines)
	page := max(m.sourceViewHeight()-1, 1)
	switch key {
	case "up", "k":
		m.cursor--
	case "down", "j":
		m.cursor++
	case "pgup", "ctrl+u":
		m.cursor -= page
	case "pgdown", "ctrl+d":
		m.cursor += page
	case "g", "home":
		m.cursor = 1
	case "G", "end":
		m.cursor = lines
	}
	m.cursor = min(max(m.cursor, 1), max(lines, 1))
	m.scrollSourceToCursor()
}

func (m *LogDebugModel) handleStackKey(key string) {
	frames := m.session.Frames()
	switch key {
	case "up", "k":
		if m.frame > 0 {
			m.selectFrame(m.frame - 1)
		}
	case "down", "j":
		if m.frame < len(frames)-1 {
			m.selectFrame(m.frame + 1)
		}
	}
}

func (m *LogDebugModel) handleVariablesKey(key string) {
	rows := m.variableRows()
	switch key {
	case "up", "k":
		if m.varCursor > 0 {
			m.varCursor--
		}
	case "down", "j":
		if m.varCursor < len(rows)-1 {
			m.varCursor++
		}
	case "enter", " ", "right", "l":
		if m.varCursor < len(rows) && rows[m.varCursor].expandable {
			path := rows[m.varCursor].path
			if key == "right" || key == "l" {
				m.expanded[path] = true
			} else {
				m.expanded[path] = !m.expanded[path]
			}
		}
	case "left", "h":
		if m.varCursor < len(rows) {
			delete(m.expanded, rows[m.varCursor].path)
		}
	}
	m.scrollVariablesToCursor()
}

// afterStop selects the innermost frame and shows where execution stopped.
func (m *LogDebugModel) afterStop(reason apexlog.StopReason) {
	m.frame = 0
	m.varCursor = 0
	m.varOffset = 0
	top := m.session.Top()
	switch {
	case reason == apexlog.StoppedAtEnd || top == nil:
		m.status = "End of log. Press r to restart."
		return
	case reason == apexlog.StoppedOnBreakpoint:
		m.status = fmt.Sprintf("Breakpoint at %s line %d", m.sourceName(top.SourceKey(), top.Name), top.Line)
	default:
		m.status = "Paused"
	}
	m.showSource(top.SourceKey(), top.Line)
}

func (m *LogDebugModel) selectFrame(index int) {
	m.frame = index
	m.varCursor = 0
	m.varOffset = 0
	if f := m.selectedFrame(); f != nil {
		m.showSource(f.SourceKey(), f.Line)
	}
}

func (m LogDebugModel) selectedFrame() *apexlog.Frame {
	frames := m.session.Frames()
	i := len(frames) - 1 - m.frame
	if i < 0 || i >= len(frames) {
		return nil
	}
	return frames[i]
}

func (m *LogDebugModel) showSource(key string, line int) {
	m.viewKey = key
	m.cursor = max(line, 1)
	m.centerSource(m.cursor)
}

func (m *LogDebugModel) toggleBreakpoint() {
	source, ok := m.sources[m.viewKey]
	if !ok || m.viewKey == "" || len(source.Lines) == 0 {
		m.status = "No source to set a breakpoint in"
		return
	}
	b := apexlog.Breakpoint{Source: m.viewKey, Line: m.cursor}
	enabled := !m.session.HasBreakpoint(b)
	m.session.SetBreakpoint(b, enabled)
	switch {
	case enabled && !m.session.Executed(m.viewKey, m.cursor):
		m.status = fmt.Sprintf("Breakpoint set at %s line %d; no statement on this line ran in the log, so it will not stop", source.Name, m.cursor)
	case enabled:
		m.status = fmt.Sprintf("Breakpoint set at %s line %d", source.Name, m.cursor)
	default:
		m.status = fmt.Sprintf("Breakpoint removed at %s line %d", source.Name, m.cursor)
	}
}

func (m LogDebugModel) sourceName(key, fallback string) string {
	if s, ok := m.sources[key]; ok && s.Name != "" {
		return s.Name
	}
	return fallback
}

// Layout

type debugLayout struct {
	topHeight    int
	leftWidth    int
	rightWidth   int
	stackHeight  int
	varsHeight   int
	outputHeight int
}

// layout returns the outer sizes of the panes, borders included.
func (m LogDebugModel) layout() debugLayout {
	var l debugLayout
	l.outputHeight = min(max(m.height/4, 4), 10)
	l.topHeight = max(m.height-2-l.outputHeight, 6)
	l.leftWidth = m.width * 3 / 5
	l.rightWidth = m.width - l.leftWidth
	frames := len(m.session.Frames())
	l.stackHeight = min(max(frames, 1)+3, l.topHeight/3)
	l.stackHeight = max(l.stackHeight, 4)
	l.varsHeight = l.topHeight - l.stackHeight
	return l
}

// sourceViewHeight is the number of source lines that fit in the source
// pane.
func (m LogDebugModel) sourceViewHeight() int {
	if m.height == 0 {
		return 0
	}
	return max(m.layout().topHeight-3, 1)
}

func (m *LogDebugModel) centerSource(line int) {
	h := m.sourceViewHeight()
	if h == 0 {
		return
	}
	m.offset = max(line-1-h/2, 0)
}

func (m *LogDebugModel) scrollSourceToCursor() {
	h := m.sourceViewHeight()
	if h == 0 {
		return
	}
	if m.cursor-1 < m.offset {
		m.offset = m.cursor - 1
	}
	if m.cursor-1 >= m.offset+h {
		m.offset = m.cursor - h
	}
}

func (m *LogDebugModel) scrollVariablesToCursor() {
	if m.height == 0 {
		return
	}
	h := max(m.layout().varsHeight-3, 1)
	if m.varCursor < m.varOffset {
		m.varOffset = m.varCursor
	}
	if m.varCursor >= m.varOffset+h {
		m.varOffset = m.varCursor - h + 1
	}
}

// View

func (m LogDebugModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}
	l := m.layout()
	var left string
	if m.picker {
		left = m.renderPicker(l.leftWidth, l.topHeight)
	} else {
		left = m.renderSource(l.leftWidth, l.topHeight)
	}
	right := lipgloss.JoinVertical(lipgloss.Left,
		m.renderStack(l.rightWidth, l.stackHeight),
		m.renderVariables(l.rightWidth, l.varsHeight),
	)
	top := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	return lipgloss.JoinVertical(lipgloss.Left,
		top,
		m.renderOutput(m.width, l.outputHeight),
		fit(m.statusLine(), m.width),
		helpStyle.Render(fit("n next  s step in  o step out  c continue  b breakpoint  f open class  r restart  tab pane  q quit", m.width)),
	)
}

// box draws a bordered pane with outer size width x height. Each line of
// content must already fit in width-2 columns.
func (m LogDebugModel) box(pane debugPane, title string, lines []string, width, height int) string {
	innerWidth := max(width-2, 1)
	innerHeight := max(height-2, 1)
	content := append([]string{paneTitleStyle.Render(fit(title, innerWidth))}, lines...)
	if len(content) > innerHeight {
		content = content[:innerHeight]
	}
	for len(content) < innerHeight {
		content = append(content, "")
	}
	style := unfocusedBorder
	if pane == m.focus && !m.picker {
		style = focusedBorder
	}
	return style.Width(innerWidth).Height(innerHeight).Render(strings.Join(content, "\n"))
}

func (m LogDebugModel) renderSource(width, height int) string {
	innerWidth := max(width-2, 1)
	frame := m.selectedFrame()
	source, ok := m.sources[m.viewKey]
	if m.viewKey == "" || !ok {
		name := "source"
		if frame != nil {
			name = frame.Name
		}
		return m.box(sourcePane, name, []string{fit("No source for this frame", innerWidth)}, width, height)
	}
	if len(source.Lines) == 0 {
		return m.box(sourcePane, source.Name, []string{fit(source.Missing, innerWidth)}, width, height)
	}
	currentLine := 0
	if frame != nil && frame.SourceKey() == m.viewKey {
		currentLine = frame.Line
	}
	digits := len(strconv.Itoa(len(source.Lines)))
	textWidth := max(innerWidth-digits-3, 1)
	var lines []string
	viewHeight := max(height-3, 1)
	for i := m.offset; i < len(source.Lines) && i < m.offset+viewHeight; i++ {
		n := i + 1
		breakpoint, current := " ", " "
		executed := m.session.Executed(m.viewKey, n)
		if m.session.HasBreakpoint(apexlog.Breakpoint{Source: m.viewKey, Line: n}) {
			if executed {
				breakpoint = breakpointStyle.Render("●")
			} else {
				breakpoint = breakpointStyle.Render("○")
			}
		}
		if n == currentLine {
			current = currentStyle.Render("▶")
		}
		marker := breakpoint + current
		number := fmt.Sprintf("%*d", digits, n)
		if n == m.cursor && m.focus == sourcePane {
			number = cursorStyle.Render(number)
		} else if executed {
			number = executedStyle.Render(number)
		} else {
			number = lineNumberStyle.Render(number)
		}
		text := fit(expandTabs(source.Lines[i]), textWidth)
		if n == currentLine {
			text = currentStyle.Render(text)
		}
		lines = append(lines, marker+number+" "+text)
	}
	title := source.Name
	if currentLine > 0 {
		title = fmt.Sprintf("%s  line %d", source.Name, currentLine)
	}
	return m.box(sourcePane, title, lines, width, height)
}

func (m LogDebugModel) renderPicker(width, height int) string {
	innerWidth := max(width-2, 1)
	refs := m.session.Sources()
	viewHeight := max(height-3, 1)
	start := max(m.pickerCursor-viewHeight+1, 0)
	var lines []string
	for i := start; i < len(refs) && i < start+viewHeight; i++ {
		line := fit("  "+m.sourceName(refs[i].Key, refs[i].Name), innerWidth)
		if i == m.pickerCursor {
			line = cursorStyle.Render(line)
		}
		lines = append(lines, line)
	}
	if len(refs) == 0 {
		lines = append(lines, "No classes in this log")
	}
	return focusedBorder.Width(innerWidth).Height(max(height-2, 1)).Render(
		strings.Join(append([]string{paneTitleStyle.Render(fit("Open class (enter to open, esc to close)", innerWidth))}, padLines(lines, max(height-3, 1))...), "\n"))
}

func (m LogDebugModel) renderStack(width, height int) string {
	innerWidth := max(width-2, 1)
	frames := m.session.Frames()
	var lines []string
	for i := 0; i < len(frames); i++ {
		f := frames[len(frames)-1-i]
		label := f.Name
		if f.Line > 0 {
			label = fmt.Sprintf("%s:%d", f.Name, f.Line)
		}
		prefix := "  "
		if i == m.frame {
			prefix = "▶ "
		}
		line := fit(prefix+label, innerWidth)
		if i == m.frame && m.focus == stackPane {
			line = cursorStyle.Render(line)
		}
		lines = append(lines, line)
	}
	return m.box(stackPane, "Call Stack", lines, width, height)
}

type variableRow struct {
	path  string
	label string
	// note is shown dimmed after the label if it fits.
	note       string
	depth      int
	expandable bool
	header     bool
}

func (m LogDebugModel) variableRows() []variableRow {
	frame := m.selectedFrame()
	if frame == nil {
		return nil
	}
	prefix := fmt.Sprintf("%d:%s", len(m.session.Frames())-1-m.frame, frame.Name)
	var rows []variableRow
	rows = append(rows, variableRow{label: "Locals", header: true})
	for _, v := range frame.Locals.Variables() {
		rows = m.appendVariable(rows, prefix+"/local/"+v.Name, v, 1)
	}
	if frame.This != "" {
		this := &apexlog.Variable{Name: "this", Value: frame.This, Ref: frame.This}
		rows = m.appendVariable(rows, prefix+"/this", this, 1)
	}
	if statics := m.session.Statics(frame); statics.Len() > 0 {
		rows = append(rows, variableRow{label: "Statics (" + frame.Class + ")", header: true})
		for _, v := range statics.Variables() {
			rows = m.appendVariable(rows, prefix+"/static/"+v.Name, v, 1)
		}
	}
	return rows
}

func (m LogDebugModel) appendVariable(rows []variableRow, path string, v *apexlog.Variable, depth int) []variableRow {
	children := m.session.Children(v)
	expandable := len(children) > 0 && depth < maxVariableDepth
	row := variableRow{
		path:       path,
		label:      v.Name + " = " + m.displayValue(v, children),
		depth:      depth,
		expandable: expandable,
	}
	if v.Collection() && v.Line > 0 {
		row.note = fmt.Sprintf("as of line %d", v.Line)
	}
	rows = append(rows, row)
	if expandable && m.expanded[path] {
		for _, c := range children {
			rows = m.appendVariable(rows, path+"/"+c.Name, c, depth+1)
		}
	}
	return rows
}

// displayValue summarizes objects whose fields come from a reference
// rather than from the value written in the log.
func (m LogDebugModel) displayValue(v *apexlog.Variable, children []*apexlog.Variable) string {
	if v.Ref != "" && (v.Value == v.Ref || v.Value == "{}") && len(children) > 0 {
		return fmt.Sprintf("{%d fields}", len(children))
	}
	return strings.ReplaceAll(v.Value, "\n", "\\n")
}

func (m LogDebugModel) renderVariables(width, height int) string {
	innerWidth := max(width-2, 1)
	rows := m.variableRows()
	viewHeight := max(height-3, 1)
	var lines []string
	for i := m.varOffset; i < len(rows) && i < m.varOffset+viewHeight; i++ {
		r := rows[i]
		var line string
		if r.header {
			line = fit(r.label, innerWidth)
			line = lineNumberStyle.Render(line)
		} else {
			marker := "  "
			if r.expandable {
				marker = "▸ "
				if m.expanded[r.path] {
					marker = "▾ "
				}
			}
			text := strings.Repeat("  ", r.depth-1) + marker + r.label
			note := ""
			if r.note != "" && lipgloss.Width(text)+2+lipgloss.Width(r.note) <= innerWidth {
				note = "  " + r.note
			}
			switch {
			case i == m.varCursor && m.focus == variablesPane:
				line = fit(text+note, innerWidth)
			case note != "":
				line = variableStyle.Render(text) + lineNumberStyle.Render(fit(note, innerWidth-lipgloss.Width(text)))
			default:
				line = variableStyle.Render(fit(text, innerWidth))
			}
		}
		if i == m.varCursor && m.focus == variablesPane {
			line = cursorStyle.Render(line)
		}
		lines = append(lines, line)
	}
	if len(rows) == 0 {
		lines = append(lines, "No frame selected")
	}
	return m.box(variablesPane, "Variables", lines, width, height)
}

func (m LogDebugModel) renderOutput(width, height int) string {
	innerWidth := max(width-2, 1)
	var all []string
	for _, o := range m.session.Output() {
		prefix := o.Level
		if o.Kind != "DEBUG" {
			prefix = o.Kind
		}
		if o.SourceLine > 0 {
			prefix = fmt.Sprintf("[%d] %s", o.SourceLine, prefix)
		}
		for i, part := range strings.Split(o.Message, "\n") {
			line := prefix + " " + part
			if i > 0 {
				line = strings.Repeat(" ", runewidth.StringWidth(prefix)+1) + part
			}
			line = fit(line, innerWidth)
			if o.Kind != "DEBUG" {
				line = failureStyle.Render(line)
			}
			all = append(all, line)
		}
	}
	viewHeight := max(height-3, 1)
	if len(all) > viewHeight {
		all = all[len(all)-viewHeight:]
	}
	return unfocusedBorder.Width(innerWidth).Height(max(height-2, 1)).Render(
		strings.Join(append([]string{paneTitleStyle.Render(fit("Output", innerWidth))}, padLines(all, viewHeight)...), "\n"))
}

func (m LogDebugModel) statusLine() string {
	switch count := len(m.session.Breakpoints()); count {
	case 0:
		return m.status
	case 1:
		return m.status + "  (1 breakpoint)"
	default:
		return fmt.Sprintf("%s  (%d breakpoints)", m.status, count)
	}
}

// fit truncates s to width columns and pads it with spaces to width.
func fit(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = runewidth.Truncate(s, width, "…")
	if w := runewidth.StringWidth(s); w < width {
		s += strings.Repeat(" ", width-w)
	}
	return s
}

func expandTabs(s string) string {
	return strings.ReplaceAll(s, "\t", "    ")
}

func padLines(lines []string, height int) []string {
	if len(lines) > height {
		return lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines
}
