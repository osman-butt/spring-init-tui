package tui

import "charm.land/bubbles/v2/key"

// keyMap holds every key binding. The help footer is rendered from it, and
// syncKeys enables only the bindings that apply at the moment.
type keyMap struct {
	Up, Down, Toggle, Next, Back, Retry, Cancel, Quit key.Binding

	// The confirm prompt.
	Switch, Confirm, Yes, No key.Binding

	// Move stands in for Up and Down in the footer, to keep it short.
	Move key.Binding

	// The dependency list handles these itself. They are listed here so the
	// footer can show them.
	Filter, ApplyFilter, CancelFilter, ClearFilter key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Toggle: key.NewBinding(key.WithKeys("space", "tab"), key.WithHelp("space", "select")),
		Next:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "continue")),
		Back:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Retry:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "retry")),
		// Cancel replaces Quit where "q" has to be typeable.
		Cancel: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "quit")),
		Quit:   key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),

		Switch:  key.NewBinding(key.WithKeys("left", "right", "h", "l", "tab"), key.WithHelp("←/→", "switch")),
		Confirm: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
		Yes:     key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yes")),
		No:      key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "no")),

		Move: key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "move")),

		Filter:       key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		ApplyFilter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "apply filter")),
		CancelFilter: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		ClearFilter:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear filter")),
	}
}

// ShortHelp and FullHelp implement help.KeyMap.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Move, k.Toggle, k.Filter, k.ApplyFilter, k.CancelFilter,
		k.Switch, k.Next, k.Confirm, k.Yes, k.No,
		k.ClearFilter, k.Retry, k.Back, k.Cancel, k.Quit,
	}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}
