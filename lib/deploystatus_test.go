package lib

import "testing"

func TestNumberTestsRunIncludesFailures(t *testing.T) {
	r := ForceCheckDeploymentStatusResult{NumberTestsCompleted: 10, NumberTestErrors: 36}
	if got := r.NumberTestsRun(); got != 46 {
		t.Errorf("NumberTestsRun() = %d, want 46", got)
	}
}

func TestStatusStringShowsTestsRunOverTotal(t *testing.T) {
	r := ForceCheckDeploymentStatusResult{
		Status:               "InProgress",
		StateDetail:          "Running Test: Foo.bar",
		NumberTestsTotal:     120,
		NumberTestsCompleted: 0,
		NumberTestErrors:     36,
	}
	want := "Status: InProgress (36/120) Running Test: Foo.bar"
	if got := r.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
