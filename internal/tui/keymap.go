package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the global key bindings.
type KeyMap struct {
	Quit         key.Binding
	Tab          key.Binding
	ShiftTab     key.Binding
	ToggleThread key.Binding
	CmdPalette   key.Binding
	Search       key.Binding
	NewDM        key.Binding
	NewChannel   key.Binding
	TagPicker    key.Binding
	Escape       key.Binding
	Enter        key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next pane"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev pane"),
		),
		ToggleThread: key.NewBinding(
			key.WithKeys("ctrl+t"),
			key.WithHelp("ctrl+t", "toggle thread"),
		),
		CmdPalette: key.NewBinding(
			key.WithKeys("ctrl+k"),
			key.WithHelp("ctrl+k", "command palette"),
		),
		Search: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "search"),
		),
		NewDM: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("ctrl+d", "new DM"),
		),
		NewChannel: key.NewBinding(
			key.WithKeys("ctrl+n"),
			key.WithHelp("ctrl+n", "new channel"),
		),
		TagPicker: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "tag post"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "close overlay"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "send/select"),
		),
	}
}
