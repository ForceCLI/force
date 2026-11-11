// Package apexlog parses Apex debug logs and replays them one statement at a
// time.
package apexlog

import (
	"bufio"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// Header holds the lines that precede the first event in a log.
type Header struct {
	// APIVersion is the API version the log was written with, e.g. "62.0".
	APIVersion string
	// Levels maps each log category, e.g. "APEX_CODE", to its level, e.g.
	// "FINEST".
	Levels map[string]string
	// AnonymousSource holds the anonymous Apex that was executed, one entry
	// per source line, taken from the "Execute Anonymous:" header lines.
	AnonymousSource []string
}

// Level returns the level recorded for a log category, or "" if the
// category is not in the header.
func (h Header) Level(category string) string {
	return h.Levels[category]
}

// Event is one log entry.
type Event struct {
	// LogLine is the 1-based line number of the event in the log.
	LogLine int
	// Nanos is the number of nanoseconds since the start of the request.
	Nanos int64
	// Type is the event type, e.g. "METHOD_ENTRY".
	Type string
	// Fields holds the pipe-separated fields that follow the event type.
	// Lines without a timestamp that follow an event are appended to its
	// last field, separated by newlines.
	Fields []string
	// SourceLine is the Apex line number from a leading "[n]" field, or 0
	// if the event has none.
	SourceLine int
}

// Log is a parsed debug log.
type Log struct {
	Header Header
	Events []Event
}

var eventLine = regexp.MustCompile(`^\d{1,2}:\d{2}:\d{2}\.\d+ \((\d+)\)\|`)
var sourceLineField = regexp.MustCompile(`^\[(\d+)\]$`)

const anonymousPrefix = "Execute Anonymous:"

// Parse reads a debug log.
func Parse(r io.Reader) (*Log, error) {
	log := &Log{Header: Header{Levels: map[string]string{}}}
	reader := bufio.NewReader(r)
	lineNumber := 0
	headerLineSeen := false
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 || err == nil {
			lineNumber++
			line = strings.TrimRight(line, "\r\n")
			log.addLine(line, lineNumber, &headerLineSeen)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	log.trimTrailingNewlines()
	return log, nil
}

func (l *Log) addLine(line string, lineNumber int, headerLineSeen *bool) {
	if m := eventLine.FindStringSubmatch(line); m != nil {
		l.Events = append(l.Events, parseEvent(line, lineNumber, m[1]))
		return
	}
	if len(l.Events) > 0 {
		last := &l.Events[len(l.Events)-1]
		if len(last.Fields) == 0 {
			last.Fields = []string{line}
		} else {
			last.Fields[len(last.Fields)-1] += "\n" + line
		}
		return
	}
	if strings.HasPrefix(line, anonymousPrefix) {
		source := strings.TrimPrefix(line, anonymousPrefix)
		source = strings.TrimPrefix(source, " ")
		l.Header.AnonymousSource = append(l.Header.AnonymousSource, source)
		return
	}
	if !*headerLineSeen && strings.TrimSpace(line) != "" {
		*headerLineSeen = true
		l.Header.parseLevels(line)
	}
}

func parseEvent(line string, lineNumber int, nanos string) Event {
	parts := strings.Split(line, "|")
	ns, _ := strconv.ParseInt(nanos, 10, 64)
	e := Event{
		LogLine: lineNumber,
		Nanos:   ns,
		Type:    parts[1],
		Fields:  parts[2:],
	}
	if len(e.Fields) > 0 {
		if m := sourceLineField.FindStringSubmatch(e.Fields[0]); m != nil {
			e.SourceLine, _ = strconv.Atoi(m[1])
		}
	}
	return e
}

// parseLevels reads a header line such as
// "62.0 APEX_CODE,FINEST;APEX_PROFILING,INFO;...".
func (h *Header) parseLevels(line string) {
	version, levels, found := strings.Cut(line, " ")
	if !found {
		return
	}
	h.APIVersion = version
	for _, entry := range strings.Split(levels, ";") {
		category, level, ok := strings.Cut(strings.TrimSpace(entry), ",")
		if ok {
			h.Levels[category] = level
		}
	}
}

// trimTrailingNewlines removes the newlines that blank lines at the end of
// a log add to the last event.
func (l *Log) trimTrailingNewlines() {
	if len(l.Events) == 0 {
		return
	}
	last := &l.Events[len(l.Events)-1]
	if len(last.Fields) > 0 {
		last.Fields[len(last.Fields)-1] = strings.TrimRight(last.Fields[len(last.Fields)-1], "\n")
	}
}
