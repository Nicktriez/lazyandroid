package ui

import "github.com/charmbracelet/lipgloss"

var (
	cAccent = lipgloss.Color("39")
	cDim    = lipgloss.Color("241")
	cGood   = lipgloss.Color("42")
	cBad    = lipgloss.Color("203")
	cSelBg  = lipgloss.Color("237")

	styleTitle = lipgloss.NewStyle().Bold(true).Foreground(cAccent)
	styleDim   = lipgloss.NewStyle().Foreground(cDim)
	styleSel   = lipgloss.NewStyle().Background(cSelBg).Bold(true)
	styleGood  = lipgloss.NewStyle().Foreground(cGood)
	styleBad   = lipgloss.NewStyle().Foreground(cBad)

	stylePane = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cDim).
			Padding(0, 1)
)
