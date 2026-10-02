// Package tui is the Bubble Tea user interface.
package tui

import (
	"cmp"
	"context"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/osman-butt/spring-init-tui/internal/initializr"
)

// Backend is the part of the Spring Initializr client the UI needs.
type Backend interface {
	Metadata(ctx context.Context) (initializr.Metadata, error)
	Generate(ctx context.Context, req initializr.Request, dest string) error
}

type screen int

const (
	screenLoading screen = iota
	screenName
	screenGroup
	screenJava
	screenDeps
	screenConfirm
	screenGenerating
	screenDone
	screenError
)

// Results of background work arrive as messages.
type (
	metadataLoadedMsg struct{ metadata initializr.Metadata }
	loadFailedMsg     struct{ err error }
	generatedMsg      struct{}
	generateFailedMsg struct{ err error }
)

// Model is the root Bubble Tea model.
type Model struct {
	screen        screen
	backend       Backend
	ctx           context.Context
	cancel        context.CancelFunc
	metadata      initializr.Metadata
	err           error
	failed        screen // the step that produced err: loading or generating
	quitting      bool
	interrupted   bool
	width, height int

	// Answers collected so far.
	name       string
	nameErr    error
	groupID    string
	groupErr   error
	javaCursor int
	selected   map[string]bool // dependency IDs
	confirmYes bool

	// exists reports whether a path is already taken. Tests replace it.
	exists func(path string) bool

	keys    keyMap
	help    help.Model
	spinner spinner.Model
	input   textinput.Model // project name
	group   textinput.Model
	deps    list.Model
	styles  styles
}

