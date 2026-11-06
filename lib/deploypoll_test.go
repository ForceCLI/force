package lib

import (
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
