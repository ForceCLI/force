package lib

import (
	"context"
	"testing"
	"time"

	"github.com/ForceCLI/force/lib/query"
)

// TestCancelableQueryAndSend_Cancellation ensures that CancelableQueryAndSend stops sending after context cancellation
func TestCancelableQueryAndSend_Cancellation(t *testing.T) {
	// Stub forceQuery to emit two pages with a delay
	orig := forceQuery
	defer func() { forceQuery = orig }()
	forceQuery = func(cb query.PageCallback, opts ...query.Option) error {
		// first page
		r1 := query.Record{Fields: map[string]interface{}{"v": 1}}
		if !cb([]query.Record{r1}) {
			return nil
		}
		// delay before second page
		time.Sleep(50 * time.Millisecond)
		r2 := query.Record{Fields: map[string]interface{}{"v": 2}}
		cb([]query.Record{r2})
		return nil
	}
	f := &Force{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := make(chan ForceRecord, 1)
	// start sending
	var err error
	go func() {
		// cancel shortly after first page is sent
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	err = f.CancelableQueryAndSend(ctx, "", ch)
	if err == nil {
		t.Fatalf("expected error on cancellation, got nil")
	}
	// collect results
	var recs []ForceRecord
	for r := range ch {
		recs = append(recs, r)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record before cancel, got %d", len(recs))
	}
	if recs[0]["v"] != 1 {
		t.Fatalf("expected first record v=1, got %v", recs[0]["v"])
	}
}

// TestAbortableQueryAndSend_Abort ensures that AbortableQueryAndSend stops sending after abort signal
func TestAbortableQueryAndSend_Abort(t *testing.T) {
	orig := forceQuery
	defer func() { forceQuery = orig }()
	forceQuery = func(cb query.PageCallback, opts ...query.Option) error {
		// first page
		r1 := query.Record{Fields: map[string]interface{}{"v": 1}}
		if !cb([]query.Record{r1}) {
			return nil
		}
		// second page
		r2 := query.Record{Fields: map[string]interface{}{"v": 2}}
		cb([]query.Record{r2})
		return nil
	}
	f := &Force{}
	abortCh := make(chan bool)
	ch := make(chan ForceRecord, 1)
	// trigger abort after first record
	go func() {
		time.Sleep(10 * time.Millisecond)
		close(abortCh)
	}()
	err := f.AbortableQueryAndSend("", ch, abortCh)
	if err != nil {
		t.Fatalf("expected no error on abort, got %v", err)
	}
	var recs []ForceRecord
	for r := range ch {
		recs = append(recs, r)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record before abort, got %d", len(recs))
	}
	if recs[0]["v"] != 1 {
		t.Fatalf("expected first record v=1, got %v", recs[0]["v"])
	}
}
