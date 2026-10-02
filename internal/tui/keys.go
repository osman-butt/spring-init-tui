package tui

import "charm.land/bubbles/v2/key"

// keyMap holds every key binding. The help footer is rendered from it, and
// setScreen enables only the bindings that apply to the current screen.
type keyMap struct {
	Up, Down, Next, Back, Retry, Cancel, Quit key.Binding
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
	}
}

// ShortHelp and FullHelp implement help.KeyMap.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Next, k.Back, k.Retry, k.Cancel, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}
