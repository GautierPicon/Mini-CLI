package tui

import "github.com/charmbracelet/lipgloss"

var (
	TitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	UserStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	AIStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("118"))
	ErrorStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	HintStyle   = lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("245"))
	BorderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(0, 1)
)
