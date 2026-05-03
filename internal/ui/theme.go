package ui

import "github.com/charmbracelet/lipgloss"

var (
	colorCyan    = lipgloss.Color("#00FFFF")
	colorGreen   = lipgloss.Color("#00FF88")
	colorYellow  = lipgloss.Color("#FFD700")
	colorRed     = lipgloss.Color("#FF4444")
	colorDim     = lipgloss.Color("#444466")
	colorWhite   = lipgloss.Color("#E0E0FF")
	colorMagenta = lipgloss.Color("#CC88FF")

	StyleInfo    = lipgloss.NewStyle().Foreground(colorCyan)
	StyleSuccess = lipgloss.NewStyle().Foreground(colorGreen)
	StyleWarning = lipgloss.NewStyle().Foreground(colorYellow)
	StyleDanger  = lipgloss.NewStyle().Foreground(colorRed)
	StyleDim     = lipgloss.NewStyle().Foreground(colorDim)
	StyleWhite   = lipgloss.NewStyle().Foreground(colorWhite)
	StyleRegion  = lipgloss.NewStyle().Foreground(colorMagenta)
	StyleID      = lipgloss.NewStyle().Foreground(colorCyan)
	StyleBold    = lipgloss.NewStyle().Bold(true)

	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorCyan).
			Padding(0, 1)

	PanelStyle = BorderStyle.Copy()
)
