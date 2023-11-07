package bubbles

import (
	"bytes"
	"fmt"
	"math"
	"strconv"

	force "github.com/ForceCLI/force/lib"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/olekukonko/tablewriter"
)

type DeployModel struct {
	force.ForceCheckDeploymentStatusResult
	progress     progress.Model
	testProgress progress.Model
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
		m.progress.Width = msg.Width - padding*2 - 4
		if m.progress.Width > maxWidth {
			m.progress.Width = maxWidth
		}
		m.testProgress.Width = m.progress.Width
		return m, nil

	case NewStatusMsg:
		m.ForceCheckDeploymentStatusResult = msg.ForceCheckDeploymentStatusResult
		completion := float64(m.NumberComponentsDeployed) / float64(m.NumberComponentsTotal)
		if math.IsNaN(completion) || completion < 0.01 {
			completion = 0
		}
		cmds = append(cmds, m.progress.SetPercent(completion))

		testCompletion := float64(m.NumberTestsCompleted) / float64(m.NumberTestsTotal)
		if math.IsNaN(testCompletion) || testCompletion < 0.01 {
			testCompletion = 0
		}
		cmds = append(cmds, m.testProgress.SetPercent(testCompletion))

		return m, tea.Batch(cmds...)
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
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

func (m DeployModel) View() string {
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
	createdBy := infoStyle.Render(fmt.Sprintf("Created Date: %s", m.CreatedByName))
	lastModifiedDate := infoStyle.Render(fmt.Sprintf("Last Modified Date: %s", m.LastModifiedDate))
	totalComponents := infoStyle.Render(fmt.Sprintf("Components Deployed: %d", m.NumberComponentsDeployed))
	deployedComponents := infoStyle.Render(fmt.Sprintf("Total Components: %d", m.NumberComponentsTotal))
	totalTests := infoStyle.Render(fmt.Sprintf("Total Tests: %d", m.NumberTestsTotal))
	passedTests := infoStyle.Render(fmt.Sprintf("Tests Passed: %d", m.NumberTestsCompleted-m.NumberTestErrors))

	testResultsHeader := subHeaderStyle.Render("Test Results:")
	numTestsRun := testResultStyle.Render(fmt.Sprintf("Number of Tests Run: %d", m.Details.RunTestResult.NumberOfTestsRun))
	numTestsPassed := testResultStyle.Render(fmt.Sprintf("Number of Tests Passed: %d", m.Details.RunTestResult.NumberOfTestsRun-m.Details.RunTestResult.NumberOfFailures))
	totalTime := testResultStyle.Render(fmt.Sprintf("Total Time: %f", m.Details.RunTestResult.TotalTime))

	failuresHeader := subHeaderStyle.Render("Test Failures:")
	failures := ""
	for _, failure := range m.Details.RunTestResult.TestFailures {
		failures += failureStyle.Render(fmt.Sprintf("Method: %s::%s | Message: %s", failure.Name, failure.MethodName, failure.Message)) + "\n"
	}

	components := []string{
		header, "", id, status, stateDetail, success, createdDate, createdBy,
		lastModifiedDate, totalComponents, deployedComponents, m.progress.View(), totalTests,
		passedTests,
	}
	if m.NumberTestsCompleted > 0 {
		components = append(components, m.testProgress.View())
	} else {
		components = append(components, m.testProgress.ViewAs(0))
	}
	if m.Details.RunTestResult.NumberOfTestsRun > 0 {
		components = append(components, "", testResultsHeader, numTestsRun, numTestsPassed, totalTime)
	}
	if len(m.Details.RunTestResult.TestFailures) > 0 {
		components = append(components, "", failuresHeader, failures)
	}

	deploymentFailures := new(bytes.Buffer)
	table := tablewriter.NewWriter(deploymentFailures)
	table.SetRowLine(true)
	table.SetHeader([]string{
		"Component Type",
		"Name",
		// "File Name",
		"Line Number",
		"Problem Type",
		"Problem",
	})
	for _, f := range m.Details.ComponentFailures {
		table.Append([]string{
			f.ComponentType,
			f.FullName,
			// f.FileName,
			strconv.Itoa(f.LineNumber),
			f.ProblemType,
			f.Problem,
		})
	}
	if table.NumLines() > 0 {
		table.Render()
		components = append(components, "", deploymentFailures.String())
	}
	return lipgloss.JoinVertical(lipgloss.Top, components...)
}

func NewDeployModel() DeployModel {
	return DeployModel{
		progress:     progress.New(progress.WithDefaultGradient()),
		testProgress: progress.New(progress.WithDefaultGradient()),
	}
}
