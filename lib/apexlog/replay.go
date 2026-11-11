package apexlog

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// AnonymousKey is the source key of frames that run anonymous Apex.
const AnonymousKey = "anonymous"

// FrameKind identifies the event that created a frame.
type FrameKind int

const (
	CodeUnitFrame FrameKind = iota
	MethodFrame
	ConstructorFrame
)

// Frame is one entry in the replayed call stack.
type Frame struct {
	Kind FrameKind
	// Name is the name shown in the call stack.
	Name string
	// Signature is the last field of the entry event. Exit events are
	// matched against it.
	Signature string
	// EntityID is the 15-character ID of the Apex class or trigger, or ""
	// if the log does not give one (system methods, flows, anonymous Apex).
	EntityID string
	// Anonymous is true for the frame that runs anonymous Apex.
	Anonymous bool
	// Class is the name used to look up the frame's static variables.
	Class string
	// Line is the current line, or 0 before the first statement.
	Line int
	// Locals holds the frame's local variables.
	Locals *Scope
	// This is the reference of the object the frame runs on, if known.
	This string
	// Initializer is true for the frame that initializes a class's static
	// variables before the class is first used. Stepping does not stop in
	// it; breakpoints do.
	Initializer bool
	// systemDepth counts the system methods (SYSTEM_METHOD_ENTRY) the frame
	// has called that have not returned. Statements logged inside them refer
	// to lines of Salesforce's code, not the frame's.
	systemDepth int
}

// SourceKey returns the key used to look up the frame's source and
// breakpoints: AnonymousKey for anonymous Apex, the entity ID for classes
// and triggers, and "" for frames without source.
func (f *Frame) SourceKey() string {
	if f.Anonymous {
		return AnonymousKey
	}
	return f.EntityID
}

// Variable is a named value.
type Variable struct {
	Name string
	Type string
	// Value is the value as written in the log. Strings are shown in
	// single quotes. Objects and collections are JSON.
	Value string
	// Ref is the reference of the object the variable holds, if the log
	// gave one.
	Ref string
	// Line is the line of the variable's last assignment in the log, or 0
	// if the log has not assigned it.
	Line int
}

// Collection reports whether the variable holds a list, set, or map. The log
// records a collection's contents only when the variable is assigned;
// changes made by methods such as add or put are not logged, so the value
// is the contents as of the last assignment.
func (v *Variable) Collection() bool {
	for _, prefix := range []string{"List<", "Set<", "Map<"} {
		if strings.HasPrefix(v.Type, prefix) {
			return true
		}
	}
	if strings.HasSuffix(v.Type, "[]") {
		return true
	}
	return v.Type == "" && (strings.HasPrefix(v.Value, "[") ||
		strings.HasPrefix(v.Value, "'List of size ") ||
		strings.HasPrefix(v.Value, "'Set of size ") ||
		strings.HasPrefix(v.Value, "'Map of size "))
}

// Scope is an ordered set of variables.
type Scope struct {
	names []string
	vars  map[string]*Variable
}

func newScope() *Scope {
	return &Scope{vars: map[string]*Variable{}}
}

// Get returns the variable with the given name.
func (s *Scope) Get(name string) (*Variable, bool) {
	v, ok := s.vars[name]
	return v, ok
}

// Variables returns the variables in the order they were declared.
func (s *Scope) Variables() []*Variable {
	if s == nil {
		return nil
	}
	result := make([]*Variable, len(s.names))
	for i, name := range s.names {
		result[i] = s.vars[name]
	}
	return result
}

// Len returns the number of variables.
func (s *Scope) Len() int {
	if s == nil {
		return 0
	}
	return len(s.names)
}

func (s *Scope) declare(name, typ string) *Variable {
	if v, ok := s.vars[name]; ok {
		return v
	}
	v := &Variable{Name: name, Type: typ, Value: "null"}
	s.vars[name] = v
	s.names = append(s.names, name)
	return v
}

func (s *Scope) assign(name, value, ref string, line int) {
	v := s.declare(name, "")
	v.Value = value
	v.Ref = ref
	v.Line = line
}

// Output is a line of program output: a debug statement or an exception.
type Output struct {
	LogLine int
	// Kind is "DEBUG", "EXCEPTION" or "FATAL_ERROR".
	Kind string
	// Level is the logging level of a debug statement.
	Level      string
	SourceLine int
	Message    string
}

// Breakpoint is a line in a source.
type Breakpoint struct {
	// Source is AnonymousKey or a 15-character entity ID.
	Source string
	Line   int
}

