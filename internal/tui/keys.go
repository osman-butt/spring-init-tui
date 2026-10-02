package tui

import "charm.land/bubbles/v2/key"

// keyMap holds every key binding. The help footer is rendered from it, and
// syncKeys enables only the bindings that apply at the moment.
type keyMap struct {
	Up, Down, Next, Back, Retry, Cancel, Quit key.Binding

	// Interrupt is always handled. It is listed in the footer only where
	// no other quit key is available.
	Interrupt key.Binding

	// The confirm prompt.
	Switch, Confirm, Yes, No key.Binding

	// Move stands in for Up and Down in the footer, to keep it short.
	Move key.Binding

	// The dependency screen. Letters and space are search text there, so
	// its bindings avoid them.
	Toggle, ListNav, ClearSearch key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up:    key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:  key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Next:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "continue")),
		Back:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Retry: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "retry")),
		// Cancel replaces Quit where "q" has to be typeable.
		Cancel: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "quit")),
		Quit:   key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),

		Interrupt: key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),

		Switch:  key.NewBinding(key.WithKeys("left", "right", "h", "l", "tab"), key.WithHelp("←/→", "switch")),
		Confirm: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
		Yes:     key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yes")),
		No:      key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "no")),

		Move: key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "move")),

		Toggle:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "select")),
		ListNav:     key.NewBinding(key.WithKeys("up", "down", "pgup", "pgdown")),
		ClearSearch: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear search")),
	}
}

// ShortHelp and FullHelp implement help.KeyMap.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Move, k.Toggle, k.Switch, k.Next, k.Confirm, k.Yes, k.No,
		k.ClearSearch, k.Retry, k.Back, k.Cancel, k.Quit, k.Interrupt,
	}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}
