package lib

import (
	"errors"
	"testing"
	"time"
)

func TestDeployPollIntervalBacksOffToFiveSeconds(t *testing.T) {
	want := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		5 * time.Second,
		5 * time.Second,
	}
	for waits, w := range want {
		if got := deployPollInterval(waits); got != w {
			t.Errorf("deployPollInterval(%d) = %s, want %s", waits, got, w)
		}
	}
	if got := deployPollInterval(1000); got != 5*time.Second {
		t.Errorf("deployPollInterval(1000) = %s, want 5s", got)
	}
}

func TestDeployPollIntervalNegativeWaitsUsesInitial(t *testing.T) {
	if got := deployPollInterval(-1); got != time.Second {
		t.Errorf("deployPollInterval(-1) = %s, want 1s", got)
	}
}

// fakeSleep records the waits instead of sleeping.
func fakeSleep(t *testing.T) *[]time.Duration {
	var waits []time.Duration
	original := deploySleep
	deploySleep = func(d time.Duration) { waits = append(waits, d) }
	t.Cleanup(func() { deploySleep = original })
	return &waits
}

func sameWaits(got, want []time.Duration) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestPollDeployStatus(t *testing.T) {
	boom := errors.New("boom")
	const (
		one  = 1 * time.Second
		two  = 2 * time.Second
		four = 4 * time.Second
		five = 5 * time.Second
	)
	cases := []struct {
		name      string
		errs      []error
		retries   int
		wantErr   error
		wantWaits []time.Duration
	}{
		{
			name:      "backs off until done",
			errs:      []error{nil, nil, nil, nil},
			wantWaits: []time.Duration{one, two, four},
		},
		{
			name:    "stops on the first error without retries",
			errs:    []error{boom},
			wantErr: boom,
		},
		{
			name:      "waits the maximum interval before retrying the first check",
			errs:      []error{boom, nil},
			retries:   1,
			wantWaits: []time.Duration{five},
		},
		{
			name:      "continues backing off after a successful retry",
			errs:      []error{nil, boom, nil, nil},
			retries:   1,
			wantWaits: []time.Duration{one, five, four},
		},
		{
			name:      "returns the error once retries are used up",
			errs:      []error{boom, boom},
			retries:   1,
			wantErr:   boom,
			wantWaits: []time.Duration{five},
		},
		{
			name:      "counts only consecutive errors against retries",
			errs:      []error{boom, nil, boom, nil},
			retries:   1,
			wantWaits: []time.Duration{five, two, five},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			waits := fakeSleep(t)
			calls := 0
			result, err := PollDeployStatus(func() (ForceCheckDeploymentStatusResult, error) {
				e := c.errs[calls]
				calls++
				return ForceCheckDeploymentStatusResult{Done: e == nil && calls == len(c.errs)}, e
			}, c.retries)
			if err != c.wantErr {
				t.Fatalf("err = %v, want %v", err, c.wantErr)
			}
			if result.Done != (c.wantErr == nil) {
				t.Errorf("result.Done = %v with err = %v", result.Done, err)
			}
			if calls != len(c.errs) {
				t.Errorf("calls = %d, want %d", calls, len(c.errs))
			}
			if !sameWaits(*waits, c.wantWaits) {
				t.Errorf("waits = %v, want %v", *waits, c.wantWaits)
			}
		})
	}
}

func TestWaitForStatusBacksOffAndReportsErrorState(t *testing.T) {
	waits := fakeSleep(t)
	calls := 0
	err := waitForStatus(func() (MetadataDeployStatus, error) {
		calls++
		if calls < 3 {
			return MetadataDeployStatus{State: "InProgress"}, nil
		}
		return MetadataDeployStatus{Done: true, State: "Error", Message: "bad metadata"}, nil
	})
	if err == nil || err.Error() != "bad metadata" {
		t.Fatalf("waitForStatus() error = %v, want bad metadata", err)
	}
	want := []time.Duration{time.Second, 2 * time.Second}
	if !sameWaits(*waits, want) {
		t.Errorf("waits = %v, want %v", *waits, want)
	}
}
