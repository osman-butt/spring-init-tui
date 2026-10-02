// Package tui is the Bubble Tea user interface.
package tui

import (
	"context"
	"fmt"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/osman-butt/spring-init-tui/internal/initializr"
)

// Backend is the part of the Spring Initializr client the UI needs.
type Backend interface {
	Metadata(ctx context.Context) (initializr.Metadata, error)
}

type screen int

const (
	screenLoading screen = iota
	screenReady
	screenError
)

// Results of background work arrive as messages.
type (
	metadataLoadedMsg struct{ metadata initializr.Metadata }
	loadFailedMsg     struct{ err error }
)

// Model is the root Bubble Tea model.
type Model struct {
	screen      screen
	backend     Backend
	ctx         context.Context
	cancel      context.CancelFunc
	metadata    initializr.Metadata
	err         error
	quitting    bool
	interrupted bool
	width       int

	keys    keyMap
	help    help.Model
	spinner spinner.Model
	styles  styles
}

// New returns a model that loads its options from backend.
func New(backend Backend) Model {
	ctx, cancel := context.WithCancel(context.Background())
	m := Model{
		backend: backend,
		ctx:     ctx,
		cancel:  cancel,
		keys:    defaultKeyMap(),
		help:    help.New(),
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
	}
	m.setStyles(true) // replaced once tea.BackgroundColorMsg arrives
	m.setScreen(screenLoading)
	return m
}

// Err returns the error the user quit on, if any.
func (m Model) Err() error {
	return m.err
}

// Interrupted reports whether the user quit with ctrl+c.
func (m Model) Interrupted() bool {
	return m.interrupted
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tea.RequestBackgroundColor, m.spinner.Tick, m.loadCmd())
}

func (m Model) loadCmd() tea.Cmd {
	backend, ctx := m.backend, m.ctx
	return func() tea.Msg {
		metadata, err := backend.Metadata(ctx)
		if err != nil {
			return loadFailedMsg{err}
		}
		return metadataLoadedMsg{metadata}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.help.SetWidth(msg.Width)
		return m, nil

	case tea.BackgroundColorMsg:
		m.setStyles(msg.IsDark())
		return m, nil

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			// tea.Interrupt would skip the final render and leave the
			// current screen behind, so quit normally and report the
			// interrupt through Interrupted.
			m.err = nil
			m.interrupted = true
			return m.quit()
		}

	case metadataLoadedMsg:
		m.metadata = msg.metadata
		m.setScreen(screenReady)
		return m, nil

	case loadFailedMsg:
		m.err = msg.err
		m.setScreen(screenError)
		return m, nil
	}

	switch m.screen {
	case screenLoading:
		return m.updateLoading(msg)
	case screenReady:
		return m.updateReady(msg)
	case screenError:
		return m.updateError(msg)
	}
	return m, nil
}

func (m Model) updateLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && key.Matches(k, m.keys.Quit) {
		return m.quit()
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m Model) updateReady(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && key.Matches(k, m.keys.Quit) {
		return m.quit()
	}
	return m, nil
}

func (m Model) updateError(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(k, m.keys.Retry):
			m.err = nil
			m.setScreen(screenLoading)
			return m, tea.Batch(m.spinner.Tick, m.loadCmd())
		case key.Matches(k, m.keys.Quit):
			return m.quit()
		}
	}
	return m, nil
}

// setScreen switches screens and enables only the bindings that apply there,
// which keeps the help footer accurate.
func (m *Model) setScreen(s screen) {
	m.screen = s
	m.keys.Retry.SetEnabled(s == screenError)
}

func (m *Model) setStyles(isDark bool) {
	m.styles = newStyles(isDark)
	m.help.Styles = help.DefaultStyles(isDark)
	m.spinner.Style = m.styles.spinner
}

func (m Model) quit() (tea.Model, tea.Cmd) {
	m.cancel()
	m.quitting = true
	return m, tea.Quit
}

// View ends every frame with an empty line: on exit Bubble Tea leaves the
// cursor on the last line of the frame and erases it.
func (m Model) View() tea.View {
	return tea.NewView(m.content() + "\n")
}

func (m Model) content() string {
	// The last frame stays in the scrollback. When the user quits, keep only
	// the banner, unless there is an error worth leaving on screen.
	if m.quitting && m.err == nil {
		return m.bannerView()
	}

	var body string
	switch m.screen {
	case screenLoading:
		body = m.spinner.View() + "Fetching metadata…" // Dot frames end with a space
	case screenReady:
		body = fmt.Sprintf("Loaded %d Java versions and %d dependencies.",
			len(m.metadata.JavaVersions), len(m.metadata.Dependencies))
	case screenError:
		body = m.styles.err.Width(m.width).Render("Error: " + m.err.Error())
	}

	parts := []string{m.bannerView(), "", body}
	if !m.quitting {
		parts = append(parts, "", m.help.View(m.keys))
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m Model) bannerView() string {
	title := m.styles.title.Render("SPRING INITIALIZR")
	banner := m.styles.banner.Render(lipgloss.JoinVertical(lipgloss.Left,
		title,
		m.styles.tagline.Render("Build. Configure. Generate."),
	))
	if m.width > 0 && lipgloss.Width(banner) > m.width {
		return title
	}
	return banner
}
