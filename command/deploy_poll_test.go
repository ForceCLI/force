package command

import (
	"errors"
	"testing"
	"time"

	. "github.com/ForceCLI/force/lib"
)

func TestMonitorDeployStatusBacksOffAndRetriesOnce(t *testing.T) {
	var waits []time.Duration
	original := deploySleep
	deploySleep = func(d time.Duration) { waits = append(waits, d) }
	defer func() { deploySleep = original }()

	responses := []error{nil, errors.New("timeout"), nil, nil}
	calls := 0
	result, err := monitorDeployStatus(func() (ForceCheckDeploymentStatusResult, error) {
		e := responses[calls]
		calls++
		return ForceCheckDeploymentStatusResult{Done: calls == len(responses)}, e
	})
	if err != nil || !result.Done {
		t.Fatalf("monitorDeployStatus() = %+v, %v; want done, nil", result, err)
	}
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}
	if len(waits) != len(want) {
		t.Fatalf("waits = %v, want %v", waits, want)
	}
	for i := range want {
		if waits[i] != want[i] {
			t.Fatalf("waits = %v, want %v", waits, want)
		}
	}
}

func TestMonitorDeployStatusGivesUpAfterTwoErrors(t *testing.T) {
	original := deploySleep
	deploySleep = func(time.Duration) {}
	defer func() { deploySleep = original }()

	calls := 0
	_, err := monitorDeployStatus(func() (ForceCheckDeploymentStatusResult, error) {
		calls++
		return ForceCheckDeploymentStatusResult{}, errors.New("timeout")
	})
	if err == nil || calls != 2 {
		t.Errorf("err = %v after %d calls; want an error after 2 calls", err, calls)
	}
}
