package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the global key bindings.
type KeyMap struct {
	Quit       key.Binding
	Tab        key.Binding
	ShiftTab   key.Binding
	CmdPalette key.Binding
	Search     key.Binding
	NewDM      key.Binding
	NewChannel key.Binding
	TagPicker  key.Binding
	Copy       key.Binding
	Edit       key.Binding
	Delete     key.Binding
	Pin        key.Binding
	Help       key.Binding
	Escape     key.Binding
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
		CmdPalette: key.NewBinding(
			key.WithKeys("ctrl+k"),
			key.WithHelp("ctrl+k", "palette"),
		),
		Search: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "search"),
		),
		NewDM: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("ctrl+d", "people"),
		),
		NewChannel: key.NewBinding(
			key.WithKeys("ctrl+n"),
			key.WithHelp("ctrl+n", "new channel"),
		),
		TagPicker: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "tag post"),
		),
		Copy: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "copy selection"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit your post"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete your post"),
		),
		Pin: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "pin or unpin a post"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "close overlay"),
		),
	}
}
