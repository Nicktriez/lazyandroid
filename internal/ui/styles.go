package ui

import "github.com/charmbracelet/lipgloss"

var (
	cAccent = lipgloss.Color("39")
	cDim    = lipgloss.Color("241")
	cGood   = lipgloss.Color("42")
	cWarn   = lipgloss.Color("214")
	cBad    = lipgloss.Color("203")
	cSelBg  = lipgloss.Color("237")

	styleTitle = lipgloss.NewStyle().Bold(true).Foreground(cAccent)
	styleDim   = lipgloss.NewStyle().Foreground(cDim)
	styleSel   = lipgloss.NewStyle().Background(cSelBg).Bold(true)
	styleGood  = lipgloss.NewStyle().Foreground(cGood)
	styleWarn  = lipgloss.NewStyle().Foreground(cWarn)
	styleBad   = lipgloss.NewStyle().Foreground(cBad)

	stylePane = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cDim).
			Padding(0, 1)
)

// stateStyle colours the state a row reports: green when the device can be
// targeted, yellow while it is seen but is not usable yet, red when the host
// has to fix something — accept the USB debugging prompt, install udev rules.
// An unrecognised state is taken to be broken rather than usable.
func stateStyle(state string) lipgloss.Style {
	switch state {
	case "ready", "running":
		return styleGood
	case "offline", "booting":
		return styleWarn
	default:
		return styleBad
	}
}
