package bubbles

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"

	force "github.com/ForceCLI/force/lib"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/olekukonko/tablewriter"
)

// DeployModel renders deployment status as a fixed header of status lines and
// progress bars, followed by a scrollable viewport holding the test and
// component failures. The viewport is sized to the terminal height that
// remains below the header so that a long failure list never pushes the
// status lines off the screen.
type DeployModel struct {
	force.ForceCheckDeploymentStatusResult
	progress     progress.Model
	testProgress progress.Model
	failures     viewport.Model
	width        int
	height       int
}

type NewStatusMsg struct {
	force.ForceCheckDeploymentStatusResult
}

func (m DeployModel) Init() tea.Cmd {
	return nil
}

func (m DeployModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progress.Width = msg.Width - padding*2 - 4
		if m.progress.Width > maxWidth {
			m.progress.Width = maxWidth
		}
		m.testProgress.Width = m.progress.Width
		m.layout()
		return m, nil

	case NewStatusMsg:
		m.ForceCheckDeploymentStatusResult = msg.ForceCheckDeploymentStatusResult
		completion := float64(m.NumberComponentsDeployed) / float64(m.NumberComponentsTotal)
		if math.IsNaN(completion) || completion < 0.01 {
			completion = 0
		}
		cmds = append(cmds, m.progress.SetPercent(completion))

		testCompletion := float64(m.NumberTestsRun()) / float64(m.NumberTestsTotal)
		if math.IsNaN(testCompletion) || testCompletion < 0.01 {
			testCompletion = 0
		}
		cmds = append(cmds, m.testProgress.SetPercent(testCompletion))

		m.layout()
		return m, tea.Batch(cmds...)
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
		var cmd tea.Cmd
		m.failures, cmd = m.failures.Update(msg)
		return m, cmd
	case progress.FrameMsg:
		progressModel, progressCmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)

		testProgressModel, testProgressCmd := m.testProgress.Update(msg)
		m.testProgress = testProgressModel.(progress.Model)

		return m, tea.Batch(progressCmd, testProgressCmd)
	case QuitMsg:
		return m, tea.Quit
	}
	return m, nil
}

// layout refreshes the viewport contents and sizes it to fill the terminal
// height left below the header and above the footer.
func (m *DeployModel) layout() {
	content := m.failureContent()
	m.failures.Width = m.width
	if m.width == 0 || m.height == 0 || content == "" {
		m.failures.Height = 0
		m.failures.SetContent("")
		return
	}
	// View separates the header, viewport and footer with one blank line each.
	const separators = 2
	available := m.height - lipgloss.Height(m.headerView()) - lipgloss.Height(m.footerView()) - separators
	contentHeight := lipgloss.Height(content)
	if available > contentHeight {
		available = contentHeight
	}
	if available < 1 {
		available = 1
	}
	m.failures.Height = available
	m.failures.SetContent(content)
}

