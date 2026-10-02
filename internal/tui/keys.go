package tui

import "charm.land/bubbles/v2/key"

// keyMap holds every key binding. The help footer is rendered from it, and
// setScreen enables only the bindings that apply to the current screen.
type keyMap struct {
	Retry, Quit key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Retry: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "retry")),
		Quit:  key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	}
}

// ShortHelp and FullHelp implement help.KeyMap.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Retry, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}
