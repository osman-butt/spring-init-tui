package tui

import "charm.land/lipgloss/v2"

// styles is rebuilt when the terminal reports its background colour, so the
// UI stays readable on light and dark themes.
type styles struct {
	header, selected, subtle, err, spinner lipgloss.Style
	button, activeButton, result, success  lipgloss.Style
}

func newStyles(isDark bool) styles {
	ld := lipgloss.LightDark(isDark)
	green := ld(lipgloss.Color("#4C8B2B"), lipgloss.Color("#6DB33F"))
	muted := ld(lipgloss.Color("#8A8A8A"), lipgloss.Color("#626262"))

	return styles{
		header:   lipgloss.NewStyle().Bold(true),
		selected: lipgloss.NewStyle().Foreground(green),
		subtle:   lipgloss.NewStyle().Foreground(muted),
		err:      lipgloss.NewStyle().Bold(true).Foreground(ld(lipgloss.Color("#D7263D"), lipgloss.Color("#FF5F87"))),
		spinner:  lipgloss.NewStyle().Foreground(green),

		button:       lipgloss.NewStyle().Padding(0, 2).Foreground(muted),
		activeButton: lipgloss.NewStyle().Padding(0, 2).Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#4C8B2B")),
		result:       lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(1, 2),
		success:      lipgloss.NewStyle().Bold(true).Foreground(green),
	}
}
