// Package webdebug serves a browser-based debugger that replays an Apex
// debug log.
package webdebug

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/ForceCLI/force/lib/apexlog"
)

//go:embed index.html
var indexHTML []byte

// Source is the source of a class, trigger, or anonymous Apex.
type Source struct {
	Name  string
	Lines []string
	// Missing explains why there are no lines, such as a class in a
	// managed package.
	Missing string
}

// Server serves the debugger page and the API it uses to control a replay.
type Server struct {
	mu      sync.Mutex
	session *apexlog.Session
	sources map[string]Source
	status  string
	token   string
	quit    chan struct{}
	once    sync.Once
}

// NewServer returns a server for a session, stopped at the first
// statement.
func NewServer(session *apexlog.Session, sources map[string]Source) (*Server, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	s := &Server{session: session, sources: sources, token: hex.EncodeToString(b), quit: make(chan struct{})}
	s.afterStop(session.Resume(apexlog.StepIn))
	return s, nil
}

// Token is the value API requests must send in the X-Debug-Token header.
// It is given to the page in the URL, so that other local users and web
// pages cannot control the debugger.
func (s *Server) Token() string {
	return s.token
}

// Run listens on a random port of 127.0.0.1, calls ready with the
// debugger's URL, and serves until the page's Quit button is pressed or
// ctx is done.
func (s *Server) Run(ctx context.Context, ready func(url string)) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	server := &http.Server{Handler: s.Handler()}
	errs := make(chan error, 1)
	go func() { errs <- server.Serve(listener) }()
	ready(fmt.Sprintf("http://%s/?token=%s", listener.Addr(), s.token))
	select {
	case <-ctx.Done():
	case <-s.quit:
	case err := <-errs:
		return err
	}
	err = server.Shutdown(context.Background())
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Handler returns the HTTP handler for the page and the API.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(indexHTML)
	})
	mux.HandleFunc("GET /api/state", s.api(func(r *http.Request) (any, error) {
		return s.state(), nil
	}))
	mux.HandleFunc("GET /api/source", s.api(func(r *http.Request) (any, error) {
		return s.source(r.URL.Query().Get("key")), nil
	}))
	mux.HandleFunc("GET /api/variables", s.api(func(r *http.Request) (any, error) {
		var path []string
		if p := r.URL.Query().Get("path"); p != "" {
			if err := json.Unmarshal([]byte(p), &path); err != nil {
				return nil, fmt.Errorf("path: %w", err)
			}
		}
		var frame int
		fmt.Sscan(r.URL.Query().Get("frame"), &frame)
		return s.variables(frame, path)
	}))
	mux.HandleFunc("POST /api/step", s.api(func(r *http.Request) (any, error) {
		if err := s.step(r.URL.Query().Get("kind")); err != nil {
			return nil, err
		}
		return s.state(), nil
	}))
	mux.HandleFunc("POST /api/breakpoint", s.api(func(r *http.Request) (any, error) {
		var b apexlog.Breakpoint
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			return nil, err
		}
		s.toggleBreakpoint(b)
		return s.state(), nil
	}))
	mux.HandleFunc("POST /api/quit", s.api(func(r *http.Request) (any, error) {
		s.once.Do(func() { close(s.quit) })
		return map[string]bool{"ok": true}, nil
	}))
	return mux
}

func (s *Server) api(handle func(r *http.Request) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Debug-Token")
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.token)) != 1 {
			http.Error(w, "missing or wrong X-Debug-Token", http.StatusForbidden)
			return
		}
		s.mu.Lock()
		result, err := handle(r)
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(result)
	}
}

type frameJSON struct {
	Index  int    `json:"index"`
	Name   string `json:"name"`
	Line   int    `json:"line"`
	Source string `json:"source"`
}

type sourceRefJSON struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type outputJSON struct {
	Kind    string `json:"kind"`
	Level   string `json:"level,omitempty"`
	Line    int    `json:"line"`
	Message string `json:"message"`
}

type stateJSON struct {
	Status      string               `json:"status"`
	Done        bool                 `json:"done"`
	AtHeapDump  bool                 `json:"atHeapDump"`
	Statement   int                  `json:"statement"`
	Frames      []frameJSON          `json:"frames"`
	Output      []outputJSON         `json:"output"`
	Breakpoints []apexlog.Breakpoint `json:"breakpoints"`
	Sources     []sourceRefJSON      `json:"sources"`
}

