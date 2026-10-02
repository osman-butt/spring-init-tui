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
	screenPackage
	screenBoot
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
	name        string
	nameErr     error
	groupID     string
	groupErr    error
	packageName string
	packageErr  error
	bootCursor  int
	javaCursor  int
	selected    map[string]bool // dependency IDs
	confirmYes  bool

	// packageDefault is the package last derived from the group and the
	// name. While the input still holds it, the user has not edited the
	// package, so it keeps following the two.
	packageDefault string

	// exists reports whether a path is already taken. Tests replace it.
	exists func(path string) bool

	keys    keyMap
	help    help.Model
	spinner spinner.Model
	input   textinput.Model // artifact name
	group   textinput.Model
	pkg     textinput.Model
	search  textinput.Model // dependency search
	deps    list.Model
	styles  styles

	// Rendered in setStyles; bannerView picks the one that fits.
	banner, compactBanner string
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
		pkg:     newPackageInput(),
		search:  newSearchInput(),
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
	// Typing in the dependency search changes which keys apply, so refresh
	// the bindings after every message.
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
		m.bootCursor = m.defaultBootIndex()
		m.javaCursor = m.defaultJavaIndex()
		m.setScreen(screenName)
		return m, tea.Batch(m.input.Focus(), m.showAvailableDeps())

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
	case screenPackage:
		return m.updatePackage(msg)
	case screenBoot:
		return m.updateBoot(msg)
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
	onDeps := s == screenDeps
	searching := onDeps && m.search.Value() != ""
	typing := s == screenGroup || s == screenPackage // esc goes back, so only ctrl+c quits
	choosing := s == screenBoot || s == screenJava

	k := &m.keys
	k.Up.SetEnabled(choosing)
	k.Down.SetEnabled(choosing)
	k.Move.SetEnabled(choosing || onDeps)
	k.Toggle.SetEnabled(onDeps)
	k.ListNav.SetEnabled(onDeps)
	// esc empties the search first and goes back only when it is empty.
	k.ClearSearch.SetEnabled(searching)
	k.Next.SetEnabled(s == screenName || typing || choosing || onDeps)
	k.Switch.SetEnabled(s == screenConfirm)
	k.Confirm.SetEnabled(s == screenConfirm)
	k.Yes.SetEnabled(s == screenConfirm)
	k.No.SetEnabled(s == screenConfirm)
	// From the error screen there is only a way back when the answers
	// exist, i.e. when generating failed rather than loading.
	generateFailed := s == screenError && m.failed == screenGenerating
	k.Back.SetEnabled(typing || choosing || s == screenConfirm || (onDeps && !searching) || generateFailed)
	k.Retry.SetEnabled(s == screenError)
	// "q" is text on the screens with an input. On the name screen esc
	// quits; on the others esc is taken, which leaves ctrl+c as the only
	// way out there.
	k.Cancel.SetEnabled(s == screenName)
	k.Quit.SetEnabled(s != screenName && !typing && !onDeps)
	k.Interrupt.SetEnabled(typing || onDeps)
}

func (m *Model) setStyles(isDark bool) {
	m.styles = newStyles(isDark)
	m.banner = renderBanner(isDark)
	m.compactBanner = renderCompactBanner(isDark)
	m.help.Styles = help.DefaultStyles(isDark)
	m.spinner.Style = m.styles.spinner

	inputStyles := textinput.DefaultStyles(isDark)
	inputStyles.Focused.Prompt = m.styles.selected
	m.input.SetStyles(inputStyles)
	m.group.SetStyles(inputStyles)
	m.pkg.SetStyles(inputStyles)
	m.search.SetStyles(inputStyles)

	listStyles := list.DefaultStyles(isDark)
	listStyles.NoItems = m.styles.subtle
	m.deps.Styles = listStyles
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
	m.pkg.SetWidth(inputWidth)
	m.search.SetWidth(inputWidth)

	const blankLines = 3 // below the banner, above the footer, end of frame
	chrome := lipgloss.Height(m.bannerView()) + depsChromeHeight + lipgloss.Height(m.footerView()) + blankLines
	m.deps.SetSize(m.width, min(max(m.height-chrome, 1), depsMaxRows))
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
	case screenPackage:
		body = m.packageView()
	case screenBoot:
		body = m.bootView()
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

func (m Model) footerView() string {
	return m.help.View(m.keys)
}
