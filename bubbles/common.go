package bubbles

import "github.com/charmbracelet/lipgloss"

var (
	headerStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFDD57"))
	subHeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4E4E4E"))
	infoStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#4E4E4E"))
	testResultStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#89D5C9"))
	detailStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#89D5C9"))
	failureStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#E88388"))
)

const (
	padding  = 2
	maxWidth = 80
)
