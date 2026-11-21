package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
)

// KeyMap defines the global key bindings.
type KeyMap struct {
	Quit       key.Binding
	Tab        key.Binding
	ShiftTab   key.Binding
	CmdPalette key.Binding
	Search     key.Binding
	NewDM      key.Binding
	NewChannel key.Binding
	Theme      key.Binding
	Info       key.Binding
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
		Quit:       bind("ctrl+c", "quit"),
		Tab:        bind("tab", "next pane"),
		ShiftTab:   bind("shift+tab", "prev pane"),
		CmdPalette: bind("ctrl+k", "palette"),
		Search:     bind("ctrl+s", "search"),
		NewDM:      bind("ctrl+d", "people"),
		NewChannel: bind("ctrl+n", "new channel"),
		Theme:      bind("ctrl+t", "themes"),
		TagPicker:  bind("t", "tag post"),
		Info:       bind("i", "post details"),
		Copy:       bind("y", "copy selection"),
		Edit:       bind("e", "edit your post"),
		Delete:     bindHelp("d", "d d", "delete your post"),
		Pin:        bind("p", "pin or unpin a post"),
		Help:       bind("?", "help"),
		Escape:     bind("esc", "close overlay"),
	}
}

// bind makes a binding of one key, with its help text.
func bind(keys, desc string) key.Binding {
	return bindHelp(keys, keys, desc)
}

// bindHelp makes a binding of one key whose help names other keys, as
// "d d" for the delete that asks first.
func bindHelp(keys, helpKeys, desc string) key.Binding {
	b := key.NewBinding(key.WithKeys(keys), key.WithHelp(helpKeys, desc))
	assert.That(len(b.Keys()) == 1 && b.Help().Desc == desc, "bindHelp: binding not built")
	return b
}