// StepKind is how far Resume runs.
type StepKind int

const (
	// StepIn stops at the next statement.
	StepIn StepKind = iota
	// StepOver stops at the next statement in the current frame or a
	// caller.
	StepOver
	// StepOut stops at the next statement in a caller.
	StepOut
	// Continue stops only at a breakpoint.
	Continue
)

// StopReason is why Resume stopped.
type StopReason int

const (
	StoppedOnStep StopReason = iota
	StoppedOnBreakpoint
	// StoppedAtEnd means the log has no more statements.
	StoppedAtEnd
)

// SourceRef identifies an Apex class or trigger that ran in the log.
type SourceRef struct {
	Key  string
	Name string
}

// ErrLogLevel is returned when a log was not captured at a level that
// records statements and variables.
type ErrLogLevel struct {
	Level string
}

func (e ErrLogLevel) Error() string {
	level := e.Level
	if level == "" {
		level = "not set"
	}
	return fmt.Sprintf("log has APEX_CODE level %s; replaying a log requires APEX_CODE at FINEST", level)
}

// Session replays a log.
type Session struct {
	log         *Log
	pos         int
	paused      bool
	done        bool
	frames      []*Frame
	statics     map[string]*Scope
	objects     map[string]*Scope
	output      []Output
	breakpoints map[Breakpoint]bool
	sources     []SourceRef
	executed    map[string]map[int]bool
}

// NewSession returns a session positioned before the first event. It
// returns ErrLogLevel if the log was not captured with APEX_CODE at FINEST.
func NewSession(log *Log) (*Session, error) {
	if level := log.Header.Level("APEX_CODE"); level != "FINEST" {
		return nil, ErrLogLevel{Level: level}
	}
	s := &Session{log: log, breakpoints: map[Breakpoint]bool{}}
	s.Restart()
	s.sources = findSources(log)
	s.executed = executedLines(log)
	return s, nil
}

// executedLines replays the whole log and records the lines of each source
// on which a statement ran.
func executedLines(log *Log) map[string]map[int]bool {
	replay := &Session{log: log, breakpoints: map[Breakpoint]bool{}}
	replay.Restart()
	executed := map[string]map[int]bool{}
	for replay.advance() {
		top := replay.Top()
		key := top.SourceKey()
		if executed[key] == nil {
			executed[key] = map[int]bool{}
		}
		executed[key][top.Line] = true
	}
	return executed
}

// Executed reports whether a statement on a line of a source ran in the
// log. A breakpoint on a line that did not run is never reached.
func (s *Session) Executed(source string, line int) bool {
	return s.executed[source][line]
}

// Restart moves the session back to the start of the log. Breakpoints are
// kept.
func (s *Session) Restart() {
	s.pos = 0
	s.paused = false
	s.done = false
	s.frames = nil
	s.statics = map[string]*Scope{}
	s.objects = map[string]*Scope{}
	s.output = nil
}

// Log returns the log being replayed.
func (s *Session) Log() *Log {
	return s.log
}

// Done reports whether the session has reached the end of the log.
func (s *Session) Done() bool {
	return s.done
}

// Frames returns the call stack, outermost frame first.
func (s *Session) Frames() []*Frame {
	return s.frames
}

// Top returns the innermost frame, or nil if the stack is empty.
func (s *Session) Top() *Frame {
	if len(s.frames) == 0 {
		return nil
	}
	return s.frames[len(s.frames)-1]
}

// Statics returns the static variables of a frame's class.
func (s *Session) Statics(f *Frame) *Scope {
	if f == nil || f.Class == "" {
		return nil
	}
	return s.statics[f.Class]
}

// Object returns the fields recorded for an object reference.
func (s *Session) Object(ref string) *Scope {
	return s.objects[ref]
}

// Output returns the program output written up to the current position.
func (s *Session) Output() []Output {
	return s.output
}

// Sources returns the classes and triggers that have frames in the log,
// with names taken from the log, sorted by name. Anonymous Apex is listed
// first when the log has any.
func (s *Session) Sources() []SourceRef {
	return s.sources
}

// SetBreakpoint adds or removes a breakpoint.
func (s *Session) SetBreakpoint(b Breakpoint, enabled bool) {
	if enabled {
		s.breakpoints[b] = true
	} else {
		delete(s.breakpoints, b)
	}
}

// HasBreakpoint reports whether a breakpoint is set.
func (s *Session) HasBreakpoint(b Breakpoint) bool {
	return s.breakpoints[b]
}

