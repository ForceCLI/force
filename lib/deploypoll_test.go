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
		if got := DeployPollInterval(waits); got != w {
			t.Errorf("DeployPollInterval(%d) = %s, want %s", waits, got, w)
		}
	}
	if got := DeployPollInterval(1000); got != 5*time.Second {
		t.Errorf("DeployPollInterval(1000) = %s, want 5s", got)
	}
}

func TestDeployPollIntervalNegativeWaitsUsesInitial(t *testing.T) {
	if got := DeployPollInterval(-1); got != time.Second {
		t.Errorf("DeployPollInterval(-1) = %s, want 1s", got)
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

func TestWaitForDeployBacksOffUntilDone(t *testing.T) {
	waits := fakeSleep(t)
	calls := 0
	result, err := waitForDeploy(func() (ForceCheckDeploymentStatusResult, error) {
		calls++
		return ForceCheckDeploymentStatusResult{Done: calls == 4}, nil
	})
	if err != nil || !result.Done {
		t.Fatalf("waitForDeploy() = %+v, %v; want done, nil", result, err)
	}
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}
	if calls != 4 || !sameWaits(*waits, want) {
		t.Errorf("calls = %d, waits = %v; want 4 calls, waits %v", calls, *waits, want)
	}
}

func TestWaitForDeployStopsOnError(t *testing.T) {
	waits := fakeSleep(t)
	boom := errors.New("boom")
	_, err := waitForDeploy(func() (ForceCheckDeploymentStatusResult, error) {
		return ForceCheckDeploymentStatusResult{}, boom
	})
	if err != boom || len(*waits) != 0 {
		t.Errorf("err = %v, waits = %v; want boom and no waits", err, *waits)
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
