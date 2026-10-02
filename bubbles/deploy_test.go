package bubbles

import (
	"fmt"
	"strings"
	"testing"

	force "github.com/ForceCLI/force/lib"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

func manyFailures(n int) []force.TestFailure {
	failures := make([]force.TestFailure, n)
	for i := range failures {
		failures[i] = force.TestFailure{
			Name:       "ReadRequirementProbe8",
			MethodName: fmt.Sprintf("chunk%d", i),
			Message:    "System.AssertException: Assertion Failed: Expected: DOC, Actual: DOC|" + strings.Repeat("0009ee05f04e001fb8020000", 8),
		}
	}
	return failures
}

func sizedModel(width, height int, failures []force.TestFailure) DeployModel {
	m := NewDeployModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	m = updated.(DeployModel)
	result := force.ForceCheckDeploymentStatusResult{
		NumberTestsTotal: 120,
		NumberTestErrors: len(failures),
	}
	result.Details.RunTestResult.TestFailures = failures
	updated, _ = m.Update(NewStatusMsg{ForceCheckDeploymentStatusResult: result})
	return updated.(DeployModel)
}

func TestDeployViewFitsTerminalHeightWithManyFailures(t *testing.T) {
	const height = 40
	m := sizedModel(200, height, manyFailures(500))
	view := m.View()
	if got := lipgloss.Height(view); got > height {
		t.Errorf("view is %d lines, want at most %d", got, height)
	}
	for _, want := range []string{"Deployment Status", "Tests Failed: 500", "Test Failures (500):", "::chunk0 |", "scroll", "q: quit"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}

func TestDeployViewWrapsLongFailureLinesToTerminalWidth(t *testing.T) {
	const width = 100
	m := sizedModel(width, 40, manyFailures(3))
	for _, line := range strings.Split(m.View(), "\n") {
		if w := lipgloss.Width(line); w > width {
			t.Errorf("line is %d columns wide, want at most %d: %q", w, width, line)
		}
	}
}

func TestDeployViewScrollsFailuresWithArrowKeys(t *testing.T) {
	m := sizedModel(200, 30, manyFailures(500))
	before := m.failures.YOffset
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(DeployModel)
	if m.failures.YOffset != before+1 {
		t.Errorf("YOffset after down = %d, want %d", m.failures.YOffset, before+1)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m = updated.(DeployModel)
	if m.failures.YOffset <= before+1 {
		t.Errorf("YOffset after page down = %d, want more than %d", m.failures.YOffset, before+1)
	}
}

func TestDeployViewOmitsViewportWhenNothingFailed(t *testing.T) {
	m := sizedModel(200, 30, nil)
	view := m.View()
	if strings.Contains(view, "Test Failures") || strings.Contains(view, "scroll") {
		t.Errorf("view should not show a failures section:\n%s", view)
	}
	if !strings.Contains(view, "q: quit") {
		t.Errorf("view missing footer:\n%s", view)
	}
}
