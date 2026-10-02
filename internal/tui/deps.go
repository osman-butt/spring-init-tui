package tui

import (
	"fmt"
	"io"
	"maps"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/osman-butt/spring-init-tui/internal/initializr"
)

const (
	// depsMaxRows caps the dependency list so the inline UI stays compact
	// on tall terminals.
	depsMaxRows = 12

	// depsChromeHeight is the number of lines depsView draws around the
	// rows: header, search input and match count above, page dots and the
	// selection below.
	depsChromeHeight = 5
)

type depItem struct{ initializr.Dependency }

// FilterValue is what the search matches against.
func (d depItem) FilterValue() string { return d.Name + " " + d.ID }

// newDepsList returns a list that only draws its rows. The search input is
// always active on this screen, so the list's own filter mode ("/" to start,
// no cursor while typing) is not used: updateDeps feeds it the search text,
// and depsView draws everything around the rows.
func newDepsList() list.Model {
	l := list.New(nil, depDelegate{}, 80, depsMaxRows)
	l.SetStatusBarItemName("match", "matches") // for the "No matches." text
	l.SetShowTitle(false)
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()

	// Letters belong to the search, so only keys that are not text may move
	// the cursor. Bindings without keys never match.
	l.KeyMap.CursorUp = key.NewBinding(key.WithKeys("up"))
	l.KeyMap.CursorDown = key.NewBinding(key.WithKeys("down"))
	l.KeyMap.PrevPage = key.NewBinding(key.WithKeys("pgup"))
	l.KeyMap.NextPage = key.NewBinding(key.WithKeys("pgdown"))
	l.KeyMap.GoToStart = key.NewBinding()
	l.KeyMap.GoToEnd = key.NewBinding()
	l.KeyMap.Filter = key.NewBinding()
	l.KeyMap.ClearFilter = key.NewBinding()
	l.KeyMap.ShowFullHelp = key.NewBinding()
	l.KeyMap.CloseFullHelp = key.NewBinding()
	return l
}

func newSearchInput() textinput.Model {
	input := textinput.New()
	input.Placeholder = "Type to search"
	input.CharLimit = 64
	return input
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

	cursor, nameStyle := "  ", lipgloss.NewStyle()
	if index == m.Index() {
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

// availableDeps returns the dependencies that work with the chosen Spring
// Boot version.
func (m Model) availableDeps() []initializr.Dependency {
	boot := m.boot().ID
	var deps []initializr.Dependency
	for _, d := range m.metadata.Dependencies {
		if d.Supports(boot) {
			deps = append(deps, d)
		}
	}
	return deps
}

// showAvailableDeps lists the dependencies for the chosen Spring Boot
// version, with the search still applied.
func (m *Model) showAvailableDeps() tea.Cmd {
	cmd := m.deps.SetItems(depItems(m.availableDeps()))
	m.deps.ResetSelected()
	m.applySearch()
	return cmd
}

// selectedDeps returns the chosen dependency IDs in metadata order. A
// dependency picked under another Spring Boot version is left out while the
// current one cannot use it.
func (m Model) selectedDeps() []string {
	var ids []string
	for _, d := range m.availableDeps() {
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

// applySearch narrows the list to the dependencies matching the search text
// and puts the cursor on the best match.
func (m *Model) applySearch() {
	if query := m.search.Value(); query != "" {
		m.deps.SetFilterText(query)
	} else {
		m.deps.ResetFilter()
	}
}

func (m Model) updateDeps(msg tea.Msg) (Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(k, m.keys.Toggle):
			m.toggleDep()
			return m, nil
		case key.Matches(k, m.keys.Next):
			m.search.Blur()
			m.setScreen(screenConfirm)
			return m, nil
		case key.Matches(k, m.keys.ClearSearch):
			m.search.SetValue("")
			m.applySearch()
			return m, nil
		case key.Matches(k, m.keys.Back):
			m.search.Blur()
			m.setScreen(screenJava)
			return m, nil
		case key.Matches(k, m.keys.ListNav):
			var cmd tea.Cmd
			m.deps, cmd = m.deps.Update(msg)
			return m, cmd
		}
	}

	// Everything else is search text.
	before := m.search.Value()
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	if m.search.Value() != before {
		m.applySearch()
	}
	return m, cmd
}

func (m Model) depsView() string {
	total := len(m.availableDeps())
	count := fmt.Sprintf("%d dependencies", total)
	if m.search.Value() != "" {
		count = fmt.Sprintf("%d of %d", len(m.deps.VisibleItems()), total)
	}
	if hidden := len(m.metadata.Dependencies) - total; hidden > 0 {
		count += fmt.Sprintf(" (%d not available for Spring Boot %s)", hidden, bootLabel(m.boot()))
	}

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

	return strings.Join([]string{
		m.styles.header.Render("Dependencies"),
		m.search.View(),
		m.styles.subtle.Render(count),
		m.deps.View(),
		pages,
		chosen,
	}, "\n")
}
