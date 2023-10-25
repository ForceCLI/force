package bubbles

import (
	"fmt"

	force "github.com/ForceCLI/force/lib"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	jobInfoStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#4E4E4E")).TabWidth(lipgloss.NoTabConversion)
	jobStatusStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#89D5C9")).TabWidth(lipgloss.NoTabConversion)
)

type JobModel struct {
	force.JobInfo
	progress progress.Model
}

type NewJobStatusMsg struct {
	force.JobInfo
}

type QuitMsg struct{}

func NewJobModel() JobModel {
	return JobModel{
		progress: progress.New(progress.WithDefaultGradient()),
	}
}

func (m JobModel) Init() tea.Cmd {
	return nil
}

func (m JobModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.progress.Width = msg.Width - padding*2 - 4
		if m.progress.Width > maxWidth {
			m.progress.Width = maxWidth
		}
		return m, nil

	case NewJobStatusMsg:
		m.JobInfo = msg.JobInfo
		// cmd := m.progress.SetPercent(float64(m.NumberComponentsDeployed) / float64(m.NumberComponentsTotal))
		cmd := m.progress.SetPercent(0.5)
		return m, cmd
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd
	case QuitMsg:
		return m, tea.Quit
	}
	return m, nil
}

func (m JobModel) View() string {
	header := headerStyle.Render("Bulk Job Status")
	var infoMsg = `
Id				%s
State 				%s
Operation			%s
Object 				%s
Api Version 			%s

Created By Id 			%s
Created Date 			%s
System Mod Stamp		%s
Content Type 			%s
Concurrency Mode 		%s
`

	var statusMsg = `
Number Batches Queued 		%d
Number Batches In Progress	%d
Number Batches Completed 	%d
Number Batches Failed 		%d
Number Batches Total 		%d
Number Records Processed 	%d
Number Retries 			%d

Number Records Failed 		%d
Total Processing Time 		%d
Api Active Processing Time 	%d
Apex Processing Time 		%d
`
	return lipgloss.JoinVertical(lipgloss.Top, header,
		jobInfoStyle.Render(fmt.Sprintf(infoMsg, m.Id, m.State, m.Operation, m.Object, m.ApiVersion,
			m.CreatedById, m.CreatedDate, m.SystemModStamp,
			m.ContentType, m.ConcurrencyMode)),
		jobStatusStyle.Render(fmt.Sprintf(statusMsg,
			m.NumberBatchesQueued, m.NumberBatchesInProgress,
			m.NumberBatchesCompleted, m.NumberBatchesFailed,
			m.NumberBatchesTotal, m.NumberRecordsProcessed,
			m.NumberRetries,
			m.NumberRecordsFailed, m.TotalProcessingTime,
			m.ApiActiveProcessingTime, m.ApexProcessingTime)))
}
