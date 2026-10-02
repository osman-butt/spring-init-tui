package tui

import (
	"fmt"
	"io"
	"maps"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/osman-butt/spring-init-tui/internal/initializr"
)

// depsMaxHeight caps the dependency list so the inline UI stays compact on
// tall terminals.
const depsMaxHeight = 14

type depItem struct{ initializr.Dependency }

// FilterValue is what the list's "/" filter matches against.
func (d depItem) FilterValue() string { return d.Name + " " + d.ID }

func newDepsList() list.Model {
	l := list.New(nil, depDelegate{}, 80, depsMaxHeight)
	l.Title = "Dependencies"
	l.SetStatusBarItemName("dependency", "dependencies")
	l.SetShowHelp(false) // the root model renders one footer for all screens
	// The list's own pagination is one line taller with several pages than
	// with one, which would make the frame jump. depsView draws it instead.
	l.SetShowPagination(false)
	l.DisableQuitKeybindings()
	l.KeyMap.ShowFullHelp.SetEnabled(false)
	l.KeyMap.CloseFullHelp.SetEnabled(false)
	return l
}

func depItems(deps []initializr.Dependency) []list.Item {
	items := make([]list.Item, len(deps))
	for i, d := range deps {
		items[i] = depItem{d}
	}
	return items
}

// depDelegate draws one dependency per line with a checkbox.
type depDelegate struct {
	selected map[string]bool
	styles   styles
}

func (d depDelegate) Height() int                         { return 1 }
func (d depDelegate) Spacing() int                        { return 0 }
func (d depDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d depDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	dep, ok := item.(depItem)
	if !ok {
		return
	}

	// Matches are rune positions in FilterValue: the name, a space, the ID.
	var nameMatches, idMatches []int
	if m.FilterState() != list.Unfiltered {
		nameLen := utf8.RuneCountInString(dep.Name)
		for _, i := range m.MatchesForItem(index) {
			if i < nameLen {
				nameMatches = append(nameMatches, i)
			} else if i > nameLen {
				idMatches = append(idMatches, i-nameLen-1)
			}
		}
	}

	// The list has no cursor while the user is typing a filter.
	cursor, nameStyle := "  ", lipgloss.NewStyle()
	if index == m.Index() && m.FilterState() != list.Filtering {
		cursor, nameStyle = d.styles.selected.Render("> "), d.styles.selected
	}
	box := "[ ] "
	if d.selected[dep.ID] {
		box = d.styles.selected.Render("[x] ")
	}

	line := cursor + box +
		lipgloss.StyleRunes(dep.Name, nameMatches, nameStyle.Underline(true), nameStyle) + "  " +
		lipgloss.StyleRunes(dep.ID, idMatches, d.styles.subtle.Underline(true), d.styles.subtle)
	fmt.Fprint(w, lipgloss.NewStyle().MaxWidth(m.Width()).Render(line))
}

// syncDepsDelegate hands the list a delegate with the current selection and
// styles. The delegate is a value inside the list, so it has to be replaced
// whenever either of them changes.
func (m *Model) syncDepsDelegate() {
	m.deps.SetDelegate(depDelegate{selected: m.selected, styles: m.styles})
}

// selectedDeps returns the chosen dependency IDs in metadata order.
func (m Model) selectedDeps() []string {
	var ids []string
	for _, d := range m.metadata.Dependencies {
		if m.selected[d.ID] {
			ids = append(ids, d.ID)
		}
	}
	return ids
}

func (m *Model) toggleDep() {
	dep, ok := m.deps.SelectedItem().(depItem)
	if !ok {
		return
	}
	// Copy before changing, so earlier copies of the model keep their own
	// selection.
	selected := maps.Clone(m.selected)
	if selected == nil {
		selected = map[string]bool{}
	}
	if selected[dep.ID] {
		delete(selected, dep.ID)
	} else {
		selected[dep.ID] = true
	}
	m.selected = selected
	m.syncDepsDelegate()
}

func (m Model) updateDeps(msg tea.Msg) (Model, tea.Cmd) {
	// While the user types a filter every key belongs to the list.
	if k, ok := msg.(tea.KeyPressMsg); ok && m.deps.FilterState() != list.Filtering {
		switch {
		case key.Matches(k, m.keys.Toggle):
			m.toggleDep()
			return m, nil
		case key.Matches(k, m.keys.Next):
			m.setScreen(screenSummary)
			return m, nil
		case key.Matches(k, m.keys.Back):
			// With a filter applied, esc clears it (handled by the list).
			if m.deps.FilterState() == list.Unfiltered {
				m.setScreen(screenJava)
				return m, nil
			}
		case key.Matches(k, m.keys.Quit):
			return m.quit()
		}
	}

	var cmd tea.Cmd
	m.deps, cmd = m.deps.Update(msg)
	if _, ok := msg.(list.FilterMatchesMsg); ok {
		// The list recounts its pages on key presses only, so the page
		// dots would lag one keystroke behind the matches. Setting the
		// size makes it recount.
		m.deps.SetSize(m.deps.Width(), m.deps.Height())
	}
	return m, cmd
}

// depsFooterHeight is the number of lines depsView adds below the list.
const depsFooterHeight = 2

func (m Model) depsView() string {
	// Page dots, or "3/15" when the dots do not fit.
	var pages string
	if p := m.deps.Paginator; p.TotalPages > 1 {
		pages = p.View()
		if lipgloss.Width(pages) > m.width {
			pages = m.styles.subtle.Render(fmt.Sprintf("%d/%d", p.Page+1, p.TotalPages))
		}
	}

	chosen := m.styles.subtle.Render("Selected: ") + "none"
	if ids := m.selectedDeps(); len(ids) > 0 {
		chosen = m.styles.subtle.Render(fmt.Sprintf("Selected (%d): ", len(ids))) + strings.Join(ids, ", ")
	}
	return strings.Join([]string{m.deps.View(), pages, chosen}, "\n")
}