// Breakpoints returns the breakpoints that are set.
func (s *Session) Breakpoints() []Breakpoint {
	result := make([]Breakpoint, 0, len(s.breakpoints))
	for b := range s.breakpoints {
		result = append(result, b)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Source != result[j].Source {
			return result[i].Source < result[j].Source
		}
		return result[i].Line < result[j].Line
	})
	return result
}

// Resume replays events until the step is complete, a breakpoint is
// reached, or the log ends.
func (s *Session) Resume(kind StepKind) StopReason {
	previousDepth := len(s.frames)
	for s.advance() {
		depth := len(s.frames)
		if s.atBreakpoint() {
			return StoppedOnBreakpoint
		}
		if s.Top().Initializer {
			continue
		}
		switch kind {
		case StepIn:
			return StoppedOnStep
		case StepOver:
			if depth <= previousDepth {
				return StoppedOnStep
			}
		case StepOut:
			if depth < previousDepth {
				return StoppedOnStep
			}
		}
	}
	return StoppedAtEnd
}

func (s *Session) atBreakpoint() bool {
	top := s.Top()
	if top == nil {
		return false
	}
	return s.breakpoints[Breakpoint{Source: top.SourceKey(), Line: top.Line}]
}

// advance processes events until the next statement. It returns false when
// the log ends first.
func (s *Session) advance() bool {
	for s.pos < len(s.log.Events) {
		e := s.log.Events[s.pos]
		s.pos++
		s.paused = false
		s.handle(e)
		if s.paused {
			return true
		}
	}
	s.done = true
	return false
}

func (s *Session) handle(e Event) {
	switch e.Type {
	case "CODE_UNIT_STARTED":
		s.push(codeUnitFrame(e))
	case "METHOD_ENTRY":
		s.push(methodFrame(e))
	case "CONSTRUCTOR_ENTRY":
		s.push(constructorFrame(e))
	case "CODE_UNIT_FINISHED":
		s.pop(CodeUnitFrame, lastField(e))
	case "METHOD_EXIT":
		s.pop(MethodFrame, lastField(e))
	case "CONSTRUCTOR_EXIT":
		s.pop(ConstructorFrame, lastField(e))
	case "SYSTEM_METHOD_ENTRY", "SYSTEM_CONSTRUCTOR_ENTRY":
		if top := s.Top(); top != nil {
			top.systemDepth++
		}
	case "SYSTEM_METHOD_EXIT", "SYSTEM_CONSTRUCTOR_EXIT":
		if top := s.Top(); top != nil && top.systemDepth > 0 {
			top.systemDepth--
		}
	case "STATEMENT_EXECUTE":
		if top := s.Top(); top != nil && e.SourceLine > 0 && top.systemDepth == 0 {
			top.Line = e.SourceLine
			s.paused = true
		}
	case "VARIABLE_SCOPE_BEGIN":
		s.declare(e)
	case "VARIABLE_ASSIGNMENT":
		s.assign(e)
	case "USER_DEBUG":
		if len(e.Fields) >= 3 {
			s.output = append(s.output, Output{
				LogLine:    e.LogLine,
				Kind:       "DEBUG",
				Level:      e.Fields[1],
				SourceLine: e.SourceLine,
				Message:    strings.Join(e.Fields[2:], "|"),
			})
		}
	case "EXCEPTION_THROWN", "FATAL_ERROR":
		fields := e.Fields
		if e.SourceLine > 0 || (len(fields) > 0 && fields[0] == "[EXTERNAL]") {
			fields = fields[1:]
		}
		kind := "EXCEPTION"
		if e.Type == "FATAL_ERROR" {
			kind = "FATAL_ERROR"
		}
		s.output = append(s.output, Output{
			LogLine:    e.LogLine,
			Kind:       kind,
			SourceLine: e.SourceLine,
			Message:    strings.Join(fields, "|"),
		})
	}
}

func (s *Session) push(f *Frame) {
	f.Locals = newScope()
	s.frames = append(s.frames, f)
}

// pop removes frames down to and including the innermost frame that
// matches an exit event. If no frame matches, the stack is left unchanged.
func (s *Session) pop(kind FrameKind, signature string) {
	for i := len(s.frames) - 1; i >= 0; i-- {
		f := s.frames[i]
		if f.Kind == kind && exitMatches(f.Signature, signature) {
			s.frames = s.frames[:i]
			return
		}
	}
}