// New returns a model that loads its options from backend.
func New(backend Backend) Model {
	ctx, cancel := context.WithCancel(context.Background())
	m := Model{
		backend: backend,
		ctx:     ctx,
		cancel:  cancel,
		exists:  pathExists,

		confirmYes: true,

		keys:    defaultKeyMap(),
		help:    help.New(),
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
		input:   newNameInput(),
		group:   newGroupInput(),
		deps:    newDepsList(),
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
	next, cmd := m.update(msg)
	// The dependency list changes which keys apply as its filter opens and
	// closes, so refresh the bindings after every message.
	next.syncKeys()
	return next, cmd
}

func (m Model) update(msg tea.Msg) (Model, tea.Cmd) {
	if m.quitting {
		// Anything that arrives after the user quit, such as the cancelled
		// request failing, must not change the last frame or the exit code.
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
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
		m.group.SetValue(cmp.Or(msg.metadata.DefaultGroupID, fallbackGroup))
		m.javaCursor = m.defaultJavaIndex()
		m.setScreen(screenName)
		return m, tea.Batch(m.input.Focus(), m.deps.SetItems(depItems(msg.metadata.Dependencies)))

	case loadFailedMsg:
		return m.fail(screenLoading, msg.err), nil

	case generatedMsg:
		m.setScreen(screenDone)
		return m.quit()

	case generateFailedMsg:
		return m.fail(screenGenerating, msg.err), nil
	}

	switch m.screen {
	case screenLoading:
		return m.updateLoading(msg)
	case screenName:
		return m.updateName(msg)
	case screenGroup:
		return m.updateGroup(msg)
	case screenJava:
		return m.updateJava(msg)
	case screenDeps:
		return m.updateDeps(msg)
	case screenConfirm:
		return m.updateConfirm(msg)
	case screenGenerating:
		return m.updateGenerating(msg)
	case screenError:
		return m.updateError(msg)
	}
	return m, nil
}

func (m Model) updateLoading(msg tea.Msg) (Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && key.Matches(k, m.keys.Quit) {
		return m.quit()
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// fail shows err on the error screen and remembers which step to retry.
func (m Model) fail(step screen, err error) Model {
	m.err, m.failed = err, step
	m.setScreen(screenError)
	return m
}

func (m Model) updateError(msg tea.Msg) (Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(k, m.keys.Retry):
			m.err = nil
			if m.failed == screenGenerating {
				return m.startGenerating()
			}
			m.setScreen(screenLoading)
			return m, tea.Batch(m.spinner.Tick, m.loadCmd())
		case key.Matches(k, m.keys.Back):
			m.err = nil
			m.setScreen(screenConfirm)
		case key.Matches(k, m.keys.Quit):
			return m.quit()
		}
	}
	return m, nil
}

func (m *Model) setScreen(s screen) {
	m.screen = s
	m.syncKeys()
}

// syncKeys enables only the bindings that apply right now, which keeps the
// help footer accurate and stops stray keys from triggering hidden actions.
func (m *Model) syncKeys() {
	s := m.screen
	filter := m.deps.FilterState()
	typingFilter := s == screenDeps && filter == list.Filtering
	browsingDeps := s == screenDeps && !typingFilter
	filterApplied := browsingDeps && filter == list.FilterApplied

	k := &m.keys
	k.Up.SetEnabled(s == screenJava || browsingDeps)
	k.Down.SetEnabled(s == screenJava || browsingDeps)
	k.Move.SetEnabled(s == screenJava || browsingDeps)
	k.Toggle.SetEnabled(browsingDeps)
	k.Filter.SetEnabled(browsingDeps)
	k.ApplyFilter.SetEnabled(typingFilter)
	k.CancelFilter.SetEnabled(typingFilter)
	k.ClearFilter.SetEnabled(filterApplied)
	k.Next.SetEnabled(s == screenName || s == screenGroup || s == screenJava || browsingDeps)
	k.Switch.SetEnabled(s == screenConfirm)
	k.Confirm.SetEnabled(s == screenConfirm)
	k.Yes.SetEnabled(s == screenConfirm)
	k.No.SetEnabled(s == screenConfirm)
	// From the error screen there is only a way back when the answers
	// exist, i.e. when generating failed rather than loading.
	generateFailed := s == screenError && m.failed == screenGenerating
	k.Back.SetEnabled(s == screenGroup || s == screenJava || s == screenConfirm || (browsingDeps && !filterApplied) || generateFailed)
	k.Retry.SetEnabled(s == screenError)
	// "q" is text on the screens with an input, and esc is taken by Back on
	// the group screen, which leaves ctrl+c as the only way out there.
	k.Cancel.SetEnabled(s == screenName)
	k.Quit.SetEnabled(s != screenName && s != screenGroup && !typingFilter)
	k.Interrupt.SetEnabled(s == screenGroup)
}

func (m *Model) setStyles(isDark bool) {
	m.styles = newStyles(isDark)
	m.help.Styles = help.DefaultStyles(isDark)
	m.spinner.Style = m.styles.spinner

	inputStyles := textinput.DefaultStyles(isDark)
	inputStyles.Focused.Prompt = m.styles.selected
	m.input.SetStyles(inputStyles)
	m.group.SetStyles(inputStyles)

	// Drop the list's default indentation so it lines up with the other
	// screens.
	listStyles := list.DefaultStyles(isDark)
	listStyles.TitleBar = lipgloss.NewStyle()
	listStyles.Title = m.styles.header
	listStyles.StatusBar = m.styles.subtle
	listStyles.Filter.Focused.Prompt = m.styles.selected
	listStyles.Filter.Blurred.Prompt = m.styles.selected
	m.deps.Styles = listStyles
	m.deps.FilterInput.SetStyles(listStyles.Filter)
	m.syncDepsDelegate()
}

// resize fits the components to the terminal. The dependency list gets the
// rows left over after everything else on its screen.
func (m *Model) resize() {
	m.help.SetWidth(m.width)
	// Leave room for the prompt and the cursor.
	inputWidth := max(m.width-len(m.input.Prompt)-1, 1)
	m.input.SetWidth(inputWidth)
	m.group.SetWidth(inputWidth)

	const blankLines = 3 // below the banner, above the footer, end of frame
	chrome := lipgloss.Height(m.bannerView()) + depsFooterHeight + lipgloss.Height(m.footerView()) + blankLines
	m.deps.SetSize(m.width, min(max(m.height-chrome, 4), depsMaxHeight))
}

func (m Model) quit() (Model, tea.Cmd) {
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
	// the banner, unless there is a result or an error worth leaving on
	// screen.
	if m.quitting && m.err == nil && m.screen != screenDone {
		return m.bannerView()
	}

	var body string
	switch m.screen {
	case screenLoading:
		body = m.spinner.View() + "Fetching metadata…" // Dot frames end with a space
	case screenName:
		body = m.nameView()
	case screenGroup:
		body = m.groupView()
	case screenJava:
		body = m.javaView()
	case screenDeps:
		body = m.depsView()
	case screenConfirm:
		body = m.confirmView()
	case screenGenerating:
		body = m.spinner.View() + "Generating " + m.name + "…"
	case screenDone:
		body = m.doneView()
	case screenError:
		body = m.styles.err.Width(m.width).Render("Error: " + m.err.Error())
	}

	parts := []string{m.bannerView(), "", body}
	if !m.quitting {
		parts = append(parts, "", m.footerView())
	}
	// MaxWidth cuts off anything a screen failed to fit, so a narrow
	// terminal never wraps lines behind the renderer's back.
	return lipgloss.NewStyle().MaxWidth(m.width).Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
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

func (m Model) footerView() string {
	return m.help.View(m.keys)
}
