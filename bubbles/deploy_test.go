package bubbles

import (
	"strings"
	"testing"

	force "github.com/ForceCLI/force/lib"
	"github.com/charmbracelet/bubbles/progress"
)

func TestDeployViewReportsPassedAndFailedTestsSeparately(t *testing.T) {
	m := DeployModel{
		ForceCheckDeploymentStatusResult: force.ForceCheckDeploymentStatusResult{
			NumberTestsTotal:     120,
			NumberTestsCompleted: 0,
			NumberTestErrors:     36,
		},
		progress:     progress.New(),
		testProgress: progress.New(),
	}
	view := m.View()
	for _, want := range []string{"Total Tests: 120", "Tests Passed: 0", "Tests Failed: 36"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Tests Passed: -") {
		t.Errorf("view reports negative tests passed:\n%s", view)
	}
}

func TestDeployUpdateCountsFailedTestsTowardProgress(t *testing.T) {
	m := DeployModel{
		progress:     progress.New(),
		testProgress: progress.New(),
	}
	updated, _ := m.Update(NewStatusMsg{
		ForceCheckDeploymentStatusResult: force.ForceCheckDeploymentStatusResult{
			NumberTestsTotal:     100,
			NumberTestsCompleted: 10,
			NumberTestErrors:     40,
		},
	})
	m = updated.(DeployModel)
	// SetPercent sets the target; the animation starts from the current percent.
	if got := m.testProgress.Percent(); got != 0.5 {
		t.Errorf("test progress target = %v, want 0.5", got)
	}
}