// state returns the replay's position. Frames are listed from the
// innermost frame outwards; a frame's index is its position in
// Session.Frames.
func (s *Server) state() stateJSON {
	st := stateJSON{
		Status:      s.status,
		Done:        s.session.Done(),
		AtHeapDump:  s.session.AtHeapDump(),
		Statement:   s.session.Statement(),
		Frames:      []frameJSON{},
		Output:      []outputJSON{},
		Breakpoints: s.session.Breakpoints(),
		Sources:     []sourceRefJSON{},
	}
	if !st.Done {
		frames := s.session.Frames()
		for i := len(frames) - 1; i >= 0; i-- {
			f := frames[i]
			st.Frames = append(st.Frames, frameJSON{Index: i, Name: f.Name, Line: f.Line, Source: f.SourceKey()})
		}
	}
	for _, o := range s.session.Output() {
		st.Output = append(st.Output, outputJSON{Kind: o.Kind, Level: o.Level, Line: o.SourceLine, Message: o.Message})
	}
	for _, ref := range s.session.Sources() {
		st.Sources = append(st.Sources, sourceRefJSON{Key: ref.Key, Name: s.sourceName(ref.Key, ref.Name)})
	}
	return st
}

type sourceJSON struct {
	Key      string   `json:"key"`
	Name     string   `json:"name"`
	Lines    []string `json:"lines"`
	Missing  string   `json:"missing,omitempty"`
	Executed []int    `json:"executed"`
}

func (s *Server) source(key string) sourceJSON {
	src, ok := s.sources[key]
	result := sourceJSON{Key: key, Name: src.Name, Lines: src.Lines, Missing: src.Missing, Executed: []int{}}
	if !ok {
		result.Missing = "No source for " + key + "."
	}
	if result.Lines == nil {
		result.Lines = []string{}
	}
	for n := 1; n <= len(src.Lines); n++ {
		if s.session.Executed(key, n) {
			result.Executed = append(result.Executed, n)
		}
	}
	return result
}

func (s *Server) sourceName(key, fallback string) string {
	if src, ok := s.sources[key]; ok && src.Name != "" {
		return src.Name
	}
	return fallback
}

type variableJSON struct {
	Name       string   `json:"name"`
	Type       string   `json:"type,omitempty"`
	Value      string   `json:"value"`
	Note       string   `json:"note,omitempty"`
	Expandable bool     `json:"expandable"`
	Path       []string `json:"path"`
}

type groupJSON struct {
	Title     string         `json:"title"`
	Variables []variableJSON `json:"variables"`
}

type variablesJSON struct {
	Groups    []groupJSON    `json:"groups,omitempty"`
	Variables []variableJSON `json:"variables,omitempty"`
}

// variables returns a frame's variables, grouped into locals, this, and
// statics, or with a path, the children of the variable at that path. A
// path starts with "local", "this", or "static", then the variable's name,
// then the names of children.
func (s *Server) variables(frameIndex int, path []string) (variablesJSON, error) {
	frames := s.session.Frames()
	if s.session.Done() || frameIndex < 0 || frameIndex >= len(frames) {
		return variablesJSON{Groups: []groupJSON{}}, nil
	}
	frame := frames[frameIndex]
	if len(path) == 0 {
		locals := groupJSON{Title: "Locals", Variables: []variableJSON{}}
		for _, v := range frame.Locals.Variables() {
			locals.Variables = append(locals.Variables, s.variable(v, []string{"local", v.Name}))
		}
		groups := []groupJSON{locals}
		if frame.This != "" {
			this := &apexlog.Variable{Name: "this", Value: frame.This, Ref: frame.This}
			groups = append(groups, groupJSON{Title: "This", Variables: []variableJSON{s.variable(this, []string{"this", "this"})}})
		}
		if statics := s.session.Statics(frame); statics.Len() > 0 {
			group := groupJSON{Title: "Statics (" + frame.Class + ")"}
			for _, v := range statics.Variables() {
				group.Variables = append(group.Variables, s.variable(v, []string{"static", v.Name}))
			}
			groups = append(groups, group)
		}
		return variablesJSON{Groups: groups}, nil
	}
	if len(path) < 2 {
		return variablesJSON{}, errors.New("path must name a variable")
	}
	var v *apexlog.Variable
	switch path[0] {
	case "local":
		v, _ = frame.Locals.Get(path[1])
	case "static":
		if statics := s.session.Statics(frame); statics != nil {
			v, _ = statics.Get(path[1])
		}
	case "this":
		if frame.This != "" {
			v = &apexlog.Variable{Name: "this", Value: frame.This, Ref: frame.This}
		}
	}
	for _, name := range path[2:] {
		if v == nil {
			break
		}
		var next *apexlog.Variable
		for _, c := range s.session.Children(v) {
			if c.Name == name {
				next = c
				break
			}
		}
		v = next
	}
	if v == nil {
		return variablesJSON{}, fmt.Errorf("no variable at %s", strings.Join(path, "/"))
	}
	result := variablesJSON{Variables: []variableJSON{}}
	for _, c := range s.session.Children(v) {
		result.Variables = append(result.Variables, s.variable(c, append(append([]string{}, path...), c.Name)))
	}
	return result, nil
}

