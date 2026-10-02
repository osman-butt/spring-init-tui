package tui

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// foregrounds returns the distinct true-colour foregrounds used in s.
func foregrounds(s string) map[string]bool {
	colors := map[string]bool{}
	for _, m := range regexp.MustCompile(`38;2;\d+;\d+;\d+`).FindAllString(s, -1) {
		colors[m] = true
	}
	return colors
}

func TestBannerArtLinesHaveTheSameWidth(t *testing.T) {
	want := lipgloss.Width(bannerArt[0])
	for i, line := range bannerArt {
		if got := lipgloss.Width(line); got != want {
			t.Errorf("art line %d is %d wide, want %d", i, got, want)
		}
	}
}

func TestBigBanner(t *testing.T) {
	m := New(&fakeBackend{})
	m, _ = step(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})

	banner := m.bannerView()
	if w, h := lipgloss.Size(banner); w != 46 || h != 7 {
		t.Errorf("banner is %dx%d, want 46x7", w, h)
	}

	lines := strings.Split(ansiSequence.ReplaceAllString(banner, ""), "\n")
	for i, art := range bannerArt {
		if lines[i] != strings.TrimRight(art, " ") {
			t.Errorf("banner line %d = %q, want the art line %q", i, lines[i], art)
		}
	}
	// 46 columns of art, 19 of wordmark: 13 to the left, 14 to the right.
	if want := strings.Repeat(" ", 13) + "I N I T I A L I Z R"; lines[6] != want {
		t.Errorf("last banner line = %q, want %q", lines[6], want)
	}
	if strings.Contains(banner, "Build") {
		t.Error("the banner should not carry a tagline")
	}
}

func TestBannerUsesAGradient(t *testing.T) {
	dark := renderBanner(true)
	light := renderBanner(false)

	// One colour per column of the art.
	if got, want := len(foregrounds(dark)), lipgloss.Width(bannerArt[0]); got != want {
		t.Errorf("dark banner uses %d colours, want %d", got, want)
	}
	for c := range foregrounds(light) {
		if foregrounds(dark)[c] {
			t.Errorf("colour %s is used on both light and dark backgrounds", c)
		}
	}

	// A column keeps its colour from line to line: every art line starts
	// in column 0.
	first := regexp.MustCompile(`(?m)^\x1b\[[0-9;]*38;2;(\d+;\d+;\d+)`).FindAllStringSubmatch(dark, -1)
	if len(first) != len(bannerArt) {
		t.Fatalf("found a leading colour on %d lines, want %d", len(first), len(bannerArt))
	}
	for i, m := range first {
		if m[1] != first[0][1] {
			t.Errorf("line %d starts with colour %s, line 0 with %s", i, m[1], first[0][1])
		}
	}

	// The centred wordmark starts in column 13 and takes that column's
	// colour, not the first one.
	lines := strings.Split(dark, "\n")
	wordmark := regexp.MustCompile(`38;2;(\d+;\d+;\d+)`).FindStringSubmatch(lines[len(lines)-1])
	if wordmark == nil || wordmark[1] == first[0][1] {
		t.Errorf("wordmark starts with colour %v, want the colour of its own column", wordmark)
	}
}

func TestCompactBannerOnSmallTerminals(t *testing.T) {
	tests := []struct {
		name          string
		width, height int
		compact       bool
	}{
		{name: "roomy", width: 120, height: 40},
		{name: "exactly enough", width: 46, height: 20},
		{name: "one column short", width: 45, height: 40, compact: true},
		{name: "one row short", width: 120, height: 19, compact: true},
		{name: "narrow", width: 40, height: 40, compact: true},
		{name: "short", width: 80, height: 16, compact: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(&fakeBackend{})
			m, _ = step(t, m, tea.WindowSizeMsg{Width: tt.width, Height: tt.height})

			banner := m.bannerView()
			text := ansiSequence.ReplaceAllString(banner, "")
			if tt.compact {
				if text != "SPRING INITIALIZR" {
					t.Errorf("banner = %q, want the one-line fallback", text)
				}
				if len(foregrounds(banner)) < 2 {
					t.Error("the fallback should keep the gradient")
				}
				return
			}
			if lipgloss.Height(banner) != 7 {
				t.Errorf("banner is %d lines, want the 7-line wordmark", lipgloss.Height(banner))
			}
		})
	}
}

func TestBannerFollowsTheBackgroundColour(t *testing.T) {
	m := New(&fakeBackend{})
	dark := m.bannerView()
	m.setStyles(false)
	if m.bannerView() == dark {
		t.Error("the banner should be re-rendered for a light background")
	}
}