func (m DeployModel) headerView() string {
	headerLabel := "Deployment Status"
	if m.CheckOnly {
		headerLabel = "Validation Deployment Status"
	}
	header := headerStyle.Render(headerLabel)
	id := infoStyle.Render(fmt.Sprintf("ID: %s", m.Id))
	var status string
	switch m.Status {
	case "Canceled", "Failed":
		status = failureStyle.Render(fmt.Sprintf("Status: %s", m.Status))
	default:
		status = infoStyle.Render(fmt.Sprintf("Status: %s", m.Status))
	}
	stateDetail := detailStyle.Render(fmt.Sprintf("State Detail: %s", m.StateDetail))
	success := infoStyle.Render(fmt.Sprintf("Success: %v", m.Success))
	createdDate := infoStyle.Render(fmt.Sprintf("Created Date: %s", m.CreatedDate))
	createdBy := infoStyle.Render(fmt.Sprintf("Created By: %s", m.CreatedByName))
	lastModifiedDate := infoStyle.Render(fmt.Sprintf("Last Modified Date: %s", m.LastModifiedDate))
	deployedComponents := infoStyle.Render(fmt.Sprintf("Components Deployed: %d", m.NumberComponentsDeployed))
	totalComponents := infoStyle.Render(fmt.Sprintf("Total Components: %d", m.NumberComponentsTotal))
	totalTests := infoStyle.Render(fmt.Sprintf("Total Tests: %d", m.NumberTestsTotal))
	passedTests := infoStyle.Render(fmt.Sprintf("Tests Passed: %d", m.NumberTestsCompleted))
	failedTests := infoStyle.Render(fmt.Sprintf("Tests Failed: %d", m.NumberTestErrors))

	components := []string{
		header, "", id, status, stateDetail, success, createdDate, createdBy,
		lastModifiedDate, deployedComponents, totalComponents, m.progress.View(), totalTests,
		passedTests, failedTests,
	}
	if m.NumberTestsRun() > 0 {
		components = append(components, m.testProgress.View())
	} else {
		components = append(components, m.testProgress.ViewAs(0))
	}
	if m.Details.RunTestResult.NumberOfTestsRun > 0 {
		testResultsHeader := subHeaderStyle.Render("Test Results:")
		numTestsRun := testResultStyle.Render(fmt.Sprintf("Number of Tests Run: %d", m.Details.RunTestResult.NumberOfTestsRun))
		numTestsPassed := testResultStyle.Render(fmt.Sprintf("Number of Tests Passed: %d", m.Details.RunTestResult.NumberOfTestsRun-m.Details.RunTestResult.NumberOfFailures))
		totalTime := testResultStyle.Render(fmt.Sprintf("Total Time: %f", m.Details.RunTestResult.TotalTime))
		components = append(components, "", testResultsHeader, numTestsRun, numTestsPassed, totalTime)
	}
	return lipgloss.JoinVertical(lipgloss.Top, components...)
}

// failureContent renders the test failures and the component failure table
// that go into the scrollable viewport. Lines are wrapped to the terminal
// width so that the viewport, not the terminal, controls what is shown.
func (m DeployModel) failureContent() string {
	var sections []string

	if n := len(m.Details.RunTestResult.TestFailures); n > 0 {
		lines := []string{subHeaderStyle.Render(fmt.Sprintf("Test Failures (%d):", n))}
		lineStyle := failureStyle
		if m.width > 0 {
			lineStyle = lineStyle.Copy().Width(m.width)
		}
		for _, failure := range m.Details.RunTestResult.TestFailures {
			lines = append(lines, lineStyle.Render(fmt.Sprintf("Method: %s::%s | Message: %s", failure.Name, failure.MethodName, failure.Message)))
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	if n := len(m.Details.ComponentFailures); n > 0 {
		deploymentFailures := new(bytes.Buffer)
		table := tablewriter.NewWriter(deploymentFailures)
		table.SetRowLine(true)
		table.SetHeader([]string{
			"Component Type",
			"Name",
			"Line Number",
			"Problem Type",
			"Problem",
		})
		for _, f := range m.Details.ComponentFailures {
			table.Append([]string{
				f.ComponentType,
				f.FullName,
				strconv.Itoa(f.LineNumber),
				f.ProblemType,
				f.Problem,
			})
		}
		table.Render()
		sections = append(sections, subHeaderStyle.Render(fmt.Sprintf("Component Failures (%d):", n))+"\n"+strings.TrimRight(deploymentFailures.String(), "\n"))
	}

	return strings.Join(sections, "\n\n")
}

func (m DeployModel) footerView() string {
	hints := []string{"q: quit"}
	if m.failures.Height > 0 && m.failures.TotalLineCount() > m.failures.Height {
		hints = append([]string{fmt.Sprintf("%3.0f%%", m.failures.ScrollPercent()*100), "↑/↓ PgUp/PgDn: scroll"}, hints...)
	}
	return infoStyle.Render(strings.Join(hints, " · "))
}

func (m DeployModel) View() string {
	components := []string{m.headerView()}
	if m.failures.Height > 0 {
		components = append(components, "", m.failures.View())
	}
	components = append(components, "", m.footerView())
	return lipgloss.JoinVertical(lipgloss.Top, components...)
}

func NewDeployModel() DeployModel {
	return DeployModel{
		progress:     progress.New(progress.WithDefaultGradient()),
		testProgress: progress.New(progress.WithDefaultGradient()),
		failures:     viewport.New(0, 0),
	}
}
