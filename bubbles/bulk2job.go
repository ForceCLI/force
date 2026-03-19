package bubbles

import (
	"fmt"

	force "github.com/ForceCLI/force/lib"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	bulk2InfoStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#4E4E4E")).TabWidth(lipgloss.NoTabConversion)
	bulk2StatusStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#89D5C9")).TabWidth(lipgloss.NoTabConversion)
)

type Bulk2JobModel struct {
	force.Bulk2IngestJobInfo
	progress progress.Model
}

type NewBulk2JobStatusMsg struct {
	force.Bulk2IngestJobInfo
}

func NewBulk2JobModel() Bulk2JobModel {
	return Bulk2JobModel{
		progress: progress.New(progress.WithDefaultGradient()),
	}
}

func (m Bulk2JobModel) Init() tea.Cmd {
	return nil
}

func (m Bulk2JobModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.progress.Width = min(msg.Width-padding*2-4, maxWidth)
		return m, nil

	case NewBulk2JobStatusMsg:
		m.Bulk2IngestJobInfo = msg.Bulk2IngestJobInfo
		var cmd tea.Cmd
		if m.IsTerminal() {
			cmd = m.progress.SetPercent(1.0)
		} else if m.State == force.Bulk2JobStateInProgress {
			cmd = m.progress.SetPercent(0.5)
		} else {
			cmd = m.progress.SetPercent(0.1)
		}
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

func (m Bulk2JobModel) View() string {
	header := headerStyle.Render("Bulk API 2.0 Job Status")

	var infoMsg = `
Id				%s
State 				%s
Operation			%s
Object 				%s
Api Version 			%.1f

Created By Id 			%s
Created Date 			%s
Content Type 			%s
`

	var statusMsg = `
Number Records Processed 	%d
Number Records Failed 		%d
Retries 			%d

Total Processing Time 		%d ms
Api Active Processing Time 	%d ms
Apex Processing Time 		%d ms
`

	components := []string{
		lipgloss.JoinVertical(lipgloss.Top, header,
			bulk2InfoStyle.Render(fmt.Sprintf(infoMsg,
				m.Id, m.State, m.Operation, m.Object, m.ApiVersion,
				m.CreatedById, m.CreatedDate, m.ContentType)),
			m.progress.View(),
			bulk2StatusStyle.Render(fmt.Sprintf(statusMsg,
				m.NumberRecordsProcessed, m.NumberRecordsFailed, m.Retries,
				m.TotalProcessingTime, m.ApiActiveProcessingTime, m.ApexProcessingTime))),
	}

	if m.ErrorMessage != "" {
		errorMsg := failureStyle.Render(fmt.Sprintf("Error: %s", m.ErrorMessage))
		components = append(components, errorMsg)
	}

	return lipgloss.JoinVertical(lipgloss.Top, components...)
}
