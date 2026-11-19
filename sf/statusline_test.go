package sf

import (
	"bytes"
	"testing"
)

func TestStatusLineDrawsTheSpinnerFrameBeforeTheStatus(t *testing.T) {
	t.Setenv("FORCE_COLOR", "1")
	var out bytes.Buffer
	line := &statusLine{out: &out, message: "Version create.", terminal: true, status: "10 minutes remaining until timeout. Create version status: Initializing"}
	line.draw()
	line.draw()
	want := "\r\x1b[2KVersion create.... \x1b[35m⣾\x1b[39m 10 minutes remaining until timeout. Create version status: Initializing" +
		"\r\x1b[2KVersion create.... \x1b[35m⣽\x1b[39m 10 minutes remaining until timeout. Create version status: Initializing"
	if out.String() != want {
		t.Errorf("got  %q\nwant %q", out.String(), want)
	}
	out.Reset()
	line.stop("Create version status: Success")
	if out.String() != "\r\x1b[2KVersion create.... Create version status: Success\n" {
		t.Errorf("stop: %q", out.String())
	}
}

func TestAbandonStatusLineEndsTheLineBeforeAnError(t *testing.T) {
	var out bytes.Buffer
	line := &statusLine{out: &out, message: "Working", terminal: true, done: make(chan struct{})}
	activeStatusLine = line
	abandonStatusLine()
	abandonStatusLine()
	if out.String() != "\r\x1b[2KWorking... \n" || line.done != nil || activeStatusLine != nil {
		t.Errorf("abandon: %q", out.String())
	}
}
