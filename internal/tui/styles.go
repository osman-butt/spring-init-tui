package tui

import "charm.land/lipgloss/v2"

// styles is rebuilt when the terminal reports its background colour, so the
// UI stays readable on light and dark themes.
type styles struct {
	banner, title, tagline, header, selected, subtle, err, spinner lipgloss.Style
}

func newStyles(isDark bool) styles {
	ld := lipgloss.LightDark(isDark)
	green := ld(lipgloss.Color("#4C8B2B"), lipgloss.Color("#6DB33F"))
	muted := ld(lipgloss.Color("#8A8A8A"), lipgloss.Color("#626262"))

	return styles{
		banner:   lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(green).Padding(1, 3),
		title:    lipgloss.NewStyle().Bold(true).Foreground(green),
		tagline:  lipgloss.NewStyle().Italic(true),
		header:   lipgloss.NewStyle().Bold(true),
		selected: lipgloss.NewStyle().Foreground(green),
		subtle:   lipgloss.NewStyle().Foreground(muted),
		err:      lipgloss.NewStyle().Bold(true).Foreground(ld(lipgloss.Color("#D7263D"), lipgloss.Color("#FF5F87"))),
		spinner:  lipgloss.NewStyle().Foreground(green),
	}
}