// exitMatches reports whether an exit event's signature refers to a frame.
// METHOD_EXIT for a class's implicit initializer gives only the class name,
// e.g. "Calc" for the entry signature "Calc.Calc()".
func exitMatches(entry, exit string) bool {
	return entry == exit ||
		strings.HasPrefix(entry, exit+".") ||
		strings.HasPrefix(entry, exit+"(")
}

func (s *Session) declare(e Event) {
	if len(e.Fields) < 5 {
		return
	}
	name, typ, isStatic := e.Fields[1], e.Fields[2], e.Fields[4] == "true"
	if isStatic {
		class, varName := splitLast(name)
		if class == "" {
			return
		}
		s.staticScope(class).declare(varName, typ)
		return
	}
	// The object a frame runs on is recorded in Frame.This, not as a local.
	if name == "this" {
		return
	}
	if top := s.Top(); top != nil {
		top.Locals.declare(name, typ)
	}
}

func (s *Session) assign(e Event) {
	if len(e.Fields) < 3 {
		return
	}
	name := e.Fields[1]
	rest := e.Fields[2:]
	ref := ""
	if len(rest) >= 2 && isRef(rest[len(rest)-1]) {
		ref = rest[len(rest)-1]
		rest = rest[:len(rest)-1]
	}
	value := logValue(strings.Join(rest, "|"))
	if ref == "" && isRef(value) {
		if _, ok := s.objects[value]; ok {
			ref = value
		}
	}
	if ref != "" {
		s.recordObject(ref, value)
	}

	top := s.Top()
	switch {
	case name == "this":
		if top != nil && ref != "" {
			top.This = ref
		}
	case strings.HasPrefix(name, "this."):
		field := strings.TrimPrefix(name, "this.")
		target := ref
		if target == "" && top != nil {
			target = top.This
		}
		if target != "" {
			s.objectScope(target).assign(field, value, childRef(s, value), e.SourceLine)
		}
	default:
		if class, varName := splitLast(name); class != "" {
			if scope, ok := s.statics[class]; ok {
				scope.assign(varName, value, ref, e.SourceLine)
				return
			}
		}
		if top != nil {
			top.Locals.assign(name, value, ref, e.SourceLine)
		}
	}
}

// recordObject stores the fields of a JSON object value under its
// reference, so that a variable holding the reference shows the fields.
func (s *Session) recordObject(ref, value string) {
	scope := s.objectScope(ref)
	if !strings.HasPrefix(value, "{") {
		return
	}
	children, ok := jsonChildren(value)
	if !ok {
		return
	}
	for _, c := range children {
		scope.assign(c.Name, c.Value, childRef(s, c.Value), 0)
	}
}

// childRef returns value if it is the reference of a recorded object.
func childRef(s *Session, value string) string {
	if isRef(value) {
		if _, ok := s.objects[value]; ok {
			return value
		}
	}
	return ""
}

func (s *Session) staticScope(class string) *Scope {
	scope, ok := s.statics[class]
	if !ok {
		scope = newScope()
		s.statics[class] = scope
	}
	return scope
}

func (s *Session) objectScope(ref string) *Scope {
	scope, ok := s.objects[ref]
	if !ok {
		scope = newScope()
		s.objects[ref] = scope
	}
	return scope
}

// Children returns the members of a variable's value: the fields recorded
// for its object reference, or else the entries of its JSON value.
func (s *Session) Children(v *Variable) []*Variable {
	if v.Ref != "" {
		if scope := s.objects[v.Ref]; scope.Len() > 0 {
			return scope.Variables()
		}
	}
	children, ok := jsonChildren(v.Value)
	if !ok {
		return nil
	}
	for _, c := range children {
		c.Ref = childRef(s, c.Value)
	}
	return children
}

// HasChildren reports whether Children returns any members.
func (s *Session) HasChildren(v *Variable) bool {
	return len(s.Children(v)) > 0
}

var refPattern = regexp.MustCompile(`^0x[0-9a-fA-F]+$`)

func isRef(s string) bool {
	return refPattern.MatchString(s)
}

// logValue converts a value as written in the log to the form shown to
// the user. The log writes strings in double quotes; Apex uses single
// quotes.
func logValue(raw string) string {
	if len(raw) >= 2 && strings.HasPrefix(raw, `"`) && strings.HasSuffix(raw, `"`) {
		return "'" + raw[1:len(raw)-1] + "'"
	}
	return raw
}

func lastField(e Event) string {
	if len(e.Fields) == 0 {
		return ""
	}
	return e.Fields[len(e.Fields)-1]
}

