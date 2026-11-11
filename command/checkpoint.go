package command

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	. "github.com/ForceCLI/force/lib"
	"github.com/ForceCLI/force/lib/apexlog"
)

const checkpointFlagUsage = "save a heap dump when execution first reaches a line, given as Class:line; repeat for up to 5 lines"

// parseCheckpoints parses --checkpoint values of the form Class:line.
func parseCheckpoints(values []string) ([]Checkpoint, error) {
	if len(values) > MaxCheckpoints {
		return nil, fmt.Errorf("at most %d checkpoints can be set; %d were given", MaxCheckpoints, len(values))
	}
	var checkpoints []Checkpoint
	for _, value := range values {
		name, lineText, ok := strings.Cut(value, ":")
		line, err := strconv.Atoi(lineText)
		if !ok || name == "" || err != nil || line < 1 {
			return nil, fmt.Errorf("checkpoint %q is not of the form Class:line", value)
		}
		checkpoints = append(checkpoints, Checkpoint{Name: name, Line: line})
	}
	return checkpoints, nil
}

// createCheckpoints creates the checkpoints and returns a function that
// deletes them, reporting a failure without stopping the command.
func createCheckpoints(checkpoints []Checkpoint) (func(), error) {
	if len(checkpoints) == 0 {
		return func() {}, nil
	}
	remove, err := force.CreateCheckpoints(checkpoints)
	if err != nil {
		return nil, err
	}
	return func() {
		if err := remove(); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: %v\n", err)
		}
	}, nil
}

// addHeapDumps fetches the heap dumps named by the log's HEAP_DUMP events
// and adds them to the session. A heap dump that cannot be fetched, such as
// one that has expired, is reported and skipped.
func addHeapDumps(session *apexlog.Session) {
	for _, id := range apexlog.HeapDumpIDs(session.Log()) {
		raw, err := force.HeapDump(id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: %v\n", err)
			continue
		}
		dump, err := apexlog.ParseHeapDump(raw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: heap dump %s: %v\n", id, err)
			continue
		}
		session.AddHeapDump(id, dump)
	}
}