func (s *Server) variable(v *apexlog.Variable, path []string) variableJSON {
	children := s.session.Children(v)
	result := variableJSON{
		Name:       v.Name,
		Type:       v.Type,
		Value:      displayValue(s.session, v, children),
		Expandable: len(children) > 0,
		Path:       path,
	}
	if v.Collection() && v.Line > 0 {
		result.Note = fmt.Sprintf("as of line %d", v.Line)
	}
	return result
}

// displayValue summarizes objects whose fields come from a reference rather
// than from the value written in the log.
func displayValue(session *apexlog.Session, v *apexlog.Variable, children []*apexlog.Variable) string {
	if v.Ref != "" && (v.Value == v.Ref || v.Value == "{}") && len(children) > 0 {
		summary := fmt.Sprintf("{%d fields}", len(children))
		if strings.HasPrefix(children[0].Name, "[") {
			summary = fmt.Sprintf("[%d elements]", len(children))
		}
		if t := session.ObjectType(v.Ref); t != "" {
			summary = t + " " + summary
		}
		return summary
	}
	return v.Value
}

func (s *Server) step(kind string) error {
	switch kind {
	case "in":
		s.afterStop(s.session.Resume(apexlog.StepIn))
	case "over":
		s.afterStop(s.session.Resume(apexlog.StepOver))
	case "out":
		s.afterStop(s.session.Resume(apexlog.StepOut))
	case "continue":
		s.afterStop(s.session.Resume(apexlog.Continue))
	case "back":
		if s.session.StepBack() == apexlog.StoppedAtStart {
			s.status = "At the first statement"
			return nil
		}
		s.afterStop(apexlog.StoppedOnStep)
		s.status = fmt.Sprintf("Stepped back to statement %d", s.session.Statement())
	case "restart":
		s.session.Restart()
		s.afterStop(s.session.Resume(apexlog.StepIn))
		s.status = "Restarted"
	default:
		return fmt.Errorf("unknown step %q", kind)
	}
	return nil
}

func (s *Server) afterStop(reason apexlog.StopReason) {
	top := s.session.Top()
	switch {
	case reason == apexlog.StoppedAtEnd || top == nil:
		s.status = "End of log"
		return
	case reason == apexlog.StoppedOnBreakpoint:
		s.status = fmt.Sprintf("Breakpoint at %s line %d", s.sourceName(top.SourceKey(), top.Name), top.Line)
	default:
		s.status = "Paused"
	}
	if s.session.AtHeapDump() {
		s.status += "; variables are from the heap dump taken before this line"
	}
}

func (s *Server) toggleBreakpoint(b apexlog.Breakpoint) {
	name := s.sourceName(b.Source, b.Source)
	enabled := !s.session.HasBreakpoint(b)
	s.session.SetBreakpoint(b, enabled)
	switch {
	case enabled && !s.session.Executed(b.Source, b.Line):
		s.status = fmt.Sprintf("Breakpoint set at %s line %d; no statement on this line ran in the log, so it will not stop", name, b.Line)
	case enabled:
		s.status = fmt.Sprintf("Breakpoint set at %s line %d", name, b.Line)
	default:
		s.status = fmt.Sprintf("Breakpoint removed at %s line %d", name, b.Line)
	}
}