// splitLast splits "A.B.c" into "A.B" and "c". It returns "" and the name
// when the name has no dot.
func splitLast(name string) (string, string) {
	i := strings.LastIndex(name, ".")
	if i < 0 {
		return "", name
	}
	return name[:i], name[i+1:]
}

var entityIDPattern = regexp.MustCompile(`^01[pq][0-9A-Za-z]{12}(?:[0-9A-Za-z]{3})?$`)

// entityID returns the 15-character class or trigger ID among an event's
// fields, or "".
func entityID(e Event) string {
	for _, field := range e.Fields {
		if entityIDPattern.MatchString(field) {
			return field[:15]
		}
	}
	return ""
}

const triggerPrefix = "__sfdc_trigger/"

func codeUnitFrame(e Event) *Frame {
	signature := lastField(e)
	f := &Frame{Kind: CodeUnitFrame, Signature: signature, Name: signature, EntityID: entityID(e)}
	switch {
	case signature == "execute_anonymous_apex":
		f.Anonymous = true
		f.Name = "anonymous Apex"
	case strings.HasPrefix(signature, triggerPrefix):
		f.Name = strings.TrimPrefix(signature, triggerPrefix)
	default:
		f.Class = methodClass(signature)
	}
	return f
}

func methodFrame(e Event) *Frame {
	signature := lastField(e)
	f := &Frame{
		Kind:      MethodFrame,
		Signature: signature,
		Name:      signature,
		EntityID:  entityID(e),
		Class:     methodClass(signature),
	}
	if isInitializer(signature) {
		f.Initializer = true
		f.Name = f.Class + " static initialization"
	}
	return f
}

// isInitializer reports whether a METHOD_ENTRY signature is a class's static
// initialization, which the log gives as a method named after the class
// ("Calc.Calc()"). Apex does not allow a method named after its class;
// constructors are logged with CONSTRUCTOR_ENTRY.
func isInitializer(signature string) bool {
	name, ok := strings.CutSuffix(signature, "()")
	if !ok {
		return false
	}
	class, method := splitLast(name)
	_, last := splitLast(class)
	return method != "" && last == method
}

// constructorFrame handles CONSTRUCTOR_ENTRY|[n]|<id>|<init>(Integer)|Calc.Result
func constructorFrame(e Event) *Frame {
	signature := lastField(e)
	params := ""
	if len(e.Fields) >= 2 {
		params = strings.TrimPrefix(e.Fields[len(e.Fields)-2], "<init>")
	}
	class := signature
	if i := strings.LastIndex(class, "/"); i >= 0 {
		class = class[i+1:]
	}
	return &Frame{
		Kind:      ConstructorFrame,
		Signature: signature,
		Name:      "new " + class + params,
		EntityID:  entityID(e),
		Class:     class,
	}
}

// methodClass returns the class of a method signature: "Calc.Result" for
// "Calc.Result.total(Integer)".
func methodClass(signature string) string {
	name, _, _ := strings.Cut(signature, "(")
	class, _ := splitLast(name)
	return class
}

func findSources(log *Log) []SourceRef {
	names := map[string]string{}
	anonymous := false
	for _, e := range log.Events {
		switch e.Type {
		case "CODE_UNIT_STARTED", "METHOD_ENTRY", "CONSTRUCTOR_ENTRY":
		default:
			continue
		}
		if e.Type == "CODE_UNIT_STARTED" && lastField(e) == "execute_anonymous_apex" {
			anonymous = true
			continue
		}
		id := entityID(e)
		if id == "" || names[id] != "" {
			continue
		}
		names[id] = sourceName(e)
	}
	var refs []SourceRef
	for id, name := range names {
		refs = append(refs, SourceRef{Key: id, Name: name})
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Name != refs[j].Name {
			return refs[i].Name < refs[j].Name
		}
		return refs[i].Key < refs[j].Key
	})
	if anonymous {
		refs = append([]SourceRef{{Key: AnonymousKey, Name: "anonymous Apex"}}, refs...)
	}
	return refs
}

// sourceName returns the top-level class or trigger name of an entry
// event, as far as the log shows it.
func sourceName(e Event) string {
	signature := lastField(e)
	if strings.HasPrefix(signature, triggerPrefix) {
		name := strings.TrimPrefix(signature, triggerPrefix)
		if i := strings.LastIndex(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		return name
	}
	if i := strings.LastIndex(signature, "/"); i >= 0 {
		signature = signature[i+1:]
	}
	name, _, _ := strings.Cut(signature, "(")
	first, _, _ := strings.Cut(name, ".")
	return first
}
