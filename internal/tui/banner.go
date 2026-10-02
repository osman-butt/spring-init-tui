package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// bannerArt spells SPRING in the "ANSI Shadow" figlet font. Every line has
// the same width, so a column has the same colour on every line.
var bannerArt = [...]string{
	"███████╗██████╗ ██████╗ ██╗███╗   ██╗ ██████╗ ",
	"██╔════╝██╔══██╗██╔══██╗██║████╗  ██║██╔════╝ ",
	"███████╗██████╔╝██████╔╝██║██╔██╗ ██║██║  ███╗",
	"╚════██║██╔═══╝ ██╔══██╗██║██║╚██╗██║██║   ██║",
	"███████║██║     ██║  ██║██║██║ ╚████║╚██████╔╝",
	"╚══════╝╚═╝     ╚═╝  ╚═╝╚═╝╚═╝  ╚═══╝ ╚═════╝ ",
}

const (
	bannerWordmark = "I N I T I A L I Z R"
	bannerCompact  = "SPRING INITIALIZR"

	// bannerMinHeight is the terminal height below which the big banner
	// would leave the dependency list fewer than four rows.
	bannerMinHeight = 20
)

// bannerColors returns a left-to-right gradient from Spring's green to teal,
// in darker shades on a light background.
func bannerColors(isDark bool, steps int) []color.Color {
	ld := lipgloss.LightDark(isDark)
	return lipgloss.Blend1D(steps,
		ld(lipgloss.Color("#4C8B2B"), lipgloss.Color("#6DB33F")),
		ld(lipgloss.Color("#0F8B8D"), lipgloss.Color("#2EC4B6")),
	)
}

// gradient colours each character of line by the column it is in.
func gradient(line string, colors []color.Color, base lipgloss.Style) string {
	var b strings.Builder
	column := 0
	for _, r := range strings.TrimRight(line, " ") {
		if r == ' ' {
			b.WriteRune(r)
		} else {
			b.WriteString(base.Foreground(colors[min(column, len(colors)-1)]).Render(string(r)))
		}
		column++
	}
	return b.String()
}

// renderBanner returns the big banner: the SPRING art with INITIALIZR
// centred below it.
func renderBanner(isDark bool) string {
	width := lipgloss.Width(bannerArt[0])
	colors := bannerColors(isDark, width)
	lines := make([]string, 0, len(bannerArt)+1)
	for _, line := range bannerArt {
		lines = append(lines, gradient(line, colors, lipgloss.NewStyle()))
	}
	// Indent before colouring, so each letter gets the colour of the art
	// column above it.
	indent := strings.Repeat(" ", (width-lipgloss.Width(bannerWordmark))/2)
	lines = append(lines, gradient(indent+bannerWordmark, colors, lipgloss.NewStyle().Bold(true)))
	return strings.Join(lines, "\n")
}

// renderCompactBanner returns the one-line banner for small terminals.
func renderCompactBanner(isDark bool) string {
	colors := bannerColors(isDark, lipgloss.Width(bannerCompact))
	return gradient(bannerCompact, colors, lipgloss.NewStyle().Bold(true))
}

// bannerView picks the banner that fits the terminal. Both are rendered
// once, in setStyles, rather than on every frame.
func (m Model) bannerView() string {
	tooNarrow := m.width > 0 && m.width < lipgloss.Width(m.banner)
	tooShort := m.height > 0 && m.height < bannerMinHeight
	if tooNarrow || tooShort {
		return m.compactBanner
	}
	return m.banner
}
